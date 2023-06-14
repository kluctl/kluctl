package webui

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	corev1 "k8s.io/api/core/v1"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
	"time"
)

const (
	tokenValidTime      = 1 * time.Hour
	tokenMaxRefreshTime = 2 * time.Hour
)

type login struct {
	Username string `form:"username" json:"username" binding:"required"`
	Password string `form:"password" json:"password" binding:"required"`
}

type authHandler struct {
	ctx context.Context

	adminEnabled bool

	serverClient    client.Client
	webuiSecretName string
}

type User struct {
	Username string `json:"username"`
	IsAdmin  bool   `json:"isAdmin"`
}

func (s *authHandler) setupAuth(r gin.IRouter) error {
	r.POST("/auth/login", s.loginHandler)
	r.POST("/auth/refresh", s.refreshTokenHandler)
	r.GET("/auth/user", s.authHandler, s.userHandler)

	return nil
}

func (s *authHandler) loginHandler(c *gin.Context) {
	if !s.adminEnabled {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	var loginVals login
	if err := c.ShouldBind(&loginVals); err != nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	userID := loginVals.Username
	password := loginVals.Password

	as, err := s.getAdminSecrets()
	if err != nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	if userID != "admin" || password != as.adminPassword {
		// we only support the admin account at the moment
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	token, err := s.createAdminToken(userID, as)
	if err != nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	c.JSON(http.StatusOK, map[string]any{
		"token": token,
	})
}

func (s *authHandler) refreshTokenHandler(c *gin.Context) {
	if !s.adminEnabled {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	oldToken := s.getBearerToken(c)
	if oldToken == "" {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	as, err := s.getAdminSecrets()
	if err != nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	claims, err := s.checkIfAdminTokenExpired(oldToken, as)
	if err != nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	user := s.getAdminUserFromClaims(claims)

	newToken, err := s.createAdminToken(user.Username, as)
	if err != nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	c.JSON(http.StatusOK, map[string]any{
		"token": newToken,
	})
}

func (s *authHandler) createAdminToken(username string, as adminSecrets) (string, error) {
	// Create the token
	token := jwt.New(jwt.SigningMethodHS256)
	claims := token.Claims.(jwt.MapClaims)
	claims["id"] = username

	expire := time.Now().Add(tokenValidTime)
	claims["exp"] = expire.Unix()
	claims["orig_iat"] = time.Now().Unix()
	tokenString, err := token.SignedString(as.secret)
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

func (s *authHandler) checkIfAdminTokenExpired(token string, as adminSecrets) (jwt.MapClaims, error) {
	jt, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		return as.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}))
	if err != nil {
		validationErr, ok := err.(*jwt.ValidationError)
		if !ok || validationErr.Errors != jwt.ValidationErrorExpired {
			return nil, err
		}
	}

	claims := jt.Claims.(jwt.MapClaims)

	origIat := int64(claims["orig_iat"].(float64))

	if origIat < time.Now().Add(-tokenMaxRefreshTime).Unix() {
		return nil, fmt.Errorf("token expired")
	}

	return claims, nil
}

func (s *authHandler) getBearerToken(c *gin.Context) string {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		return ""
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		c.AbortWithStatus(http.StatusUnauthorized)
		return ""
	}

	return parts[1]
}

func (s *authHandler) getUser(c *gin.Context) *User {
	if s == nil {
		return nil
	}
	token := s.getBearerToken(c)
	if token == "" {
		return nil
	}
	return s.getUserFromToken(token)
}

func (s *authHandler) getUserFromToken(token string) *User {
	user := s.getAdminUser(token)
	if user != nil {
		return user
	}
	return nil
}

func (s *authHandler) getAdminUser(token string) *User {
	as, err := s.getAdminSecrets()
	if err != nil {
		return nil
	}

	jt, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		return as.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}))
	if err != nil {
		return nil
	}

	claims, ok := jt.Claims.(jwt.MapClaims)
	if !ok {
		return nil
	}

	return s.getAdminUserFromClaims(claims)
}

func (s *authHandler) getAdminUserFromClaims(claims jwt.MapClaims) *User {
	x, ok := claims["id"]
	if !ok {
		return nil
	}
	id, ok := x.(string)
	if !ok {
		return nil
	}

	return &User{
		Username: id,
		IsAdmin:  true,
	}
}

func (s *authHandler) authHandler(c *gin.Context) {
	if s == nil {
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

type adminSecrets struct {
	secret        []byte
	adminPassword string
}

func (s *authHandler) getAdminSecrets() (adminSecrets, error) {
	if s.serverClient == nil {
		return adminSecrets{}, fmt.Errorf("no serverClient set")
	}

	var secret corev1.Secret
	err := s.serverClient.Get(s.ctx, client.ObjectKey{Name: s.webuiSecretName, Namespace: "kluctl-system"}, &secret)
	if err != nil {
		return adminSecrets{}, err
	}

	var ret adminSecrets

	x, ok := secret.Data["secret"]
	if !ok {
		return adminSecrets{}, fmt.Errorf("admin secret %s has no 'secret' key", s.webuiSecretName)
	}
	ret.secret = x

	x, ok = secret.Data["adminPassword"]
	if !ok {
		return adminSecrets{}, fmt.Errorf("admin secret %s has no adminPassword", s.webuiSecretName)
	}
	ret.adminPassword = string(x)

	return ret, nil
}
