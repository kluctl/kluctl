package webui

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	kluctlv1 "github.com/kluctl/kluctl/v2/api/v1beta1"
	"github.com/kluctl/kluctl/v2/pkg/results"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"io/fs"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"net"
	"net/http"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

const webuiManager = "kluctl-webui"

type ProjectTargetKey struct {
	Project result.ProjectKey `json:"project"`
	Target  result.TargetKey  `json:"target"`
}

type CommandResultsServer struct {
	ctx   context.Context
	store results.ResultStore
	cam   *clusterAccessorManager

	// this is the client for the k8s cluster where the server runs on
	serverClient       client.Client
	serverCoreV1Client *corev1.CoreV1Client

	auth   *authHandler
	events *eventsHandler

	onlyApi bool
}

func NewCommandResultsServer(
	ctx context.Context,
	store *results.ResultsCollector,
	configs []*rest.Config,
	serverConfig *rest.Config,
	serverClient client.Client,
	authConfig AuthConfig,
	onlyApi bool) (*CommandResultsServer, error) {

	ret := &CommandResultsServer{
		ctx:   ctx,
		store: store,
		cam: &clusterAccessorManager{
			ctx: ctx,
		},
		serverClient: serverClient,
		onlyApi:      onlyApi,
	}

	var err error
	if serverConfig != nil {
		ret.serverCoreV1Client, err = corev1.NewForConfig(serverConfig)
		if err != nil {
			return nil, err
		}
	}

	ret.events = newEventsHandler(ret)

	ret.auth, err = newAuthHandler(ctx, serverClient, authConfig)
	if err != nil {
		return nil, err
	}

	for _, config := range configs {
		ret.cam.add(config)
	}

	return ret, nil
}

