package webui

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	kluctlv1 "github.com/kluctl/kluctl/v2/api/v1beta1"
	"github.com/kluctl/kluctl/v2/pkg/results"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"io/fs"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"net"
	"net/http"
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
	serverClient client.Client

	auth   *authHandler
	events *eventsHandler

	onlyApi bool
}

func NewCommandResultsServer(ctx context.Context, store *results.ResultsCollector, configs []*rest.Config, serverClient client.Client, authEnabled bool, onlyApi bool) *CommandResultsServer {
	ret := &CommandResultsServer{
		ctx:   ctx,
		store: store,
		cam: &clusterAccessorManager{
			ctx: ctx,
		},
		serverClient: serverClient,
		onlyApi:      onlyApi,
	}

	ret.events = newEventsHandler(ret)

	adminUser := "kluctl-webui-admin"

	adminEnabled := false
	if serverClient != nil {
		adminEnabled = true
	}

	if authEnabled {
		ret.auth = &authHandler{
			ctx:             ctx,
			adminEnabled:    adminEnabled,
			serverClient:    serverClient,
			webuiSecretName: "admin-secret",
			adminRbacUser:   adminUser,
		}
	}

	for _, config := range configs {
		ret.cam.add(config)
	}

	return ret
}

func (s *CommandResultsServer) Run(host string, port int) error {
	err := s.startUpdateLogs()
	if err != nil {
		return err
	}

	s.cam.start()

	router := gin.New()
	router.Use(gin.LoggerWithConfig(gin.LoggerConfig{
		SkipPaths: []string{"/healthz", "/readyz"},
	}))
	router.Use(gin.Recovery())

	if s.auth != nil {
		err = s.auth.setupAuth(router)
		if err != nil {
			return err
		}
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

	err = s.events.startEventsWatcher()
	if err != nil {
		return err
	}
	api.GET("/events", s.events.handler)

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

	var o2 *uo.UnstructuredObject
	switch objectType.ObjectType {
	case "rendered":
		o2 = found.Rendered
	case "remote":
		o2 = found.Remote
	case "applied":
		o2 = found.Applied
	}
	if o2 == nil {
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

	kc, err := ca.getClient(user.RbacUser, nil)
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
	Cluster   string `json:"cluster"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
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
