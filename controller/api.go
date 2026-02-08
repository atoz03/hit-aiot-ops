package main

import (
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type Server struct {
	cfg   Config
	store *Store
	queue *Queue
	metr  *controllerMetrics
}

func NewServer(cfg Config, store *Store) *Server {
	return &Server{cfg: cfg, store: store, queue: NewQueue(), metr: &controllerMetrics{}}
}

func (s *Server) Router() *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	r.GET("/metrics", func(c *gin.Context) {
		c.Header("Content-Type", "text/plain; charset=utf-8")
		c.String(http.StatusOK, s.metr.render(s.queue.Len()))
	})

	api := r.Group("/api")
	api.GET("/auth/me", s.handleAuthMe)
	api.POST("/auth/login", s.handleAuthLogin)
	api.POST("/auth/logout", s.handleAuthLogout)
	api.POST("/auth/register", s.handleAuthRegister)
	api.POST("/auth/forgot-password", s.handleAuthForgotPassword)
	api.POST("/auth/reset-password", s.handleAuthResetPassword)
	api.POST("/auth/change-password", s.authSession(), s.handleAuthChangePassword)

	api.POST("/metrics", s.authAgent(), s.handleMetrics)

	api.GET("/users/:username/balance", s.handleBalance)
	api.GET("/users/:username/usage", s.handleUserUsage)
	api.POST("/users/:username/recharge", s.authAdmin(), s.handleRecharge)

	user := api.Group("/user")
	user.Use(s.authSession())
	user.GET("/me", s.handleUserMe)
	user.GET("/me/balance", s.handleUserMyBalance)
	user.GET("/me/usage", s.handleUserMyUsage)
	user.GET("/accounts", s.handleUserAccountsList)
	user.POST("/accounts", s.handleUserAccountsUpsert)
	user.PUT("/accounts", s.handleUserAccountsUpdate)
	user.DELETE("/accounts", s.handleUserAccountsDelete)

	// 用户注册/绑定与 SSH 登录校验
	api.GET("/registry/resolve", s.handleRegistryResolve)
	api.GET("/registry/nodes/:node_id/users.txt", s.handleRegistryNodeUsersTxt)

	// 用户自助登记/开号申请（管理员审核）
	api.GET("/requests", s.handleUserRequestsList)
	api.POST("/requests/bind", s.handleUserBindRequestsCreate)
	api.POST("/requests/open", s.handleUserOpenRequestCreate)

	// 排队接口（可选）：当前实现为“纯排队/不分配”的可运行版本，便于后续接入真实资源分配策略
	api.POST("/gpu/request", s.handleGPURequest)

	admin := api.Group("/admin")
	admin.Use(s.authAdmin())
	admin.POST("/bootstrap", s.handleAdminBootstrap)
	admin.GET("/users", s.handleAdminUsers)
	admin.GET("/prices", s.handleAdminPrices)
	admin.POST("/prices", s.handleAdminSetPrice)
	admin.GET("/gpu/queue", s.handleAdminGPUQueue)
	admin.GET("/requests", s.handleAdminRequestsList)
	admin.POST("/requests/:id/approve", s.handleAdminRequestApprove)
	admin.POST("/requests/:id/reject", s.handleAdminRequestReject)
	admin.GET("/usage", s.handleAdminUsage)
	admin.GET("/nodes", s.handleAdminNodes)
	admin.GET("/usage/export.csv", s.handleAdminUsageExportCSV)
	admin.GET("/mail/settings", s.handleAdminMailSettingsGet)
	admin.POST("/mail/settings", s.handleAdminMailSettingsSet)
	admin.POST("/mail/test", s.handleAdminMailTest)
	admin.GET("/accounts", s.handleAdminAccountsList)
	admin.POST("/accounts", s.handleAdminAccountsUpsert)
	admin.PUT("/accounts", s.handleAdminAccountsUpdate)
	admin.DELETE("/accounts", s.handleAdminAccountsDelete)
	admin.GET("/whitelist", s.handleAdminWhitelistList)
	admin.POST("/whitelist", s.handleAdminWhitelistUpsert)
	admin.DELETE("/whitelist", s.handleAdminWhitelistDelete)
	admin.GET("/stats/users", s.handleAdminStatsUsers)
	admin.GET("/stats/monthly", s.handleAdminStatsMonthly)
	admin.GET("/stats/recharges", s.handleAdminStatsRecharges)

	s.maybeServeWeb(r)
	return r
}

