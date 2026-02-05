package main

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const sessionCookieName = "gpuops_session"

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) handleAuthMe(c *gin.Context) {
	if s.cfg.SessionHours == 0 {
		c.JSON(http.StatusOK, gin.H{"authenticated": false, "session_disabled": true})
		return
	}
	secret := strings.TrimSpace(s.cfg.AuthSecret)
	cookie, err := c.Cookie(sessionCookieName)
	if err != nil || strings.TrimSpace(cookie) == "" {
		c.JSON(http.StatusOK, gin.H{"authenticated": false})
		return
	}
	p, err := verifySession(secret, cookie, time.Now())
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"authenticated": false})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"authenticated": true,
		"username":      p.Username,
		"role":          p.Role,
		"expires_at":    time.Unix(p.ExpUnix, 0).UTC().Format(time.RFC3339),
		"csrf_token":    p.Nonce,
	})
}

func (s *Server) handleAuthLogin(c *gin.Context) {
	if s.cfg.SessionHours == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session_disabled"})
		return
	}

	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username/password 不能为空"})
		return
	}

	ok, err := s.store.VerifyAdminPassword(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_credentials"})
		return
	}

	secret := strings.TrimSpace(s.cfg.AuthSecret)
	exp := time.Now().Add(time.Duration(s.cfg.SessionHours) * time.Hour).Unix()
	token, err := signSession(secret, sessionPayload{
		Username: req.Username,
		Role:     "admin",
		ExpUnix:  exp,
		Nonce:    newNonce(),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.SetCookie(sessionCookieName, token, int(time.Duration(s.cfg.SessionHours)*time.Hour/time.Second), "/", "", s.cfg.CookieSecure, true)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) handleAuthLogout(c *gin.Context) {
	// MaxAge=-1 让浏览器删除 cookie
	c.SetCookie(sessionCookieName, "", -1, "/", "", s.cfg.CookieSecure, true)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type bootstrapReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// handleAdminBootstrap 用于首次创建管理员账号（用于 Web 登录）。
// 规则：仅当 admin_accounts 为空时允许执行，且必须通过 admin_token 调用（避免公开入口）。
func (s *Server) handleAdminBootstrap(c *gin.Context) {
	// 强制要求 Bearer token（禁止用 session 自举，避免逻辑漏洞）
	auth := strings.TrimSpace(c.GetHeader("Authorization"))
	const prefix = "Bearer "
	if !strings.HasPrefix(auth, prefix) || strings.TrimSpace(strings.TrimPrefix(auth, prefix)) != s.cfg.AdminToken {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	count, err := s.store.CountAdminAccounts(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if count > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "already_bootstrapped"})
		return
	}

	var req bootstrapReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username/password 不能为空"})
		return
	}

	if err := s.store.CreateAdminAccount(c.Request.Context(), req.Username, req.Password); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
