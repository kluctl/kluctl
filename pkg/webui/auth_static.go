package webui

import (
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
)

const (
	staticLoginExpireTime  = 1 * time.Hour
	staticLoginRefreshTime = 10 * time.Second
)

func (s *authHandler) staticLoginHandler(c *gin.Context) {
	if !s.authConfig.StaticLoginEnabled {
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

	// we only support the admin and viewer accounts at the moment
	if userID == "admin" {
		if password != s.adminPassword {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
	} else if userID == "viewer" {
		if password != s.viewerPassword {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
	} else {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	session := sessions.Default(c)
	session.Set("staticUser", userID)
	session.Set("staticLoginTime", time.Now())

	if !s.checkIfStaticLoginExpired(session) {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	if err := session.Save(); err != nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	c.JSON(http.StatusOK, "ok")
}

func (s *authHandler) checkIfStaticLoginExpired(session sessions.Session) bool {
	staticLoginTimeI := session.Get("staticLoginTime")
	if staticLoginTimeI == nil {
		return false
	}
	staticLoginTime, ok := staticLoginTimeI.(time.Time)
	if !ok {
		return false
	}

	if time.Now().After(staticLoginTime.Add(staticLoginExpireTime)) {
		return false
	}

	if time.Now().After(staticLoginTime.Add(staticLoginRefreshTime)) {
		session.Set("staticLoginTime", time.Now())
	}

	return true
}

func (s *authHandler) getStaticUserFromSession(c *gin.Context) *User {
	if !s.authConfig.StaticLoginEnabled {
		return nil
	}

	session := sessions.Default(c)
	usernameI := session.Get("staticUser")
	if usernameI == nil {
		return nil
	}

	username, ok := usernameI.(string)
	if !ok {
		return nil
	}

	if !s.checkIfStaticLoginExpired(session) {
		return nil
	}

	if err := session.Save(); err != nil {
		return nil
	}

	if username == "admin" {
		return s.getAdminUser(username)
	} else if username == "viewer" {
		return s.getViewerUser(username)
	} else {
		return nil
	}
}

func (s *authHandler) getAdminUser(id string) *User {
	u := &User{
		Username: id,
		IsAdmin:  true,
	}
	return u
}

func (s *authHandler) getViewerUser(id string) *User {
	u := &User{
		Username: id,
		IsAdmin:  false,
	}
	return u
}