func (s *Server) authSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.cfg.SessionHours == 0 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		secret := strings.TrimSpace(s.cfg.AuthSecret)
		cookie, err := c.Cookie(sessionCookieName)
		if err != nil || strings.TrimSpace(cookie) == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		p, err := verifySession(secret, cookie, time.Now())
		if err != nil || strings.TrimSpace(p.Username) == "" || strings.TrimSpace(p.Role) == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Set("auth_user", p.Username)
		c.Set("auth_role", p.Role)
		c.Set("csrf", p.Nonce)

		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead && c.Request.Method != http.MethodOptions {
			want := p.Nonce
			got := strings.TrimSpace(c.GetHeader("X-CSRF-Token"))
			if want == "" || got == "" || want != got {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "csrf_required"})
				return
			}
		}
		c.Next()
	}
}

func (s *Server) authAgent() gin.HandlerFunc {
	return func(c *gin.Context) {
		tok := strings.TrimSpace(c.GetHeader("X-Agent-Token"))
		if tok == "" || tok != s.cfg.AgentToken {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	}
}

func (s *Server) authAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1) 优先支持脚本类 Bearer admin_token
		auth := strings.TrimSpace(c.GetHeader("Authorization"))
		const prefix = "Bearer "
		if strings.HasPrefix(auth, prefix) && strings.TrimSpace(strings.TrimPrefix(auth, prefix)) == s.cfg.AdminToken {
			c.Set("auth_method", "token")
			c.Next()
			return
		}

		// 2) Web 登录会话（cookie）
		if s.cfg.SessionHours == 0 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		secret := strings.TrimSpace(s.cfg.AuthSecret)
		cookie, err := c.Cookie(sessionCookieName)
		if err != nil || strings.TrimSpace(cookie) == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		p, err := verifySession(secret, cookie, time.Now())
		if err != nil || p.Role != "admin" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Set("auth_method", "session")
		c.Set("auth_user", p.Username)
		c.Set("auth_role", p.Role)
		c.Set("csrf", p.Nonce)

		// CSRF：仅对“有副作用”的请求要求 header（GET 不需要）
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead && c.Request.Method != http.MethodOptions {
			want := p.Nonce
			got := strings.TrimSpace(c.GetHeader("X-CSRF-Token"))
			if want == "" || got == "" || want != got {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "csrf_required"})
				return
			}
		}
		c.Next()
	}
}

func (s *Server) handleBalance(c *gin.Context) {
	username := strings.TrimSpace(c.Param("username"))
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username 不能为空"})
		return
	}

	ctx := c.Request.Context()
	var u User
	if err := s.store.WithTx(ctx, func(tx *sql.Tx) error {
		var err error
		u, err = s.store.EnsureUserTx(ctx, tx, username, s.cfg.DefaultBalance)
		return err
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"username": u.Username,
		"balance":  u.Balance,
		"status":   u.Status,
	})
}

func (s *Server) handleUserUsage(c *gin.Context) {
	username := strings.TrimSpace(c.Param("username"))
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username 不能为空"})
		return
	}
	limit := 200
	if v := strings.TrimSpace(c.Query("limit")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}

	records, err := s.store.ListUsageByUser(c.Request.Context(), username, limit)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"records": records})
}

type rechargeReq struct {
	Amount float64 `json:"amount"`
	Method string  `json:"method"`
}

