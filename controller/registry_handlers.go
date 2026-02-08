package main

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func (s *Server) handleRegistryResolve(c *gin.Context) {
	nodeID := strings.TrimSpace(c.Query("node_id"))
	localUsername := strings.TrimSpace(c.Query("local_username"))
	if nodeID == "" || localUsername == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "node_id/local_username 不能为空"})
		return
	}

	ctx := c.Request.Context()
	var billing string
	found := false
	if err := s.store.WithTx(ctx, func(tx *sql.Tx) error {
		var err error
		billing, found, err = s.store.ResolveBillingUsernameTx(ctx, tx, nodeID, localUsername)
		return err
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if !found {
		whitelisted, err := s.store.IsWhitelisted(ctx, nodeID, localUsername)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if !whitelisted {
			c.JSON(http.StatusOK, gin.H{"registered": false})
			return
		}
		c.JSON(http.StatusOK, gin.H{"registered": true, "billing_username": localUsername, "whitelisted": true})
		return
	}
	c.JSON(http.StatusOK, gin.H{"registered": true, "billing_username": billing})
}

// handleRegistryNodeUsersTxt 返回该节点已登记的本地用户名列表（每行一个），用于 PAM/SSH 校验缓存同步。
func (s *Server) handleRegistryNodeUsersTxt(c *gin.Context) {
	nodeID := strings.TrimSpace(c.Param("node_id"))
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "node_id 不能为空"})
		return
	}
	users, err := s.store.ListAllowedLocalUsersByNode(c.Request.Context(), nodeID, 200000)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Header("Content-Type", "text/plain; charset=utf-8")
	if len(users) == 0 {
		c.String(http.StatusOK, "")
		return
	}
	c.String(http.StatusOK, strings.Join(users, "\n")+"\n")
}

type bindRequestsCreateReq struct {
	BillingUsername string `json:"billing_username"`
	Items           []struct {
		NodeID        string `json:"node_id"`
		LocalUsername string `json:"local_username"`
	} `json:"items"`
	Message string `json:"message"`
}

func (s *Server) handleUserBindRequestsCreate(c *gin.Context) {
	var req bindRequestsCreateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.BillingUsername = strings.TrimSpace(req.BillingUsername)
	req.Message = strings.TrimSpace(req.Message)
	if req.BillingUsername == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "billing_username 不能为空"})
		return
	}
	if len(req.Items) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "items 不能为空"})
		return
	}
	if len(req.Items) > 200 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "items 过多（最大 200）"})
		return
	}

	ctx := c.Request.Context()
	var ids []int
	if err := s.store.WithTx(ctx, func(tx *sql.Tx) error {
		for _, it := range req.Items {
			nodeID := strings.TrimSpace(it.NodeID)
			localUsername := strings.TrimSpace(it.LocalUsername)
			if nodeID == "" || localUsername == "" {
				return strconv.ErrSyntax
			}
			id, err := s.store.CreateUserRequestTx(ctx, tx, "bind", req.BillingUsername, nodeID, localUsername, req.Message)
			if err != nil {
				return err
			}
			ids = append(ids, id)
		}
		return nil
	}); err != nil {
		if err == strconv.ErrSyntax {
			c.JSON(http.StatusBadRequest, gin.H{"error": "node_id/local_username 不能为空"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "request_ids": ids})
}

type openRequestCreateReq struct {
	BillingUsername string `json:"billing_username"`
	NodeID          string `json:"node_id"`
	LocalUsername   string `json:"local_username"`
	Message         string `json:"message"`
}

func (s *Server) handleUserOpenRequestCreate(c *gin.Context) {
	var req openRequestCreateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.BillingUsername = strings.TrimSpace(req.BillingUsername)
	req.NodeID = strings.TrimSpace(req.NodeID)
	req.LocalUsername = strings.TrimSpace(req.LocalUsername)
	req.Message = strings.TrimSpace(req.Message)

	if req.BillingUsername == "" || req.NodeID == "" || req.LocalUsername == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "billing_username/node_id/local_username 不能为空"})
		return
	}

	ctx := c.Request.Context()
	var id int
	if err := s.store.WithTx(ctx, func(tx *sql.Tx) error {
		var err error
		id, err = s.store.CreateUserRequestTx(ctx, tx, "open", req.BillingUsername, req.NodeID, req.LocalUsername, req.Message)
		return err
	}); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "request_id": id})
}

func (s *Server) handleUserRequestsList(c *gin.Context) {
	billing := strings.TrimSpace(c.Query("billing_username"))
	limit := 200
	if v := strings.TrimSpace(c.Query("limit")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	records, err := s.store.ListUserRequestsByBilling(c.Request.Context(), billing, limit)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"requests": records})
}

func (s *Server) handleAdminRequestsList(c *gin.Context) {
	status := strings.TrimSpace(c.Query("status"))
	limit := 200
	if v := strings.TrimSpace(c.Query("limit")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	records, err := s.store.ListUserRequestsAdmin(c.Request.Context(), status, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"requests": records})
}

func (s *Server) handleAdminRequestApprove(c *gin.Context) {
	s.handleAdminRequestReview(c, "approved")
}

func (s *Server) handleAdminRequestReject(c *gin.Context) {
	s.handleAdminRequestReview(c, "rejected")
}

func (s *Server) handleAdminRequestReview(c *gin.Context, newStatus string) {
	idStr := strings.TrimSpace(c.Param("id"))
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id 不合法"})
		return
	}
	reviewedBy := "admin"
	if v, ok := c.Get("auth_user"); ok {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			reviewedBy = strings.TrimSpace(s)
		}
	} else if v, ok := c.Get("auth_method"); ok {
		if m, ok := v.(string); ok && m == "token" {
			reviewedBy = "admin_token"
		}
	}

	ctx := c.Request.Context()
	now := time.Now()
	var updated UserRequest
	if err := s.store.WithTx(ctx, func(tx *sql.Tx) error {
		var err error
		updated, err = s.store.ReviewUserRequestTx(ctx, tx, id, newStatus, reviewedBy, now)
		return err
	}); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "request": updated})
}
