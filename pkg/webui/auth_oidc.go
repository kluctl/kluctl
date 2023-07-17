package webui

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"golang.org/x/oauth2"
	"net/http"
	"strings"
)

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
func (s *authHandler) verifyIDToken(ctx context.Context, token *oauth2.Token) (*oidc.IDToken, error) {
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, errors.New("no id_token field in oauth2 token")
	}

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

func (s *authHandler) oidcCallbackHandler(c *gin.Context) {
	session := sessions.Default(c)
	if c.Query("state") != session.Get("state") {
		c.String(http.StatusBadRequest, "Invalid state parameter.")
		return
	}

	// Exchange an authorization code for a token.
	token, err := s.oauth2Config.Exchange(c.Request.Context(), c.Query("code"))
	if err != nil {
		c.String(http.StatusUnauthorized, "Failed to exchange an authorization code for a token.")
		return
	}

	idToken, err := s.verifyIDToken(c.Request.Context(), token)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to verify ID Token.")
		return
	}

	var profile map[string]interface{}
	if err := idToken.Claims(&profile); err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	session.Set("access_token", token.AccessToken)
	session.Set("profile", profile)
	if err := session.Save(); err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// Redirect to logged in page.
	c.Redirect(http.StatusTemporaryRedirect, "/")
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

func (s *authHandler) getUserFromOidcProfile(c *gin.Context) (*User, error) {
	session := sessions.Default(c)
	profileI := session.Get("profile")
	if profileI == nil {
		return nil, nil
	}
	profile := profileI.(map[string]interface{})
	if profile == nil {
		return nil, nil
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