func (s *Server) handleRecharge(c *gin.Context) {
	username := strings.TrimSpace(c.Param("username"))
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username 不能为空"})
		return
	}

	var req rechargeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	now := time.Now()
	var res BalanceUpdateResult
	if err := s.store.WithTx(ctx, func(tx *sql.Tx) error {
		var err error
		res, err = s.store.RechargeTx(ctx, tx, username, req.Amount, req.Method, now, s.cfg)
		return err
	}); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"username": res.User.Username,
		"balance":  res.User.Balance,
		"status":   res.User.Status,
	})
}

func (s *Server) handleAdminUsers(c *gin.Context) {
	limit := 1000
	users, err := s.store.ListUsers(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"users": users})
}

func (s *Server) handleUserMe(c *gin.Context) {
	username := strings.TrimSpace(fmt.Sprintf("%v", c.MustGet("auth_user")))
	role := strings.TrimSpace(fmt.Sprintf("%v", c.MustGet("auth_role")))
	if role == "admin" {
		c.JSON(http.StatusOK, gin.H{
			"username": username,
			"role":     role,
		})
		return
	}
	acc, err := s.store.GetUserAccountByUsername(c.Request.Context(), username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, acc)
}

func (s *Server) handleUserMyBalance(c *gin.Context) {
	username := strings.TrimSpace(fmt.Sprintf("%v", c.MustGet("auth_user")))
	u, err := s.store.GetUser(c.Request.Context(), username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusOK, gin.H{"username": username, "balance": s.cfg.DefaultBalance, "status": "normal"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"username": u.Username, "balance": u.Balance, "status": u.Status})
}

func (s *Server) handleUserMyUsage(c *gin.Context) {
	username := strings.TrimSpace(fmt.Sprintf("%v", c.MustGet("auth_user")))
	limit := 200
	if v := strings.TrimSpace(c.Query("limit")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	records, err := s.store.ListUsageByUser(c.Request.Context(), username, limit)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"records": records})
}

type userAccountUpsertReq struct {
	NodeID        string `json:"node_id"`
	LocalUsername string `json:"local_username"`
}

type userAccountUpdateReq struct {
	OldNodeID        string `json:"old_node_id"`
	OldLocalUsername string `json:"old_local_username"`
	NewNodeID        string `json:"new_node_id"`
	NewLocalUsername string `json:"new_local_username"`
}

func (s *Server) handleUserAccountsList(c *gin.Context) {
	billing := strings.TrimSpace(fmt.Sprintf("%v", c.MustGet("auth_user")))
	rows, err := s.store.ListUserNodeAccountsByBilling(c.Request.Context(), billing, 5000)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"accounts": rows})
}

func (s *Server) handleUserAccountsUpsert(c *gin.Context) {
	billing := strings.TrimSpace(fmt.Sprintf("%v", c.MustGet("auth_user")))
	var req userAccountUpsertReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := s.store.UpsertUserNodeAccount(c.Request.Context(), req.NodeID, req.LocalUsername, billing); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) handleUserAccountsUpdate(c *gin.Context) {
	billing := strings.TrimSpace(fmt.Sprintf("%v", c.MustGet("auth_user")))
	var req userAccountUpdateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := s.store.UpdateUserNodeAccount(c.Request.Context(),
		req.OldNodeID, req.OldLocalUsername, billing,
		req.NewNodeID, req.NewLocalUsername, billing); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "记录不存在"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) handleUserAccountsDelete(c *gin.Context) {
	billing := strings.TrimSpace(fmt.Sprintf("%v", c.MustGet("auth_user")))
	nodeID := strings.TrimSpace(c.Query("node_id"))
	localUsername := strings.TrimSpace(c.Query("local_username"))
	if err := s.store.DeleteUserNodeAccount(c.Request.Context(), nodeID, localUsername, billing); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "记录不存在"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type adminAccountUpsertReq struct {
	BillingUsername string `json:"billing_username"`
	NodeID          string `json:"node_id"`
	LocalUsername   string `json:"local_username"`
}

type adminAccountUpdateReq struct {
	OldBillingUsername string `json:"old_billing_username"`
	OldNodeID          string `json:"old_node_id"`
	OldLocalUsername   string `json:"old_local_username"`
	NewBillingUsername string `json:"new_billing_username"`
	NewNodeID          string `json:"new_node_id"`
	NewLocalUsername   string `json:"new_local_username"`
}

func (s *Server) handleAdminAccountsList(c *gin.Context) {
	billing := strings.TrimSpace(c.Query("billing_username"))
	if billing == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "billing_username 不能为空"})
		return
	}
	rows, err := s.store.ListUserNodeAccountsByBilling(c.Request.Context(), billing, 5000)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"accounts": rows})
}

