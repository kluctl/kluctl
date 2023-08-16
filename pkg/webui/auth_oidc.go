package webui

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"golang.org/x/oauth2"
	"net/http"
	"strings"
	"time"
)

type oidcTokenInfo struct {
	RefreshToken string
	Expiry       time.Time
	User         *User
}

func (s *authHandler) setupOidcProvider(ctx context.Context, authConfig AuthConfig) error {
	if authConfig.OidcIssuerUrl == "" {
		return nil
	}

	clientSecret, err := s.getSecret(authConfig.OidcClientSecretName, authConfig.OidcClientSecretKey)
	if err != nil {
		return err
	}

	provider, err := oidc.NewProvider(ctx, authConfig.OidcIssuerUrl)
	if err != nil {
		return err
	}
	s.oidcProvider = provider

	s.oauth2Config = &oauth2.Config{
		ClientID:     authConfig.OidcClientId,
		ClientSecret: clientSecret,
		RedirectURL:  authConfig.OidcRedirectUrl,
		Scopes:       authConfig.OidcScopes,
		Endpoint:     provider.Endpoint(),
	}

	return nil
}

// verifyIDToken verifies that an *oauth2.Token is a valid *oidc.IDToken.
func (s *authHandler) verifyIDToken(ctx context.Context, rawIDToken string) (*oidc.IDToken, error) {
	oidcConfig := &oidc.Config{
		ClientID: s.oauth2Config.ClientID,
	}

	return s.oidcProvider.Verifier(oidcConfig).Verify(ctx, rawIDToken)
}

func (s *authHandler) oidcLoginHandler(c *gin.Context) {
	state, err := s.generateRandomState()
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// Save the state inside the session.
	session := sessions.Default(c)
	session.Set("state", state)
	if err := session.Save(); err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	var opts []oauth2.AuthCodeOption
	for _, x := range s.authConfig.OidcParams {
		s := strings.SplitN(x, "=", 2)
		if len(s) != 2 {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		opts = append(opts, oauth2.SetAuthURLParam(s[0], s[1]))
	}

	c.Redirect(http.StatusTemporaryRedirect, s.oauth2Config.AuthCodeURL(state, opts...))
}

func (s *authHandler) storeToken(c *gin.Context, session sessions.Session, token *oauth2.Token) error {
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return fmt.Errorf("no id_token field in oauth2 token")
	}

	idToken, err := s.verifyIDToken(c.Request.Context(), rawIDToken)
	if err != nil {
		return err
	}

	user, err := s.getUserFromIDToken(idToken)
	if err != nil {
		return err
	}

	tokenInfo := oidcTokenInfo{
		RefreshToken: token.RefreshToken,
		Expiry:       idToken.Expiry,
		User:         user,
	}

	session.Set("oidcToken", tokenInfo)

	return nil
}

func (s *authHandler) handleOidcCallback(c *gin.Context) (int, string) {
	session := sessions.Default(c)
	if c.Query("state") != session.Get("state") {
		return http.StatusBadRequest, "invalid state parameter"
	}

	session.Delete("state")

	// Exchange an authorization code for a token.
	token, err := s.oauth2Config.Exchange(c.Request.Context(), c.Query("code"))
	if err != nil {
		return http.StatusUnauthorized, err.Error()
	}

	err = s.storeToken(c, session, token)
	if err != nil {
		return http.StatusUnauthorized, err.Error()
	}

	if err := session.Save(); err != nil {
		return http.StatusInternalServerError, err.Error()
	}

	return http.StatusOK, ""
}

func (s *authHandler) oidcCallbackHandler(c *gin.Context) {
	status, errMsg := s.handleOidcCallback(c)
	if status != http.StatusOK {
		c.String(status, errMsg)
		return
	}

	// Redirect to logged in page.
	c.Redirect(http.StatusTemporaryRedirect, "/")
}

func (s *authHandler) oidcRefresh(c *gin.Context, session sessions.Session, tokenInfo *oidcTokenInfo) error {
	ts := s.oauth2Config.TokenSource(c, &oauth2.Token{
		RefreshToken: tokenInfo.RefreshToken,
	})
	newToken, err := ts.Token()
	if err != nil {
		return err
	}

	return s.storeToken(c, session, newToken)
}

func (s *authHandler) generateRandomState() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}

	state := base64.StdEncoding.EncodeToString(b)

	return state, nil
}

func (s *authHandler) getUserFromOidcTokenInfo(c *gin.Context, allowRefresh bool) (*User, error) {
	session := sessions.Default(c)

	tokenInfoI := session.Get("oidcToken")
	if tokenInfoI == nil {
		return nil, nil
	}
	tokenInfo, ok := tokenInfoI.(oidcTokenInfo)
	if !ok {
		return nil, nil
	}

	if tokenInfo.Expiry.Before(time.Now().Add(10 * time.Minute)) {
		if !allowRefresh {
			return nil, fmt.Errorf("token expired")
		}

		err := s.oidcRefresh(c, session, &tokenInfo)
		if err != nil {
			return nil, err
		}

		err = session.Save()
		if err != nil {
			return nil, err
		}
		return s.getUserFromOidcTokenInfo(c, false)
	}

	return tokenInfo.User, nil
}

func (s *authHandler) getUserFromIDToken(idToken *oidc.IDToken) (*User, error) {
	var profile map[string]interface{}
	if err := idToken.Claims(&profile); err != nil {
		return nil, err
	}

	usernameClaim, ok := profile[s.authConfig.OidcUserClaim]
	if !ok {
		return nil, fmt.Errorf("missing or invalid user claim")
	}
	username, ok := usernameClaim.(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid user claim")
	}

	groupsClaim, ok := profile[s.authConfig.OidcGroupClaim]
	if !ok {
		return nil, fmt.Errorf("missing or invalid groups claim")
	}
	groups, ok := groupsClaim.([]any)
	if !ok {
		return nil, fmt.Errorf("missing or invalid groups claim")
	}

	isAdmin := false
	isViewer := false

	for _, group := range groups {
		g, ok := group.(string)
		if !ok {
			return nil, fmt.Errorf("missing or invalid groups claim")
		}
		if utils.FindStrInSlice(s.authConfig.OidcAdminsGroups, g) != -1 {
			isAdmin = true
		}
		if utils.FindStrInSlice(s.authConfig.OidcViewersGroups, g) != -1 {
			isViewer = true
		}
	}

	if isAdmin {
		return s.getAdminUser(username), nil
	} else if isViewer {
		return s.getViewerUser(username), nil
	} else {
		return nil, fmt.Errorf("permission denied")
	}
}
