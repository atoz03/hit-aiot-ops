package main

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const sessionCookieName = "gpuops_session"

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type registerReq struct {
	Email                  string `json:"email"`
	Username               string `json:"username"`
	Password               string `json:"password"`
	RealName               string `json:"real_name"`
	StudentID              string `json:"student_id"`
	Advisor                string `json:"advisor"`
	ExpectedGraduationYear int    `json:"expected_graduation_year"`
	Phone                  string `json:"phone"`
}

type forgotPasswordReq struct {
	Email string `json:"email"`
}

type resetPasswordReq struct {
	Username    string `json:"username"`
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

type changePasswordReq struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type mailSettingsReq struct {
	SMTPHost   string `json:"smtp_host"`
	SMTPPort   int    `json:"smtp_port"`
	SMTPUser   string `json:"smtp_user"`
	SMTPPass   string `json:"smtp_pass"`
	UpdatePass bool   `json:"update_pass"`
	FromEmail  string `json:"from_email"`
	FromName   string `json:"from_name"`
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
	role := "admin"
	if !ok {
		ok, err = s.store.VerifyUserPassword(c.Request.Context(), req.Username, req.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_credentials"})
			return
		}
		role = "user"
	}

	secret := strings.TrimSpace(s.cfg.AuthSecret)
	exp := time.Now().Add(time.Duration(s.cfg.SessionHours) * time.Hour).Unix()
	token, err := signSession(secret, sessionPayload{
		Username: req.Username,
		Role:     role,
		ExpUnix:  exp,
		Nonce:    newNonce(),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.SetCookie(sessionCookieName, token, int(time.Duration(s.cfg.SessionHours)*time.Hour/time.Second), "/", "", s.cfg.CookieSecure, true)
	c.JSON(http.StatusOK, gin.H{"ok": true, "role": role})
}

func (s *Server) handleAuthLogout(c *gin.Context) {
	// MaxAge=-1 让浏览器删除 cookie
	c.SetCookie(sessionCookieName, "", -1, "/", "", s.cfg.CookieSecure, true)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) handleAuthRegister(c *gin.Context) {
	var req registerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Username = strings.TrimSpace(req.Username)
	if req.Email == "" || req.Username == "" || req.Password == "" || strings.TrimSpace(req.RealName) == "" ||
		strings.TrimSpace(req.StudentID) == "" || strings.TrimSpace(req.Advisor) == "" || strings.TrimSpace(req.Phone) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请完整填写注册信息"})
		return
	}
	if len(req.Password) < 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "密码至少 8 位"})
		return
	}
	ctx := c.Request.Context()
	err := s.store.WithTx(ctx, func(tx *sql.Tx) error {
		return s.store.CreateUserAccountTx(ctx, tx, UserAccount{
			Username:               req.Username,
			Email:                  req.Email,
			RealName:               req.RealName,
			StudentID:              req.StudentID,
			Advisor:                req.Advisor,
			ExpectedGraduationYear: req.ExpectedGraduationYear,
			Phone:                  req.Phone,
		}, req.Password, s.cfg.DefaultBalance)
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) handleAuthForgotPassword(c *gin.Context) {
	var req forgotPasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	email := strings.TrimSpace(strings.ToLower(req.Email))
	if email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email 不能为空"})
		return
	}

	rawToken, tokenHash, err := newResetToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	expireAt := time.Now().Add(30 * time.Minute)
	username, found, err := s.store.SetPasswordResetTokenByEmail(c.Request.Context(), email, tokenHash, expireAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 为避免枚举账号，无论是否存在都返回 ok。
	if !found {
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}

	settings, err := s.store.GetMailSettings(c.Request.Context(), s.cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	resetURL := buildResetURL(c, username, rawToken)
	subject := "HIT-AIOT-OPS 密码找回"
	body := fmt.Sprintf("您好，\n\n请在 30 分钟内访问以下链接重置密码：\n%s\n\n若非本人操作，请忽略此邮件。\n\nHIT-AIOT-OPS团队", resetURL)
	if err := sendResetPasswordMail(settings, email, subject, body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "发送邮件失败，请联系管理员检查 SMTP 配置: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) handleAuthResetPassword(c *gin.Context) {
	var req resetPasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || strings.TrimSpace(req.Token) == "" || req.NewPassword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数不完整"})
		return
	}
	tokenHash := sha256Hex(strings.TrimSpace(req.Token))
	if err := s.store.ResetPasswordByToken(c.Request.Context(), req.Username, tokenHash, req.NewPassword, time.Now()); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) handleAuthChangePassword(c *gin.Context) {
	var req changePasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	username, _ := c.Get("auth_user")
	role, _ := c.Get("auth_role")
	userStr := strings.TrimSpace(fmt.Sprintf("%v", username))
	roleStr := strings.TrimSpace(fmt.Sprintf("%v", role))
	if userStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var err error
	if roleStr == "admin" {
		err = s.store.UpdateAdminPassword(c.Request.Context(), userStr, req.CurrentPassword, req.NewPassword)
	} else {
		err = s.store.UpdateUserPassword(c.Request.Context(), userStr, req.CurrentPassword, req.NewPassword)
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) handleAdminMailSettingsGet(c *gin.Context) {
	settings, err := s.store.GetMailSettings(c.Request.Context(), s.cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"smtp_host":         settings.SMTPHost,
		"smtp_port":         settings.SMTPPort,
		"smtp_user":         settings.SMTPUser,
		"smtp_password_set": strings.TrimSpace(settings.SMTPPass) != "",
		"from_email":        settings.FromEmail,
		"from_name":         settings.FromName,
	})
}

func (s *Server) handleAdminMailSettingsSet(c *gin.Context) {
	var req mailSettingsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	settings := MailSettings{
		SMTPHost:  req.SMTPHost,
		SMTPPort:  req.SMTPPort,
		SMTPUser:  req.SMTPUser,
		SMTPPass:  req.SMTPPass,
		FromEmail: req.FromEmail,
		FromName:  req.FromName,
	}
	if err := s.store.UpsertMailSettings(c.Request.Context(), settings, req.UpdatePass); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
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

func newResetToken() (string, string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", "", err
	}
	token := base64.RawURLEncoding.EncodeToString(b[:])
	return token, sha256Hex(token), nil
}

func sha256Hex(v string) string {
	sum := sha256.Sum256([]byte(v))
	return hex.EncodeToString(sum[:])
}

func buildResetURL(c *gin.Context, username string, token string) string {
	scheme := "http"
	if strings.EqualFold(strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")), "https") || c.Request.TLS != nil {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s/reset-password?username=%s&token=%s",
		scheme,
		c.Request.Host,
		url.QueryEscape(username),
		url.QueryEscape(token),
	)
}