func (s *Server) handleAdminAccountsUpsert(c *gin.Context) {
	var req adminAccountUpsertReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := s.store.UpsertUserNodeAccount(c.Request.Context(), req.NodeID, req.LocalUsername, req.BillingUsername); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) handleAdminAccountsUpdate(c *gin.Context) {
	var req adminAccountUpdateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := s.store.UpdateUserNodeAccount(c.Request.Context(),
		req.OldNodeID, req.OldLocalUsername, req.OldBillingUsername,
		req.NewNodeID, req.NewLocalUsername, req.NewBillingUsername); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "记录不存在"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) handleAdminAccountsDelete(c *gin.Context) {
	billing := strings.TrimSpace(c.Query("billing_username"))
	nodeID := strings.TrimSpace(c.Query("node_id"))
	localUsername := strings.TrimSpace(c.Query("local_username"))
	if err := s.store.DeleteUserNodeAccount(c.Request.Context(), nodeID, localUsername, billing); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "记录不存在"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type whitelistUpsertReq struct {
	NodeID    string   `json:"node_id"`
	Usernames []string `json:"usernames"`
}

func (s *Server) handleAdminWhitelistList(c *gin.Context) {
	nodeID := strings.TrimSpace(c.Query("node_id"))
	rows, err := s.store.ListWhitelist(c.Request.Context(), nodeID, 5000)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"entries": rows})
}

func (s *Server) handleAdminWhitelistUpsert(c *gin.Context) {
	var req whitelistUpsertReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	createdBy := "admin"
	if v, ok := c.Get("auth_user"); ok {
		if s, ok2 := v.(string); ok2 && strings.TrimSpace(s) != "" {
			createdBy = strings.TrimSpace(s)
		}
	}
	if err := s.store.UpsertWhitelist(c.Request.Context(), req.NodeID, req.Usernames, createdBy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) handleAdminWhitelistDelete(c *gin.Context) {
	nodeID := strings.TrimSpace(c.Query("node_id"))
	localUsername := strings.TrimSpace(c.Query("local_username"))
	if err := s.store.DeleteWhitelist(c.Request.Context(), nodeID, localUsername); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "记录不存在"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) handleAdminPrices(c *gin.Context) {
	prices, err := s.store.ListPrices(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"prices": prices})
}

type setPriceReq struct {
	GPUModel       string  `json:"gpu_model"`
	PricePerMinute float64 `json:"price_per_minute"`
}

func (s *Server) handleAdminSetPrice(c *gin.Context) {
	var req setPriceReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := s.store.UpsertPrice(c.Request.Context(), req.GPUModel, req.PricePerMinute); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) handleAdminUsage(c *gin.Context) {
	username := strings.TrimSpace(c.Query("username"))
	limit := 200
	if v := strings.TrimSpace(c.Query("limit")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	records, err := s.store.ListUsageAdmin(c.Request.Context(), username, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"records": records})
}

