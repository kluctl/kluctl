package webui

import (
	"context"
	"encoding/gob"
	"fmt"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	"golang.org/x/oauth2"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

type AuthConfig struct {
	AuthEnabled    bool
	AuthSecretName string
	AuthSecretKey  string

	AdminEnabled    bool
	AdminSecretName string
	AdminSecretKey  string
	AdminRbacUser   string
	ViewerRbacUser  string

	OidcIssuerUrl        string
	OidcDisplayName      string
	OidcClientId         string
	OidcClientSecretName string
	OidcClientSecretKey  string
	OidcRedirectUrl      string
	OidcScopes           []string
	OidcParams           []string
	OidcUserClaim        string
	OidcGroupClaim       string
	OidcAdminsGroups     []string
	OidcViewersGroups    []string
}

type login struct {
	Username string `form:"username" json:"username" binding:"required"`
	Password string `form:"password" json:"password" binding:"required"`
}

type authHandler struct {
	ctx context.Context

	serverClient client.Client

	authConfig AuthConfig

	authSecret    []byte
	adminPassword string

	oidcProvider *oidc.Provider
	oauth2Config *oauth2.Config
}

type User struct {
	Username string `json:"username"`
	IsAdmin  bool   `json:"isAdmin"`
	RbacUser string `json:"RbacUser"`
}

func newAuthHandler(ctx context.Context, serverClient client.Client, authConfig AuthConfig) (*authHandler, error) {
	ret := &authHandler{
		ctx:          ctx,
		authConfig:   authConfig,
		serverClient: serverClient,
	}

	x, err := ret.getSecret(authConfig.AuthSecretName, authConfig.AuthSecretKey)
	if err != nil {
		return nil, err
	}
	ret.authSecret = []byte(x)

	if authConfig.AdminEnabled {
		x, err = ret.getSecret(authConfig.AdminSecretName, authConfig.AdminSecretKey)
		if err != nil {
			return nil, err
		}
		ret.adminPassword = x
	}

	err = ret.setupOidcProvider(ctx, authConfig)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func (s *authHandler) setupRoutes(router gin.IRouter) error {
	gob.Register(map[string]interface{}{})
	gob.Register(time.Time{})

	store := cookie.NewStore(s.authSecret)
	router.Use(sessions.Sessions("auth-session", store))

	router.GET("/auth/info", s.authInfoHandler)

	if s.authConfig.AdminEnabled {
		router.POST("/auth/adminLogin", s.adminLoginHandler)
	}

	if s.oidcProvider != nil {
		router.GET("/auth/login", s.oidcLoginHandler)
		router.GET("/auth/callback", s.oidcCallbackHandler)
	}

	router.GET("/auth/user", s.authHandler, s.userHandler)
	router.GET("/auth/logout", s.logoutHandler)

	return nil
}

type AuthInfo struct {
	AuthEnabled  bool `json:"authEnabled"`
	AdminEnabled bool `json:"adminEnabled"`

	OidcEnabled     bool   `json:"oidcEnabled"`
	OidcDisplayName string `json:"oidcName,omitempty"`
}

func (s *authHandler) authInfoHandler(c *gin.Context) {
	info := AuthInfo{
		AuthEnabled:     s.authConfig.AuthEnabled,
		AdminEnabled:    s.authConfig.AdminEnabled,
		OidcEnabled:     s.authConfig.OidcIssuerUrl != "",
		OidcDisplayName: s.authConfig.OidcDisplayName,
	}
	c.JSON(http.StatusOK, info)
}

func (s *authHandler) logoutHandler(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	if err := session.Save(); err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.Redirect(http.StatusTemporaryRedirect, "/")
}

func (s *authHandler) getUser(c *gin.Context) *User {
	if !s.authConfig.AuthEnabled {
		// auth is disabled, so all requests are done as admin
		return s.getAdminUser("admin")
	}

	user := s.getAdminUserFromSession(c)
	if user != nil {
		return user
	}

	user, err := s.getUserFromOidcProfile(c)
	if err != nil {
		return nil
	}
	if user != nil {
		return user
	}

	return nil
}

func (s *authHandler) authHandler(c *gin.Context) {
	if !s.authConfig.AuthEnabled {
		return
	}
	if s.getUser(c) == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
	}
}

func (s *authHandler) userHandler(c *gin.Context) {
	user := s.getUser(c)
	c.JSON(http.StatusOK, user)
}

func (s *authHandler) getSecret(secretName string, secretKey string) (string, error) {
	if s.serverClient == nil {
		return "", fmt.Errorf("no serverClient set")
	}
	return k8s.GetSingleSecret(s.ctx, s.serverClient, secretName, "kluctl-system", secretKey)
}