func (s *CommandResultsServer) Run(host string, port int, openBrowser bool) error {
	err := s.startUpdateLogs()
	if err != nil {
		return err
	}

	s.cam.start()

	if os.Getenv(gin.EnvGinMode) == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.LoggerWithConfig(gin.LoggerConfig{
		SkipPaths: []string{"/healthz", "/readyz"},
	}))
	router.Use(gin.Recovery())

	err = s.auth.setupRoutes(router)
	if err != nil {
		return err
	}

	if !s.onlyApi {
		err = s.setupStaticRoutes(router)
		if err != nil {
			return err
		}
	}

	router.GET("/healthz", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	router.GET("/readyz", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	api := router.Group("/api", s.auth.authHandler)
	api.GET("/getShortNames", s.getShortNames)
	api.GET("/getCommandResult", s.getCommandResult)
	api.GET("/getCommandResultObject", s.getCommandResultObject)
	api.GET("/getValidateResult", s.getValidateResult)
	api.POST("/validateNow", s.validateNow)
	api.POST("/reconcileNow", s.reconcileNow)
	api.POST("/deployNow", s.deployNow)
	api.POST("/setSuspended", s.setSuspended)
	api.POST("/setManualObjectsHash", s.setManualObjectsHash)

	err = s.events.startEventsWatcher()
	if err != nil {
		return err
	}
	api.Any("/events", s.events.handler)

	api.Any("/logs", s.logsHandler)

	address := fmt.Sprintf("%s:%d", host, port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}

	httpServer := http.Server{
		Addr: address,
		BaseContext: func(listener net.Listener) context.Context {
			return s.ctx
		},
		Handler: router.Handler(),
	}

	if host != "" {
		url := fmt.Sprintf("http://%s", address)
		status.Infof(s.ctx, "Webui is available at: %s\n", url)

		if openBrowser {
			status.Infof(s.ctx, "Opening browser")
			_ = utils.OpenBrowser(url)
		}
	}

	return httpServer.Serve(listener)
}

func (s *CommandResultsServer) startUpdateLogs() error {
	ch1, cancel1, err := s.store.WatchCommandResultSummaries(results.ListResultSummariesOptions{})
	if err != nil {
		return err
	}

	ch2, cancel2, err := s.store.WatchValidateResultSummaries(results.ListResultSummariesOptions{})
	if err != nil {
		cancel1()
		return err
	}

	ch3, cancel3, err := s.store.WatchKluctlDeployments()
	if err != nil {
		cancel1()
		cancel2()
		return err
	}

	go func() {
		defer cancel1()
		defer cancel2()
		defer cancel3()

		printEvent := func(id string, type_ string, deleted bool) {
			if deleted {
				status.Infof(s.ctx, "Deleted %s for %s", type_, id)
			} else {
				status.Infof(s.ctx, "Updated %s for %s", type_, id)
			}
		}

		for {
			select {
			case event := <-ch1:
				printEvent(event.Summary.Id, "command result summary", event.Delete)
			case event := <-ch2:
				printEvent(event.Summary.Id, "validate result summary", event.Delete)
			case event := <-ch3:
				printEvent(event.Deployment.Name, "KluctlDeployment", event.Delete)
			case <-s.ctx.Done():
				return
			}
		}
	}()

	return nil
}

func (s *CommandResultsServer) setupStaticRoutes(router gin.IRouter) error {
	dis, err := fs.ReadDir(uiFS, ".")
	if err != nil {
		return err
	}

	for _, di := range dis {
		if di.IsDir() {
			x, err := fs.Sub(uiFS, di.Name())
			if err != nil {
				return err
			}
			router.StaticFS("/"+di.Name(), http.FS(x))
		} else {
			router.StaticFileFS("/"+di.Name(), di.Name(), http.FS(uiFS))
		}
	}
	router.GET("/", func(c *gin.Context) {
		b, err := fs.ReadFile(uiFS, "index.html")
		if err != nil {
			_ = c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", b)
	})
	return nil
}

func (s *CommandResultsServer) getShortNames(c *gin.Context) {
	c.JSON(http.StatusOK, GetShortNames())
}

type resultIdParam struct {
	ResultId string `form:"resultId"`
}
type refParam struct {
	Group     string `form:"group"`
	Version   string `form:"version"`
	Kind      string `form:"kind"`
	Name      string `form:"name"`
	Namespace string `form:"namespace"`
}
type objectTypeParams struct {
	ObjectType string `form:"objectType"`
}

func (ref refParam) toK8sRef() k8s.ObjectRef {
	return k8s.ObjectRef{
		Group:     ref.Group,
		Version:   ref.Version,
		Kind:      ref.Kind,
		Name:      ref.Name,
		Namespace: ref.Namespace,
	}
}

func (s *CommandResultsServer) redactSensitiveVars(user *User, d *types.DeploymentProjectConfig) {
	if d == nil {
		return
	}
	if user.IsAdmin {
		// don't redact vars
		return
	}

	doRedact := func(vars *types.VarsSource) {
		if vars.RenderedSensitive {
			vars.RenderedVars = nil
		}
	}

	for _, v := range d.Vars {
		doRedact(v)
	}

	for _, di := range d.Deployments {
		for _, v := range di.Vars {
			doRedact(v)
		}
		s.redactSensitiveVars(user, di.RenderedInclude)
	}
}

func (s *CommandResultsServer) checkObjectAccess(c *gin.Context, user *User, o *uo.UnstructuredObject, objectType string) (bool, *uo.UnstructuredObject) {
	if user.IsAdmin {
		// everything allowed
		return true, o
	}
	if o == nil {
		return true, o
	}

	// non-admins can only see the rendered version
	if objectType != "rendered" {
		return false, o
	}

	// but no secrets
	if o.GetK8sRef().GroupKind() == (schema.GroupKind{Kind: "Secret"}) {
		return false, nil
	}

	return true, o
}

func (s *CommandResultsServer) getCommandResult(c *gin.Context) {
	var params resultIdParam

	err := c.Bind(&params)
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	sr, err := s.store.GetCommandResult(results.GetCommandResultOptions{
		Id:      params.ResultId,
		Reduced: true,
	})
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}
	if sr == nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	user := s.auth.getUser(c)

	s.redactSensitiveVars(user, sr.Deployment)

	for i, _ := range sr.Objects {
		o := &sr.Objects[i]
		_, o.Rendered = s.checkObjectAccess(c, user, o.Rendered, "rendered")
		_, o.Remote = s.checkObjectAccess(c, user, o.Remote, "remote")
		_, o.Applied = s.checkObjectAccess(c, user, o.Applied, "applied")
	}

	c.JSON(http.StatusOK, sr)
}

func (s *CommandResultsServer) getCommandResultObject(c *gin.Context) {
	var params resultIdParam
	var ref refParam
	var objectType objectTypeParams

	err := c.Bind(&params)
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}
	err = c.Bind(&ref)
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}
	err = c.Bind(&objectType)
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	sr, err := s.store.GetCommandResult(results.GetCommandResultOptions{
		Id:      params.ResultId,
		Reduced: false,
	})
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}
	if sr == nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	ref2 := ref.toK8sRef()

	var found *result.ResultObject
	for _, o := range sr.Objects {
		if o.Ref == ref2 {
			found = &o
			break
		}
	}
	if found == nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	user := s.auth.getUser(c)

	var ok bool
	var o2 *uo.UnstructuredObject
	switch objectType.ObjectType {
	case "rendered":
		ok, o2 = s.checkObjectAccess(c, user, found.Rendered, objectType.ObjectType)
	case "remote":
		ok, o2 = s.checkObjectAccess(c, user, found.Remote, objectType.ObjectType)
	case "applied":
		ok, o2 = s.checkObjectAccess(c, user, found.Applied, objectType.ObjectType)
	default:
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	if !ok {
		c.AbortWithStatus(http.StatusForbidden)
		return
	} else if o2 == nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	c.JSON(http.StatusOK, o2)
}