func (s *Server) handleAdminStatsUsers(c *gin.Context) {
	from, to, err := parseStatsRange(c, 365)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	limit := parseLimit(c.Query("limit"), 1000, 10000)
	rows, err := s.store.ListUsageSummaryByUser(c.Request.Context(), from, to, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"from": from.Format(time.RFC3339), "to": to.Format(time.RFC3339), "rows": rows})
}

func (s *Server) handleAdminStatsMonthly(c *gin.Context) {
	from, to, err := parseStatsRange(c, 365)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	limit := parseLimit(c.Query("limit"), 20000, 200000)
	rows, err := s.store.ListUsageMonthlyByUser(c.Request.Context(), from, to, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"from": from.Format(time.RFC3339), "to": to.Format(time.RFC3339), "rows": rows})
}

func (s *Server) handleAdminStatsRecharges(c *gin.Context) {
	from, to, err := parseStatsRange(c, 365)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	limit := parseLimit(c.Query("limit"), 1000, 10000)
	rows, err := s.store.ListRechargeSummary(c.Request.Context(), from, to, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"from": from.Format(time.RFC3339), "to": to.Format(time.RFC3339), "rows": rows})
}

func (s *Server) handleAdminNodes(c *gin.Context) {
	limit := 200
	if v := strings.TrimSpace(c.Query("limit")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	nodes, err := s.store.ListNodes(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"nodes": nodes})
}

func (s *Server) handleAdminUsageExportCSV(c *gin.Context) {
	username := strings.TrimSpace(c.Query("username"))
	fromStr := strings.TrimSpace(c.Query("from"))
	toStr := strings.TrimSpace(c.Query("to"))
	limit := 20000
	if v := strings.TrimSpace(c.Query("limit")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	if limit <= 0 || limit > 200000 {
		limit = 20000
	}

	var from time.Time
	var to time.Time
	var err error
	if fromStr != "" {
		from, err = parseTimeFlexible(fromStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "from 时间格式不合法，建议 RFC3339 或 YYYY-MM-DD"})
			return
		}
	}
	if toStr != "" {
		to, err = parseTimeFlexible(toStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "to 时间格式不合法，建议 RFC3339 或 YYYY-MM-DD"})
			return
		}
	}

	ctx := c.Request.Context()
	rows, err := s.store.queryUsageRows(ctx, username, fromStr != "", from, toStr != "", to, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	filename := "usage_export.csv"
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))

	w := csv.NewWriter(c.Writer)
	_ = w.Write([]string{"timestamp", "node_id", "username", "cpu_percent", "memory_mb", "cost", "gpu_usage_json"})

	for rows.Next() {
		var nodeID, user string
		var ts time.Time
		var cpuPercent, memoryMB, cost float64
		var gpuUsage string
		if err := rows.Scan(&nodeID, &user, &ts, &cpuPercent, &memoryMB, &gpuUsage, &cost); err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		_ = w.Write([]string{
			ts.Format(time.RFC3339),
			nodeID,
			user,
			fmt.Sprintf("%.4f", cpuPercent),
			fmt.Sprintf("%.4f", memoryMB),
			fmt.Sprintf("%.4f", cost),
			gpuUsage,
		})
	}
	w.Flush()
}

type adminMailTestReq struct {
	Username string `json:"username"`
}

