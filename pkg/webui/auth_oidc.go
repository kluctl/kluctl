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

func (s *authHandler) storeToken(session sessions.Session, token *oauth2.Token) error {
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return fmt.Errorf("no id_token field in oauth2 token.")
	}

	session.Set("token", token)
	session.Set("id_token", rawIDToken)

	return nil
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

	err = s.storeToken(session, token)
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

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

	tokenI := session.Get("token")
	if tokenI == nil {
		return nil, nil
	}
	token, ok := tokenI.(oauth2.Token)
	if !ok {
		return nil, nil
	}

	rawIdTokenI := session.Get("id_token")
	if rawIdTokenI == nil {
		return nil, nil
	}
	rawIdToken, ok := rawIdTokenI.(string)
	if !ok {
		return nil, nil
	}

	if !token.Valid() {
		return nil, nil
	}

	idToken, err := s.verifyIDToken(c.Request.Context(), rawIdToken)
	if err != nil {
		return nil, err
	}

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
