package main

import (
	"context"
	"database/sql"
	"encoding/csv"
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

	api.POST("/metrics", s.authAgent(), s.handleMetrics)

	api.GET("/users/:username/balance", s.handleBalance)
	api.GET("/users/:username/usage", s.handleUserUsage)
	api.POST("/users/:username/recharge", s.authAdmin(), s.handleRecharge)

	// 排队接口（可选）：当前实现为“纯排队/不分配”的可运行版本，便于后续接入真实资源分配策略
	api.POST("/gpu/request", s.handleGPURequest)

	admin := api.Group("/admin")
	admin.Use(s.authAdmin())
	admin.POST("/bootstrap", s.handleAdminBootstrap)
	admin.GET("/users", s.handleAdminUsers)
	admin.GET("/prices", s.handleAdminPrices)
	admin.POST("/prices", s.handleAdminSetPrice)
	admin.GET("/gpu/queue", s.handleAdminGPUQueue)
	admin.GET("/usage", s.handleAdminUsage)
	admin.GET("/nodes", s.handleAdminNodes)
	admin.GET("/usage/export.csv", s.handleAdminUsageExportCSV)

	s.maybeServeWeb(r)
	return r
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

	type agg struct {
		pids []int32
		cost float64
	}
	userAgg := make(map[string]*agg)
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

		for _, proc := range data.Users {
			gpuCost := 0.0
			if len(proc.GPUUsage) > 0 {
				gpuCost = CalculateProcessCost(proc, priceIndex, s.cfg.DefaultPricePerMinute)
			}
			cpuCost := (proc.CPUPercent / 100.0) * cpuPricePerCoreMinute * intervalMinutes
			cost := round4(gpuCost + cpuCost)

			// 如果既没有 GPU，也几乎不占 CPU，就不计费也不落库（避免噪声与膨胀）
			if len(proc.GPUUsage) == 0 && proc.CPUPercent < 1.0 {
				continue
			}
			if err := s.store.InsertUsageRecordTx(ctx, tx, data.NodeID, reportTS, proc, cost); err != nil {
				return err
			}
			usageRecords++
			costTotal += cost
			if len(proc.GPUUsage) > 0 {
				gpuProcCount++
			} else {
				cpuProcCount++
			}

			a := userAgg[proc.Username]
			if a == nil {
				a = &agg{}
				userAgg[proc.Username] = a
			}
			a.cost += cost
			a.pids = append(a.pids, proc.PID)
		}

		for username, a := range userAgg {
			res, err := s.store.DeductBalanceTx(ctx, tx, username, a.cost, now, s.cfg)
			if err != nil {
				return err
			}
			actions = append(actions, DecideActions(now, res.PrevStatus, res.User, s.cfg.WarningThreshold, s.cfg.LimitedThreshold, grace, a.pids)...)
			if s.cfg.EnableCPUControl {
				if res.User.Status == "limited" {
					actions = append(actions, Action{
						Type:            "set_cpu_quota",
						Username:        res.User.Username,
						CPUQuotaPercent: s.cfg.CPULimitPercentLimited,
						Reason:          "余额不足，限制 CPU 使用",
					})
				} else if res.User.Status == "blocked" {
					actions = append(actions, Action{
						Type:            "set_cpu_quota",
						Username:        res.User.Username,
						CPUQuotaPercent: s.cfg.CPULimitPercentBlocked,
						Reason:          "已欠费，强限制 CPU 使用",
					})
				} else if res.PrevStatus == "limited" || res.PrevStatus == "blocked" {
					actions = append(actions, Action{
						Type:            "set_cpu_quota",
						Username:        res.User.Username,
						CPUQuotaPercent: 0,
						Reason:          "余额已恢复，解除 CPU 限制",
					})
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