func (s *Server) handleAdminMailTest(c *gin.Context) {
	var req adminMailTestReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	username := strings.TrimSpace(req.Username)
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username 不能为空"})
		return
	}
	email, err := s.store.GetUserEmailByUsername(c.Request.Context(), username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "该用户没有注册邮箱"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	settings, err := s.store.GetMailSettings(c.Request.Context(), s.cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	subject := "HIT-AIOT-OPS 邮件配置测试"
	body := fmt.Sprintf("你好 %s，\n\n这是一封测试邮件，表示管理员已成功配置 SMTP。\n时间：%s\n\nHIT-AIOT-OPS团队", username, time.Now().Format(time.RFC3339))
	if err := sendResetPasswordMail(settings, email, subject, body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "发送失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "email": email})
}

func (s *Server) handleMetrics(c *gin.Context) {
	var data MetricsData
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	data.NodeID = strings.TrimSpace(data.NodeID)
	if data.NodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "node_id 不能为空"})
		return
	}
	data.ReportID = strings.TrimSpace(data.ReportID)
	if data.ReportID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "report_id 不能为空（用于幂等防重）"})
		return
	}

	reportTS, err := time.Parse(time.RFC3339, strings.TrimSpace(data.Timestamp))
	if err != nil {
		// 允许 Agent 不传或传错时间，控制器兜底为当前时间
		reportTS = time.Now()
	}

	// 先做轻量清洗：去掉无效记录，避免污染账单
	cleaned := make([]UserProcess, 0, len(data.Users))
	for _, p := range data.Users {
		p.Username = strings.TrimSpace(p.Username)
		if p.Username == "" || p.PID <= 0 {
			continue
		}
		// CPU-only 进程也允许进入：后续按 CPUPercent 决定是否计费
		cleaned = append(cleaned, p)
	}
	data.Users = cleaned

	ctx := c.Request.Context()
	actions, err := s.processMetrics(ctx, data, reportTS)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, ControllerResponse{Actions: actions})
}

type gpuRequestReq struct {
	Username string `json:"username"`
	GPUType  string `json:"gpu_type"`
	Count    int    `json:"count"`
}

func (s *Server) handleGPURequest(c *gin.Context) {
	var req gpuRequestReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	req.GPUType = strings.TrimSpace(req.GPUType)
	if req.Username == "" || req.GPUType == "" || req.Count <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username/gpu_type/count 参数不合法"})
		return
	}

	item := QueueItem{
		Username:  req.Username,
		GPUType:   req.GPUType,
		Count:     req.Count,
		Timestamp: time.Now(),
	}

	pos := s.queue.Enqueue(item)
	estimated := estimateWaitMinutes(pos)

	c.JSON(http.StatusOK, gin.H{
		"status":            "queued",
		"position":          pos,
		"estimated_minutes": estimated,
		"message":           "当前无可用 GPU，已加入排队",
	})
}

func (s *Server) handleAdminGPUQueue(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"queue": s.queue.Snapshot()})
}

