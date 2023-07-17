package webui

import (
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
)

const (
	adminExpireTime  = 1 * time.Hour
	adminRefreshTime = 10 * time.Second
)

func (s *authHandler) adminLoginHandler(c *gin.Context) {
	if !s.authConfig.AdminEnabled {
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

	if userID != "admin" || password != s.adminPassword {
		// we only support the admin account at the moment
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	session := sessions.Default(c)
	session.Set("adminUser", userID)
	session.Set("adminTime", time.Now())

	if !s.checkIfAdminExpired(session) {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	if err := session.Save(); err != nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	c.JSON(http.StatusOK, "ok")
}

func (s *authHandler) checkIfAdminExpired(session sessions.Session) bool {
	adminTimeI := session.Get("adminTime")
	if adminTimeI == nil {
		return false
	}
	adminTime, ok := adminTimeI.(time.Time)
	if !ok {
		return false
	}

	if time.Now().After(adminTime.Add(adminExpireTime)) {
		return false
	}

	if time.Now().After(adminTime.Add(adminRefreshTime)) {
		session.Set("adminTime", time.Now())
	}

	return true
}

func (s *authHandler) getAdminUserFromSession(c *gin.Context) *User {
	if !s.authConfig.AdminEnabled {
		return nil
	}

	session := sessions.Default(c)
	usernameI := session.Get("adminUser")
	if usernameI == nil {
		return nil
	}

	username, ok := usernameI.(string)
	if !ok {
		return nil
	}

	if !s.checkIfAdminExpired(session) {
		return nil
	}

	if err := session.Save(); err != nil {
		return nil
	}

	return s.getAdminUser(username)
}

func (s *authHandler) getAdminUser(id string) *User {
	u := &User{
		Username: id,
		IsAdmin:  true,
	}
	u.RbacUser = s.authConfig.AdminRbacUser
	return u
}

func (s *authHandler) getViewerUser(id string) *User {
	u := &User{
		Username: id,
		IsAdmin:  false,
	}
	u.RbacUser = s.authConfig.ViewerRbacUser
	return u
}