func (s *CommandResultsServer) getValidateResult(c *gin.Context) {
	var params resultIdParam

	err := c.Bind(&params)
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	vr, err := s.store.GetValidateResult(results.GetValidateResultOptions{
		Id: params.ResultId,
	})
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}
	if vr == nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	c.JSON(http.StatusOK, vr)
}

func (s *CommandResultsServer) doModifyKluctlDeployment(c *gin.Context, clusterId string, name string, namespace string, update func(obj *kluctlv1.KluctlDeployment) error) {
	user := s.auth.getUser(c)

	ca := s.cam.getForClusterId(clusterId)
	if ca == nil {
		_ = c.AbortWithError(http.StatusNotFound, fmt.Errorf("cluster %s not found", clusterId))
		return
	}

	rbacUser := s.auth.getRbacUser(user)
	kc, err := ca.getClient(rbacUser, nil)
	if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	var kd kluctlv1.KluctlDeployment
	err = kc.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, &kd)
	if err != nil {
		if errors.IsNotFound(err) {
			_ = c.AbortWithError(http.StatusNotFound, err)
			return
		}
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	patch := client.MergeFrom(kd.DeepCopy())

	err = update(&kd)
	if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	err = kc.Patch(ctx, &kd, patch, client.FieldOwner(webuiManager))
	if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.Status(http.StatusOK)
}

type KluctlDeploymentParam struct {
	Cluster   string `json:"cluster" form:"cluster"`
	Name      string `json:"name" form:"name"`
	Namespace string `json:"namespace" form:"namespace"`
}

func (s *CommandResultsServer) doSetAnnotation(c *gin.Context, aname string, avalue string) {
	var params KluctlDeploymentParam
	err := c.Bind(&params)
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}
	s.doModifyKluctlDeployment(c, params.Cluster, params.Name, params.Namespace, func(obj *kluctlv1.KluctlDeployment) error {
		metav1.SetMetaDataAnnotation(&obj.ObjectMeta, aname, avalue)
		return nil
	})
}

func (s *CommandResultsServer) validateNow(c *gin.Context) {
	s.doSetAnnotation(c, kluctlv1.KluctlRequestValidateAnnotation, time.Now().Format(time.RFC3339Nano))
}

func (s *CommandResultsServer) reconcileNow(c *gin.Context) {
	s.doSetAnnotation(c, kluctlv1.KluctlRequestReconcileAnnotation, time.Now().Format(time.RFC3339Nano))
}

func (s *CommandResultsServer) deployNow(c *gin.Context) {
	s.doSetAnnotation(c, kluctlv1.KluctlRequestDeployAnnotation, time.Now().Format(time.RFC3339Nano))
}

func (s *CommandResultsServer) setSuspended(c *gin.Context) {
	var params struct {
		KluctlDeploymentParam
		Suspend bool `json:"suspend"`
	}
	err := c.Bind(&params)
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	s.doModifyKluctlDeployment(c, params.Cluster, params.Name, params.Namespace, func(obj *kluctlv1.KluctlDeployment) error {
		obj.Spec.Suspend = params.Suspend
		return nil
	})
}

func (s *CommandResultsServer) setManualObjectsHash(c *gin.Context) {
	var params struct {
		KluctlDeploymentParam
		ObjectsHash string `json:"objectsHash"`
	}
	err := c.Bind(&params)
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	s.doModifyKluctlDeployment(c, params.Cluster, params.Name, params.Namespace, func(obj *kluctlv1.KluctlDeployment) error {
		if params.ObjectsHash == "" {
			obj.Spec.ManualObjectsHash = nil
		} else {
			obj.Spec.ManualObjectsHash = &params.ObjectsHash
		}
		return nil
	})
}