func (s *Server) processMetrics(ctx context.Context, data MetricsData, reportTS time.Time) ([]Action, error) {
	now := time.Now()
	grace := time.Duration(s.cfg.KillGracePeriodSeconds) * time.Second
	intervalSeconds := s.cfg.SampleIntervalSeconds
	if data.IntervalSeconds > 0 && data.IntervalSeconds <= 600 {
		intervalSeconds = data.IntervalSeconds
	}
	intervalMinutes := float64(intervalSeconds) / 60.0
	if intervalMinutes <= 0 {
		intervalMinutes = 1
	}

	type localAgg struct {
		pids []int32
	}
	type billingAgg struct {
		cost   float64
		locals map[string]*localAgg // local_username -> pids
	}
	billingAggs := make(map[string]*billingAgg)
	usageRecords := 0
	gpuProcCount := 0
	cpuProcCount := 0
	costTotal := 0.0

	var actions []Action
	duplicate := false

	err := s.store.WithTx(ctx, func(tx *sql.Tx) error {
		inserted, err := s.store.TryInsertReportTx(ctx, tx, data.ReportID, data.NodeID, reportTS, intervalSeconds)
		if err != nil {
			return err
		}
		if !inserted {
			duplicate = true
			return nil
		}

		priceRows, err := s.store.LoadPricesTx(ctx, tx)
		if err != nil {
			return err
		}
		priceIndex := NewPriceIndex(priceRows)
		cpuPricePerCoreMinute := s.cfg.CPUPricePerCoreMinute
		if v, ok := priceIndex.MatchPrice("CPU_CORE"); ok {
			cpuPricePerCoreMinute = v
		}

		// 同一台节点的映射在一次上报内复用，避免对每个进程重复查库
		resolveCache := make(map[string]string) // local_username -> billing_username（未绑定时为自身）

		for _, proc := range data.Users {
			localUsername := strings.TrimSpace(proc.Username)
			if localUsername == "" {
				continue
			}

			billingUsername, ok := resolveCache[localUsername]
			if !ok {
				mapped, found, err := s.store.ResolveBillingUsernameTx(ctx, tx, data.NodeID, localUsername)
				if err != nil {
					return err
				}
				if found && strings.TrimSpace(mapped) != "" {
					billingUsername = mapped
				} else {
					billingUsername = localUsername
				}
				resolveCache[localUsername] = billingUsername
			}

			gpuCost := 0.0
			if len(proc.GPUUsage) > 0 {
				gpuCost = CalculateProcessCost(proc, priceIndex, s.cfg.DefaultPricePerMinute)
			}
			proc.Command = strings.TrimSpace(proc.Command)
			if len(proc.Command) > 256 {
				proc.Command = proc.Command[:256]
			}
			cpuCost := (proc.CPUPercent / 100.0) * cpuPricePerCoreMinute * intervalMinutes
			cost := round4(gpuCost + cpuCost)

			// 如果既没有 GPU，也几乎不占 CPU，就不计费也不落库（避免噪声与膨胀）
			if len(proc.GPUUsage) == 0 && proc.CPUPercent < 1.0 {
				continue
			}
			// usage_records 归集到计费账号，便于按“中心账号”对账/查询
			procForStore := proc
			procForStore.Username = billingUsername
			if err := s.store.InsertUsageRecordTx(ctx, tx, data.NodeID, reportTS, procForStore, cost); err != nil {
				return err
			}
			usageRecords++
			costTotal += cost
			if len(proc.GPUUsage) > 0 {
				gpuProcCount++
			} else {
				cpuProcCount++
			}

			b := billingAggs[billingUsername]
			if b == nil {
				b = &billingAgg{locals: make(map[string]*localAgg)}
				billingAggs[billingUsername] = b
			}
			b.cost += cost
			la := b.locals[localUsername]
			if la == nil {
				la = &localAgg{}
				b.locals[localUsername] = la
			}
			la.pids = append(la.pids, proc.PID)
		}

		for billingUsername, b := range billingAggs {
			res, err := s.store.DeductBalanceTx(ctx, tx, billingUsername, b.cost, now, s.cfg)
			if err != nil {
				return err
			}

			// 注意：扣费与余额状态以“计费账号”为准；但下发动作必须针对“节点本地账号”，否则 Agent 无法生效。
			for localUsername, la := range b.locals {
				uLocal := res.User
				uLocal.Username = localUsername
				actions = append(actions, DecideActions(now, res.PrevStatus, uLocal, s.cfg.WarningThreshold, s.cfg.LimitedThreshold, grace, la.pids)...)

				if s.cfg.EnableCPUControl {
					if res.User.Status == "limited" {
						actions = append(actions, Action{
							Type:            "set_cpu_quota",
							Username:        localUsername,
							CPUQuotaPercent: s.cfg.CPULimitPercentLimited,
							Reason:          "余额不足，限制 CPU 使用",
						})
					} else if res.User.Status == "blocked" {
						actions = append(actions, Action{
							Type:            "set_cpu_quota",
							Username:        localUsername,
							CPUQuotaPercent: s.cfg.CPULimitPercentBlocked,
							Reason:          "已欠费，强限制 CPU 使用",
						})
					} else if res.PrevStatus == "limited" || res.PrevStatus == "blocked" {
						actions = append(actions, Action{
							Type:            "set_cpu_quota",
							Username:        localUsername,
							CPUQuotaPercent: 0,
							Reason:          "余额已恢复，解除 CPU 限制",
						})
					}
				}
			}
		}

		// 更新节点状态（用于运维查看在线/上报情况）
		if err := s.store.UpsertNodeStatusTx(
			ctx,
			tx,
			data.NodeID,
			now,
			data.ReportID,
			reportTS,
			intervalSeconds,
			data.CPUModel,
			data.CPUCount,
			data.GPUModel,
			data.GPUCount,
			data.NetRxBytes,
			data.NetTxBytes,
			gpuProcCount,
			cpuProcCount,
			usageRecords,
			round4(costTotal),
		); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if duplicate {
		s.metr.observeReport(now, true, 0, nil)
		return []Action{}, nil
	}
	s.metr.observeReport(now, false, usageRecords, actions)
	return actions, nil
}

func round4(v float64) float64 {
	// 避免引入更多依赖，使用 billing.go 同样的舍入策略
	return float64(int64(v*10000+0.5)) / 10000
}

func parseTimeFlexible(v string) (time.Time, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return time.Time{}, fmt.Errorf("empty")
	}
	// RFC3339
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t, nil
	}
	// YYYY-MM-DD（按 UTC 00:00:00）
	if t, err := time.Parse("2006-01-02", v); err == nil {
		return t, nil
	}
	// 兼容常见：YYYY-MM-DD HH:MM:SS（按本地时间）
	if t, err := time.ParseInLocation("2006-01-02 15:04:05", v, time.Local); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("invalid time: %s", v)
}

