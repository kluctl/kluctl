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

type CommandResultsServer struct {
	ctx       context.Context
	collector *results.ResultsCollector
	cam       *clusterAccessorManager
	vam       *validatorManager
}

func NewCommandResultsServer(ctx context.Context, collector *results.ResultsCollector, configs []*rest.Config) *CommandResultsServer {
	ret := &CommandResultsServer{
		ctx:       ctx,
		collector: collector,
		cam: &clusterAccessorManager{
			ctx: ctx,
		},
	}

	for _, config := range configs {
		ret.cam.add(config)
	}

	ret.vam = newValidatorManager(ctx, collector, ret.cam)

	return ret
}

func (s *CommandResultsServer) Run(port int) error {
	_, _ = s.collector.WatchCommandResultSummaries(results.ListCommandResultSummariesOptions{}, func(summary *result.CommandResultSummary) {
		status.Info(s.ctx, "Updated result summary for %s", summary.Id)
	}, func(id string) {
		status.Info(s.ctx, "Deleted result summary for %s", id)
	})

	s.cam.start()
	s.vam.start()

	router := gin.Default()

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

	api := router.Group("/api")
	api.GET("/getShortNames", s.getShortNames)
	api.GET("/listProjects", s.listProjects)
	api.GET("/listResults", s.listResults)
	api.GET("/getResult", s.getResult)
	api.GET("/getResultObject", s.getResultObject)
	api.POST("/validateNow", s.validateNow)
	api.POST("/reconcileNow", s.reconcileNow)
	api.POST("/deployNow", s.deployNow)

	address := fmt.Sprintf(":%d", port)
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

func (s *CommandResultsServer) getShortNames(c *gin.Context) {
	c.JSON(http.StatusOK, GetShortNames())
}

func (s *CommandResultsServer) listResults(c *gin.Context) {
	args := struct {
		FilterProject string `form:"filterProject"`
		FilterSubDir  string `form:"filterSubDir"`
	}{}
	err := c.BindQuery(&args)
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	repoKey, err := types.ParseGitRepoKey(args.FilterProject)
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	var filter *result.ProjectKey
	if args.FilterProject != "" {
		filter = &result.ProjectKey{
			GitRepoKey: repoKey,
			SubDir:     args.FilterSubDir,
		}
	}

	summaries, err := s.collector.ListCommandResultSummaries(results.ListCommandResultSummariesOptions{
		ProjectFilter: filter,
	})
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	c.JSON(http.StatusOK, summaries)
}

func (s *CommandResultsServer) listProjects(c *gin.Context) {
	summaries, err := s.collector.ListCommandResultSummaries(results.ListCommandResultSummariesOptions{})
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	projects := result.BuildProjectSummaries(summaries)

	for _, p := range projects {
		for _, t := range p.Targets {
			key := projectTargetKey{
				Project: p.Project,
				Target:  t.Target,
			}
			vr, err := s.vam.getValidateResult(key)
			if err == nil {
				t.LastValidateResult = vr
			}
		}
	}

	c.JSON(http.StatusOK, projects)
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

func (s *CommandResultsServer) getResult(c *gin.Context) {
	var params resultIdParam

	err := c.Bind(&params)
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	sr, err := s.collector.GetCommandResult(results.GetCommandResultOptions{
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

func (s *CommandResultsServer) getResultObject(c *gin.Context) {
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

	sr, err := s.collector.GetCommandResult(results.GetCommandResultOptions{
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

func (s *CommandResultsServer) validateNow(c *gin.Context) {
	var params projectTargetKey
	err := c.Bind(&params)
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	key := projectTargetKey{
		Project: params.Project,
		Target:  params.Target,
	}

	if !s.vam.validateNow(key) {
		_ = c.AbortWithError(http.StatusNotFound, err)
		return
	}

	c.Status(http.StatusOK)
}

type kluctlDeploymentParam struct {
	Cluster   string `json:"cluster"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

func (s *CommandResultsServer) doSetAnnotation(c *gin.Context, aname string, avalue string) {
	var params kluctlDeploymentParam
	err := c.Bind(&params)
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	ca := s.cam.getForClusterId(params.Cluster)
	if ca == nil {
		_ = c.AbortWithError(http.StatusNotFound, err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	var kd kluctlv1.KluctlDeployment
	err = ca.getClient().Get(ctx, client.ObjectKey{Name: params.Name, Namespace: params.Namespace}, &kd)
	if err != nil {
		if errors.IsNotFound(err) {
			_ = c.AbortWithError(http.StatusNotFound, err)
			return
		}
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	patch := client.MergeFrom(kd.DeepCopy())
	metav1.SetMetaDataAnnotation(&kd.ObjectMeta, aname, avalue)
	err = ca.getClient().Patch(ctx, &kd, patch, client.FieldOwner(webuiManager))
	if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.Status(http.StatusOK)
}

func (s *CommandResultsServer) reconcileNow(c *gin.Context) {
	s.doSetAnnotation(c, kluctlv1.KluctlRequestReconcileAnnotation, time.Now().Format(time.RFC3339Nano))
}

func (s *CommandResultsServer) deployNow(c *gin.Context) {
	s.doSetAnnotation(c, kluctlv1.KluctlRequestDeployAnnotation, time.Now().Format(time.RFC3339Nano))
}