func parseStatsRange(c *gin.Context, defaultDays int) (time.Time, time.Time, error) {
	now := time.Now()
	from := now.AddDate(0, 0, -defaultDays)
	to := now
	if x := strings.TrimSpace(c.Query("from")); x != "" {
		t, err := parseTimeFlexible(x)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("from 时间格式不合法，建议 RFC3339 或 YYYY-MM-DD")
		}
		from = t
	}
	if x := strings.TrimSpace(c.Query("to")); x != "" {
		t, err := parseTimeFlexible(x)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("to 时间格式不合法，建议 RFC3339 或 YYYY-MM-DD")
		}
		if len(x) == len("2006-01-02") {
			t = t.Add(24*time.Hour - time.Nanosecond)
		}
		to = t
	}
	if to.Before(from) {
		return time.Time{}, time.Time{}, fmt.Errorf("to 不能早于 from")
	}
	return from, to, nil
}

func parseLimit(v string, def int, max int) int {
	n := def
	if x := strings.TrimSpace(v); x != "" {
		if y, err := strconv.Atoi(x); err == nil {
			n = y
		}
	}
	if n <= 0 {
		n = def
	}
	if n > max {
		n = max
	}
	return n
}

func (s *Server) maybeServeWeb(r *gin.Engine) {
	webDir := strings.TrimSpace(s.cfg.WebDir)
	if webDir == "" {
		candidates := []string{
			filepath.FromSlash("../web/dist"),
			filepath.FromSlash("web/dist"),
		}
		for _, p := range candidates {
			if dirExists(p) {
				webDir = p
				break
			}
		}
	}
	if webDir == "" || !dirExists(webDir) {
		return
	}
	if _, err := os.Stat(filepath.Join(webDir, "index.html")); err != nil {
		return
	}

	// 静态资源直出（index 交给 NoRoute）
	if dirExists(filepath.Join(webDir, "static")) {
		r.Static("/static", filepath.Join(webDir, "static"))
	}

	// 只在 /api 不匹配时回退到 index.html，避免覆盖 API。
	r.NoRoute(func(c *gin.Context) {
		if c.Request.Method != http.MethodGet {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
			return
		}
		if strings.HasPrefix(c.Request.URL.Path, "/api/") || c.Request.URL.Path == "/api" {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
			return
		}
		c.File(filepath.Join(webDir, "index.html"))
	})
}
