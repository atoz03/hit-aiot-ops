package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

const (
	appSettingSMTPHost  = "smtp_host"
	appSettingSMTPPort  = "smtp_port"
	appSettingSMTPUser  = "smtp_user"
	appSettingSMTPPass  = "smtp_pass"
	appSettingFromEmail = "from_email"
	appSettingFromName  = "from_name"
)

type Store struct {
	db *sql.DB
}

func NewStore(cfg Config) (*Store, error) {
	db, err := sql.Open("postgres", cfg.DatabaseDSN)
	if err != nil {
		return nil, err
	}
	// 连接池参数可按实际压测调优
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) WithTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ApplyMigrations(ctx context.Context, migrationDir string) error {
	dir, err := resolveMigrationDir(migrationDir)
	if err != nil {
		return err
	}

	if err := ensureMigrationsTable(ctx, s.db); err != nil {
		return err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".sql") {
			files = append(files, filepath.Join(dir, name))
		}
	}
	sort.Strings(files)

	for _, f := range files {
		filename := filepath.Base(f)
		applied, err := isMigrationApplied(ctx, s.db, filename)
		if err != nil {
			return err
		}
		if applied {
			continue
		}

		sqlBytes, err := os.ReadFile(filepath.Clean(f))
		if err != nil {
			return err
		}
		sqlText := strings.TrimSpace(string(sqlBytes))
		if sqlText == "" {
			return fmt.Errorf("迁移文件为空：%s", filename)
		}

		if _, err := s.db.ExecContext(ctx, sqlText); err != nil {
			return fmt.Errorf("执行迁移失败 %s: %w", filename, err)
		}
		if _, err := s.db.ExecContext(ctx, `INSERT INTO schema_migrations(filename) VALUES ($1)`, filename); err != nil {
			return fmt.Errorf("记录迁移失败 %s: %w", filename, err)
		}
	}
	return nil
}

func ensureMigrationsTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
	filename TEXT PRIMARY KEY,
	applied_at TIMESTAMP NOT NULL DEFAULT NOW()
);`)
	return err
}

func isMigrationApplied(ctx context.Context, db *sql.DB, filename string) (bool, error) {
	var exists bool
	err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE filename=$1)`, filename).Scan(&exists)
	return exists, err
}

func resolveMigrationDir(cfgValue string) (string, error) {
	if cfgValue != "" {
		if dirExists(cfgValue) {
			return cfgValue, nil
		}
		return "", fmt.Errorf("migration_dir 不存在：%s", cfgValue)
	}

	candidates := []string{
		filepath.FromSlash("../database/migrations"),
		filepath.FromSlash("database/migrations"),
	}
	for _, c := range candidates {
		if dirExists(c) {
			return c, nil
		}
	}
	return "", errors.New("未找到迁移目录：请配置 migration_dir 或在 ../database/migrations 下放置迁移文件")
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func (s *Store) EnsureUserTx(ctx context.Context, tx *sql.Tx, username string, defaultBalance float64) (User, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return User{}, errors.New("username 不能为空")
	}

	_, err := tx.ExecContext(ctx, `
INSERT INTO users(username, balance, status)
VALUES($1, $2, 'normal')
ON CONFLICT (username) DO NOTHING`, username, defaultBalance)
	if err != nil {
		return User{}, err
	}

	return s.GetUserTx(ctx, tx, username)
}

// TryInsertReportTx 尝试写入上报记录（用于幂等）。
// 返回 inserted=false 表示该 report_id 已处理过，应跳过扣费与落库。
func (s *Store) TryInsertReportTx(ctx context.Context, tx *sql.Tx, reportID string, nodeID string, ts time.Time, intervalSeconds int) (bool, error) {
	reportID = strings.TrimSpace(reportID)
	if reportID == "" {
		return false, errors.New("report_id 不能为空")
	}
	if intervalSeconds <= 0 {
		intervalSeconds = 60
	}
	res, err := tx.ExecContext(ctx, `
INSERT INTO metric_reports(report_id, node_id, timestamp, interval_seconds)
VALUES($1,$2,$3,$4)
ON CONFLICT (report_id) DO NOTHING`, reportID, nodeID, ts, intervalSeconds)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected == 1, nil
}

func (s *Store) GetUser(ctx context.Context, username string) (User, error) {
	var u User
	err := s.db.QueryRowContext(ctx, `
SELECT username, balance, status, blocked_at
FROM users
WHERE username=$1`, username).Scan(&u.Username, &u.Balance, &u.Status, &u.BlockedAt)
	return u, err
}

func (s *Store) GetUserTx(ctx context.Context, tx *sql.Tx, username string) (User, error) {
	var u User
	err := tx.QueryRowContext(ctx, `
SELECT username, balance, status, blocked_at
FROM users
WHERE username=$1`, username).Scan(&u.Username, &u.Balance, &u.Status, &u.BlockedAt)
	return u, err
}

type BalanceUpdateResult struct {
	PrevStatus string
	User       User
}

func (s *Store) DeductBalanceTx(
	ctx context.Context,
	tx *sql.Tx,
	username string,
	amount float64,
	now time.Time,
	cfg Config,
) (BalanceUpdateResult, error) {
	_, err := s.EnsureUserTx(ctx, tx, username, cfg.DefaultBalance)
	if err != nil {
		return BalanceUpdateResult{}, err
	}

	// 行级锁，避免并发扣费导致余额错乱
	var balance float64
	var prevStatus string
	var blockedAt *time.Time
	if err := tx.QueryRowContext(ctx, `
SELECT balance, status, blocked_at
FROM users
WHERE username=$1
FOR UPDATE`, username).Scan(&balance, &prevStatus, &blockedAt); err != nil {
		return BalanceUpdateResult{}, err
	}

	newBalance := balance
	if !cfg.DryRun {
		newBalance = balance - amount
	}
	newStatus := StatusForBalance(newBalance, cfg.WarningThreshold, cfg.LimitedThreshold)
	newBlockedAt := blockedAt
	if newStatus == "blocked" {
		if newBlockedAt == nil {
			newBlockedAt = &now
		}
	} else {
		newBlockedAt = nil
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE users
SET balance=$2, status=$3, blocked_at=$4
WHERE username=$1`, username, newBalance, newStatus, newBlockedAt); err != nil {
		return BalanceUpdateResult{}, err
	}

	return BalanceUpdateResult{
		PrevStatus: prevStatus,
		User: User{
			Username:  username,
			Balance:   newBalance,
			Status:    newStatus,
			BlockedAt: newBlockedAt,
		},
	}, nil
}

func (s *Store) RechargeTx(ctx context.Context, tx *sql.Tx, username string, amount float64, method string, now time.Time, cfg Config) (BalanceUpdateResult, error) {
	if amount <= 0 {
		return BalanceUpdateResult{}, errors.New("amount 必须为正数")
	}
	if strings.TrimSpace(method) == "" {
		return BalanceUpdateResult{}, errors.New("method 不能为空")
	}

	_, err := s.EnsureUserTx(ctx, tx, username, cfg.DefaultBalance)
	if err != nil {
		return BalanceUpdateResult{}, err
	}

	var balance float64
	var prevStatus string
	var blockedAt *time.Time
	if err := tx.QueryRowContext(ctx, `
SELECT balance, status, blocked_at
FROM users
WHERE username=$1
FOR UPDATE`, username).Scan(&balance, &prevStatus, &blockedAt); err != nil {
		return BalanceUpdateResult{}, err
	}

	newBalance := balance + amount
	newStatus := StatusForBalance(newBalance, cfg.WarningThreshold, cfg.LimitedThreshold)
	var newBlockedAt *time.Time
	if newStatus == "blocked" {
		newBlockedAt = blockedAt
		if newBlockedAt == nil {
			newBlockedAt = &now
		}
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE users
SET balance=$2, status=$3, blocked_at=$4, last_charge_time=NOW()
WHERE username=$1`, username, newBalance, newStatus, newBlockedAt); err != nil {
		return BalanceUpdateResult{}, err
	}

	if _, err := tx.ExecContext(ctx, `
INSERT INTO recharge_records(username, amount, method)
VALUES($1, $2, $3)`, username, amount, method); err != nil {
		return BalanceUpdateResult{}, err
	}

	return BalanceUpdateResult{
		PrevStatus: prevStatus,
		User: User{
			Username:  username,
			Balance:   newBalance,
			Status:    newStatus,
			BlockedAt: newBlockedAt,
		},
	}, nil
}

func (s *Store) InsertUsageRecordTx(ctx context.Context, tx *sql.Tx, nodeID string, ts time.Time, proc UserProcess, cost float64) error {
	gpuUsage := proc.GPUUsage
	if gpuUsage == nil {
		// 保持 JSONB 非空且语义一致：CPU-only 记录也用空数组而非 null
		gpuUsage = []GPUUsage{}
	}
	gpuJSON, err := json.Marshal(gpuUsage)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO usage_records(node_id, username, timestamp, pid, cpu_percent, memory_mb, gpu_count, command, gpu_usage, cost)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		nodeID, proc.Username, ts, proc.PID, proc.CPUPercent, proc.MemoryMB, len(proc.GPUUsage), strings.TrimSpace(proc.Command), string(gpuJSON), cost)
	return err
}

func (s *Store) LoadPricesTx(ctx context.Context, tx *sql.Tx) ([]PriceRow, error) {
	rows, err := tx.QueryContext(ctx, `SELECT gpu_model, price_per_minute FROM resource_prices`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []PriceRow
	for rows.Next() {
		var model string
		var price float64
		if err := rows.Scan(&model, &price); err != nil {
			return nil, err
		}
		out = append(out, PriceRow{Model: model, Price: price})
	}
	return out, rows.Err()
}

func (s *Store) UpsertPrice(ctx context.Context, model string, price float64) error {
	model = strings.TrimSpace(model)
	if model == "" {
		return errors.New("gpu_model 不能为空")
	}
	if price < 0 {
		return errors.New("price_per_minute 不能为负数")
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO resource_prices(gpu_model, price_per_minute)
VALUES($1,$2)
ON CONFLICT (gpu_model) DO UPDATE
SET price_per_minute=EXCLUDED.price_per_minute, updated_at=NOW()`, model, price)
	return err
}

func (s *Store) ListPrices(ctx context.Context) ([]PriceRow, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT gpu_model, price_per_minute FROM resource_prices ORDER BY gpu_model`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []PriceRow
	for rows.Next() {
		var r PriceRow
		if err := rows.Scan(&r.Model, &r.Price); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) ListUsers(ctx context.Context, limit int) ([]User, error) {
	if limit <= 0 || limit > 10000 {
		limit = 1000
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT username, balance, status, blocked_at
FROM users
ORDER BY username
LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.Username, &u.Balance, &u.Status, &u.BlockedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Store) ListUsageByUser(ctx context.Context, username string, limit int) ([]UsageRecord, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, errors.New("username 不能为空")
	}
	if limit <= 0 || limit > 5000 {
		limit = 200
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT node_id, username, timestamp, pid, cpu_percent, memory_mb, gpu_count, command, gpu_usage, cost
FROM usage_records
WHERE username=$1
ORDER BY timestamp DESC
LIMIT $2`, username, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []UsageRecord
	for rows.Next() {
		var r UsageRecord
		if err := rows.Scan(&r.NodeID, &r.Username, &r.Timestamp, &r.PID, &r.CPUPercent, &r.MemoryMB, &r.GPUCount, &r.Command, &r.GPUUsage, &r.Cost); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) ListUsageAdmin(ctx context.Context, username string, limit int) ([]UsageRecord, error) {
	username = strings.TrimSpace(username)
	if limit <= 0 || limit > 5000 {
		limit = 200
	}

	var rows *sql.Rows
	var err error
	if username == "" {
		rows, err = s.db.QueryContext(ctx, `
SELECT node_id, username, timestamp, pid, cpu_percent, memory_mb, gpu_count, command, gpu_usage, cost
FROM usage_records
ORDER BY timestamp DESC
LIMIT $1`, limit)
	} else {
		rows, err = s.db.QueryContext(ctx, `
SELECT node_id, username, timestamp, pid, cpu_percent, memory_mb, gpu_count, command, gpu_usage, cost
FROM usage_records
WHERE username=$1
ORDER BY timestamp DESC
LIMIT $2`, username, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []UsageRecord
	for rows.Next() {
		var r UsageRecord
		if err := rows.Scan(&r.NodeID, &r.Username, &r.Timestamp, &r.PID, &r.CPUPercent, &r.MemoryMB, &r.GPUCount, &r.Command, &r.GPUUsage, &r.Cost); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) UpsertUserNodeAccountTx(ctx context.Context, tx *sql.Tx, nodeID string, localUsername string, billingUsername string) error {
	nodeID = strings.TrimSpace(nodeID)
	localUsername = strings.TrimSpace(localUsername)
	billingUsername = strings.TrimSpace(billingUsername)
	if nodeID == "" || localUsername == "" || billingUsername == "" {
		return errors.New("node_id/local_username/billing_username 不能为空")
	}
	_, err := tx.ExecContext(ctx, `
INSERT INTO user_node_accounts(node_id, local_username, billing_username)
VALUES($1,$2,$3)
ON CONFLICT (node_id, local_username) DO UPDATE
SET billing_username=EXCLUDED.billing_username,
    updated_at=NOW()`, nodeID, localUsername, billingUsername)
	return err
}

func (s *Store) ListUserNodeAccountsByBilling(ctx context.Context, billingUsername string, limit int) ([]UserNodeAccount, error) {
	billingUsername = strings.TrimSpace(billingUsername)
	if billingUsername == "" {
		return nil, errors.New("billing_username 不能为空")
	}
	if limit <= 0 || limit > 5000 {
		limit = 500
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT node_id, local_username, billing_username, created_at, updated_at
FROM user_node_accounts
WHERE billing_username=$1
ORDER BY node_id, local_username
LIMIT $2`, billingUsername, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]UserNodeAccount, 0)
	for rows.Next() {
		var v UserNodeAccount
		if err := rows.Scan(&v.NodeID, &v.LocalUsername, &v.BillingUsername, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Store) UpsertUserNodeAccount(ctx context.Context, nodeID string, localUsername string, billingUsername string) error {
	return s.WithTx(ctx, func(tx *sql.Tx) error {
		return s.UpsertUserNodeAccountTx(ctx, tx, nodeID, localUsername, billingUsername)
	})
}

func (s *Store) DeleteUserNodeAccount(ctx context.Context, nodeID string, localUsername string, billingUsername string) error {
	nodeID = strings.TrimSpace(nodeID)
	localUsername = strings.TrimSpace(localUsername)
	billingUsername = strings.TrimSpace(billingUsername)
	if nodeID == "" || localUsername == "" || billingUsername == "" {
		return errors.New("node_id/local_username/billing_username 不能为空")
	}
	res, err := s.db.ExecContext(ctx, `
DELETE FROM user_node_accounts
WHERE node_id=$1 AND local_username=$2 AND billing_username=$3`, nodeID, localUsername, billingUsername)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) UpdateUserNodeAccount(ctx context.Context, oldNodeID string, oldLocalUsername string, oldBillingUsername string, newNodeID string, newLocalUsername string, newBillingUsername string) error {
	oldNodeID = strings.TrimSpace(oldNodeID)
	oldLocalUsername = strings.TrimSpace(oldLocalUsername)
	oldBillingUsername = strings.TrimSpace(oldBillingUsername)
	newNodeID = strings.TrimSpace(newNodeID)
	newLocalUsername = strings.TrimSpace(newLocalUsername)
	newBillingUsername = strings.TrimSpace(newBillingUsername)
	if oldNodeID == "" || oldLocalUsername == "" || oldBillingUsername == "" ||
		newNodeID == "" || newLocalUsername == "" || newBillingUsername == "" {
		return errors.New("参数不能为空")
	}
	return s.WithTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `
DELETE FROM user_node_accounts
WHERE node_id=$1 AND local_username=$2 AND billing_username=$3`, oldNodeID, oldLocalUsername, oldBillingUsername)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return sql.ErrNoRows
		}
		return s.UpsertUserNodeAccountTx(ctx, tx, newNodeID, newLocalUsername, newBillingUsername)
	})
}

func (s *Store) ResolveBillingUsernameTx(ctx context.Context, tx *sql.Tx, nodeID string, localUsername string) (string, bool, error) {
	nodeID = strings.TrimSpace(nodeID)
	localUsername = strings.TrimSpace(localUsername)
	if nodeID == "" || localUsername == "" {
		return "", false, errors.New("node_id/local_username 不能为空")
	}
	var billing string
	err := tx.QueryRowContext(ctx, `
SELECT billing_username
FROM user_node_accounts
WHERE node_id=$1 AND local_username=$2`, nodeID, localUsername).Scan(&billing)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return billing, true, nil
}

func (s *Store) ListRegisteredLocalUsersByNode(ctx context.Context, nodeID string, limit int) ([]string, error) {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return nil, errors.New("node_id 不能为空")
	}
	if limit <= 0 || limit > 200000 {
		limit = 50000
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT local_username
FROM user_node_accounts
WHERE node_id=$1
ORDER BY local_username
LIMIT $2`, nodeID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Store) ListAllowedLocalUsersByNode(ctx context.Context, nodeID string, limit int) ([]string, error) {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return nil, errors.New("node_id 不能为空")
	}
	if limit <= 0 || limit > 200000 {
		limit = 50000
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT DISTINCT local_username FROM (
  SELECT local_username FROM user_node_accounts WHERE node_id=$1
  UNION ALL
  SELECT local_username FROM ssh_whitelist WHERE node_id=$1 OR node_id='*'
) t
ORDER BY local_username
LIMIT $2`, nodeID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Store) IsWhitelisted(ctx context.Context, nodeID string, localUsername string) (bool, error) {
	nodeID = strings.TrimSpace(nodeID)
	localUsername = strings.TrimSpace(localUsername)
	if nodeID == "" || localUsername == "" {
		return false, errors.New("node_id/local_username 不能为空")
	}
	var exists bool
	err := s.db.QueryRowContext(ctx, `
SELECT EXISTS(
  SELECT 1 FROM ssh_whitelist
  WHERE local_username=$2 AND (node_id=$1 OR node_id='*')
)`, nodeID, localUsername).Scan(&exists)
	return exists, err
}

func (s *Store) ListWhitelist(ctx context.Context, nodeID string, limit int) ([]SSHWhitelistEntry, error) {
	nodeID = strings.TrimSpace(nodeID)
	if limit <= 0 || limit > 200000 {
		limit = 1000
	}
	var rows *sql.Rows
	var err error
	if nodeID == "" {
		rows, err = s.db.QueryContext(ctx, `
SELECT node_id, local_username, created_by, created_at, updated_at
FROM ssh_whitelist
ORDER BY node_id, local_username
LIMIT $1`, limit)
	} else {
		rows, err = s.db.QueryContext(ctx, `
SELECT node_id, local_username, created_by, created_at, updated_at
FROM ssh_whitelist
WHERE node_id=$1
ORDER BY local_username
LIMIT $2`, nodeID, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]SSHWhitelistEntry, 0)
	for rows.Next() {
		var v SSHWhitelistEntry
		if err := rows.Scan(&v.NodeID, &v.LocalUsername, &v.CreatedBy, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Store) UpsertWhitelist(ctx context.Context, nodeID string, usernames []string, createdBy string) error {
	nodeID = strings.TrimSpace(nodeID)
	createdBy = strings.TrimSpace(createdBy)
	if nodeID == "" {
		return errors.New("node_id 不能为空")
	}
	if createdBy == "" {
		createdBy = "admin"
	}
	if len(usernames) == 0 {
		return errors.New("usernames 不能为空")
	}
	return s.WithTx(ctx, func(tx *sql.Tx) error {
		for _, u := range usernames {
			u = strings.TrimSpace(u)
			if u == "" {
				continue
			}
			if _, err := tx.ExecContext(ctx, `
INSERT INTO ssh_whitelist(node_id, local_username, created_by, updated_at)
VALUES($1,$2,$3,NOW())
ON CONFLICT (node_id, local_username) DO UPDATE
SET created_by=EXCLUDED.created_by, updated_at=NOW()`, nodeID, u, createdBy); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) DeleteWhitelist(ctx context.Context, nodeID string, localUsername string) error {
	nodeID = strings.TrimSpace(nodeID)
	localUsername = strings.TrimSpace(localUsername)
	if nodeID == "" || localUsername == "" {
		return errors.New("node_id/local_username 不能为空")
	}
	res, err := s.db.ExecContext(ctx, `
DELETE FROM ssh_whitelist
WHERE node_id=$1 AND local_username=$2`, nodeID, localUsername)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) CreateUserRequestTx(
	ctx context.Context,
	tx *sql.Tx,
	requestType string,
	billingUsername string,
	nodeID string,
	localUsername string,
	message string,
) (int, error) {
	requestType = strings.TrimSpace(requestType)
	billingUsername = strings.TrimSpace(billingUsername)
	nodeID = strings.TrimSpace(nodeID)
	localUsername = strings.TrimSpace(localUsername)
	message = strings.TrimSpace(message)

	if requestType != "bind" && requestType != "open" {
		return 0, errors.New("request_type 仅支持 bind/open")
	}
	if billingUsername == "" || nodeID == "" || localUsername == "" {
		return 0, errors.New("billing_username/node_id/local_username 不能为空")
	}

	var id int
	err := tx.QueryRowContext(ctx, `
INSERT INTO user_requests(request_type, billing_username, node_id, local_username, message, status)
VALUES($1,$2,$3,$4,$5,'pending')
RETURNING request_id`, requestType, billingUsername, nodeID, localUsername, message).Scan(&id)
	return id, err
}

func (s *Store) ListUserRequestsByBilling(ctx context.Context, billingUsername string, limit int) ([]UserRequest, error) {
	billingUsername = strings.TrimSpace(billingUsername)
	if billingUsername == "" {
		return nil, errors.New("billing_username 不能为空")
	}
	if limit <= 0 || limit > 5000 {
		limit = 200
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT request_id, request_type, billing_username, node_id, local_username, message, status,
       reviewed_by, reviewed_at, created_at, updated_at
FROM user_requests
WHERE billing_username=$1
ORDER BY created_at DESC
LIMIT $2`, billingUsername, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []UserRequest
	for rows.Next() {
		var r UserRequest
		var reviewedBy sql.NullString
		var reviewedAt sql.NullTime
		if err := rows.Scan(
			&r.RequestID, &r.RequestType, &r.BillingUsername, &r.NodeID, &r.LocalUsername,
			&r.Message, &r.Status, &reviewedBy, &reviewedAt, &r.CreatedAt, &r.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if reviewedBy.Valid {
			v := reviewedBy.String
			r.ReviewedBy = &v
		}
		if reviewedAt.Valid {
			v := reviewedAt.Time
			r.ReviewedAt = &v
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) ListUserRequestsAdmin(ctx context.Context, status string, limit int) ([]UserRequest, error) {
	status = strings.TrimSpace(status)
	if limit <= 0 || limit > 5000 {
		limit = 200
	}

	var rows *sql.Rows
	var err error
	if status == "" {
		rows, err = s.db.QueryContext(ctx, `
SELECT request_id, request_type, billing_username, node_id, local_username, message, status,
       reviewed_by, reviewed_at, created_at, updated_at
FROM user_requests
ORDER BY created_at DESC
LIMIT $1`, limit)
	} else {
		rows, err = s.db.QueryContext(ctx, `
SELECT request_id, request_type, billing_username, node_id, local_username, message, status,
       reviewed_by, reviewed_at, created_at, updated_at
FROM user_requests
WHERE status=$1
ORDER BY created_at DESC
LIMIT $2`, status, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []UserRequest
	for rows.Next() {
		var r UserRequest
		var reviewedBy sql.NullString
		var reviewedAt sql.NullTime
		if err := rows.Scan(
			&r.RequestID, &r.RequestType, &r.BillingUsername, &r.NodeID, &r.LocalUsername,
			&r.Message, &r.Status, &reviewedBy, &reviewedAt, &r.CreatedAt, &r.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if reviewedBy.Valid {
			v := reviewedBy.String
			r.ReviewedBy = &v
		}
		if reviewedAt.Valid {
			v := reviewedAt.Time
			r.ReviewedAt = &v
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) ReviewUserRequestTx(
	ctx context.Context,
	tx *sql.Tx,
	requestID int,
	newStatus string,
	reviewedBy string,
	reviewedAt time.Time,
) (UserRequest, error) {
	if requestID <= 0 {
		return UserRequest{}, errors.New("request_id 不合法")
	}
	newStatus = strings.TrimSpace(newStatus)
	reviewedBy = strings.TrimSpace(reviewedBy)
	if newStatus != "approved" && newStatus != "rejected" {
		return UserRequest{}, errors.New("status 仅支持 approved/rejected")
	}
	if reviewedBy == "" {
		reviewedBy = "admin"
	}

	// 锁住记录，避免并发重复审批
	var r UserRequest
	var reviewedByPrev sql.NullString
	var reviewedAtPrev sql.NullTime
	if err := tx.QueryRowContext(ctx, `
SELECT request_id, request_type, billing_username, node_id, local_username, message, status,
       reviewed_by, reviewed_at, created_at, updated_at
FROM user_requests
WHERE request_id=$1
FOR UPDATE`, requestID).Scan(
		&r.RequestID, &r.RequestType, &r.BillingUsername, &r.NodeID, &r.LocalUsername,
		&r.Message, &r.Status, &reviewedByPrev, &reviewedAtPrev, &r.CreatedAt, &r.UpdatedAt,
	); err != nil {
		return UserRequest{}, err
	}
	if r.Status != "pending" {
		return UserRequest{}, errors.New("该申请已处理，不能重复审核")
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE user_requests
SET status=$2, reviewed_by=$3, reviewed_at=$4, updated_at=NOW()
WHERE request_id=$1`, requestID, newStatus, reviewedBy, reviewedAt); err != nil {
		return UserRequest{}, err
	}

	// bind 申请在 approved 时，写入映射表，供计费与 SSH 校验使用
	if newStatus == "approved" && r.RequestType == "bind" {
		if err := s.UpsertUserNodeAccountTx(ctx, tx, r.NodeID, r.LocalUsername, r.BillingUsername); err != nil {
			return UserRequest{}, err
		}
	}

	r.Status = newStatus
	r.ReviewedBy = &reviewedBy
	r.ReviewedAt = &reviewedAt
	r.UpdatedAt = reviewedAt
	return r, nil
}

func (s *Store) UpsertNodeStatusTx(
	ctx context.Context,
	tx *sql.Tx,
	nodeID string,
	lastSeenAt time.Time,
	reportID string,
	reportTS time.Time,
	intervalSeconds int,
	cpuModel string,
	cpuCount int,
	gpuModel string,
	gpuCount int,
	netRxBytes uint64,
	netTxBytes uint64,
	gpuProcCount int,
	cpuProcCount int,
	usageRecordsCount int,
	costTotal float64,
) error {
	nodeID = strings.TrimSpace(nodeID)
	reportID = strings.TrimSpace(reportID)
	if nodeID == "" || reportID == "" {
		return errors.New("node_id/report_id 不能为空")
	}
	if intervalSeconds <= 0 {
		intervalSeconds = 60
	}
	cpuModel = strings.TrimSpace(cpuModel)
	gpuModel = strings.TrimSpace(gpuModel)
	if cpuCount < 0 {
		cpuCount = 0
	}
	if gpuCount < 0 {
		gpuCount = 0
	}

	month := lastSeenAt.Format("2006-01")
	var prevRx int64
	var prevTx int64
	var prevMonth string
	var prevRxMBMonth float64
	var prevTxMBMonth float64
	_ = tx.QueryRowContext(ctx, `
SELECT net_rx_bytes, net_tx_bytes, traffic_month, net_rx_mb_month, net_tx_mb_month
FROM nodes
WHERE node_id=$1
FOR UPDATE`, nodeID).Scan(&prevRx, &prevTx, &prevMonth, &prevRxMBMonth, &prevTxMBMonth)

	rxMBMonth := prevRxMBMonth
	txMBMonth := prevTxMBMonth
	if prevMonth != month {
		rxMBMonth = 0
		txMBMonth = 0
	}
	if prevRx >= 0 && int64(netRxBytes) >= prevRx {
		rxMBMonth += float64(int64(netRxBytes)-prevRx) / 1024.0 / 1024.0
	}
	if prevTx >= 0 && int64(netTxBytes) >= prevTx {
		txMBMonth += float64(int64(netTxBytes)-prevTx) / 1024.0 / 1024.0
	}

	_, err := tx.ExecContext(ctx, `
INSERT INTO nodes(
  node_id, last_seen_at, last_report_id, last_report_ts, interval_seconds,
  cpu_model, cpu_count, gpu_model, gpu_count, net_rx_bytes, net_tx_bytes, net_rx_mb_month, net_tx_mb_month, traffic_month,
  gpu_process_count, cpu_process_count, usage_records_count, cost_total, updated_at
)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,NOW())
ON CONFLICT (node_id) DO UPDATE SET
  last_seen_at=EXCLUDED.last_seen_at,
  last_report_id=EXCLUDED.last_report_id,
  last_report_ts=EXCLUDED.last_report_ts,
  interval_seconds=EXCLUDED.interval_seconds,
  cpu_model=EXCLUDED.cpu_model,
  cpu_count=EXCLUDED.cpu_count,
  gpu_model=EXCLUDED.gpu_model,
  gpu_count=EXCLUDED.gpu_count,
  net_rx_bytes=EXCLUDED.net_rx_bytes,
  net_tx_bytes=EXCLUDED.net_tx_bytes,
  net_rx_mb_month=EXCLUDED.net_rx_mb_month,
  net_tx_mb_month=EXCLUDED.net_tx_mb_month,
  traffic_month=EXCLUDED.traffic_month,
  gpu_process_count=EXCLUDED.gpu_process_count,
  cpu_process_count=EXCLUDED.cpu_process_count,
  usage_records_count=EXCLUDED.usage_records_count,
  cost_total=EXCLUDED.cost_total,
  updated_at=NOW()
`, nodeID, lastSeenAt, reportID, reportTS, intervalSeconds,
		cpuModel, cpuCount, gpuModel, gpuCount,
		int64(netRxBytes), int64(netTxBytes), rxMBMonth, txMBMonth, month,
		gpuProcCount, cpuProcCount, usageRecordsCount, costTotal)
	return err
}

func (s *Store) ListNodes(ctx context.Context, limit int) ([]NodeStatus, error) {
	if limit <= 0 || limit > 2000 {
		limit = 200
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT node_id, last_seen_at, last_report_id, last_report_ts, interval_seconds,
       cpu_model, cpu_count, gpu_model, gpu_count, net_rx_mb_month, net_tx_mb_month,
       gpu_process_count, cpu_process_count, usage_records_count, cost_total, updated_at
FROM nodes
ORDER BY last_seen_at DESC
LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []NodeStatus
	for rows.Next() {
		var n NodeStatus
		if err := rows.Scan(
			&n.NodeID,
			&n.LastSeenAt,
			&n.LastReportID,
			&n.LastReportTS,
			&n.IntervalSeconds,
			&n.CPUModel,
			&n.CPUCount,
			&n.GPUModel,
			&n.GPUCount,
			&n.NetRxMBMonth,
			&n.NetTxMBMonth,
			&n.GPUProcessCount,
			&n.CPUProcessCount,
			&n.UsageRecordsCount,
			&n.CostTotal,
			&n.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Store) CountAdminAccounts(ctx context.Context) (int, error) {
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM admin_accounts`).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func (s *Store) CreateAdminAccount(ctx context.Context, username string, password string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return errors.New("username 不能为空")
	}
	if len(password) < 8 {
		return errors.New("password 至少 8 位")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO admin_accounts(username, password_hash)
VALUES($1,$2)
ON CONFLICT (username) DO NOTHING`, username, string(hash))
	return err
}

func (s *Store) VerifyAdminPassword(ctx context.Context, username string, password string) (bool, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return false, errors.New("username 不能为空")
	}
	var hash string
	err := s.db.QueryRowContext(ctx, `SELECT password_hash FROM admin_accounts WHERE username=$1`, username).Scan(&hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return false, nil
	}
	_, _ = s.db.ExecContext(ctx, `UPDATE admin_accounts SET last_login_at=NOW(), updated_at=NOW() WHERE username=$1`, username)
	return true, nil
}

func (s *Store) CreateUserAccountTx(ctx context.Context, tx *sql.Tx, in UserAccount, password string, defaultBalance float64) error {
	in.Username = strings.TrimSpace(in.Username)
	in.Email = strings.TrimSpace(strings.ToLower(in.Email))
	in.RealName = strings.TrimSpace(in.RealName)
	in.StudentID = strings.TrimSpace(in.StudentID)
	in.Advisor = strings.TrimSpace(in.Advisor)
	in.Phone = strings.TrimSpace(in.Phone)
	if in.Username == "" || in.Email == "" || in.RealName == "" || in.StudentID == "" || in.Advisor == "" || in.Phone == "" {
		return errors.New("注册信息不完整")
	}
	if len(password) < 8 {
		return errors.New("password 至少 8 位")
	}
	if in.ExpectedGraduationYear < 2000 || in.ExpectedGraduationYear > 2200 {
		return errors.New("expected_graduation_year 不合法")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO user_accounts(
  username, email, password_hash, real_name, student_id, advisor, expected_graduation_year, phone, role
)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,'user')`,
		in.Username, in.Email, string(hash), in.RealName, in.StudentID, in.Advisor, in.ExpectedGraduationYear, in.Phone); err != nil {
		return err
	}
	_, err = s.EnsureUserTx(ctx, tx, in.Username, defaultBalance)
	return err
}

func (s *Store) VerifyUserPassword(ctx context.Context, username string, password string) (bool, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return false, errors.New("username 不能为空")
	}
	var hash string
	err := s.db.QueryRowContext(ctx, `SELECT password_hash FROM user_accounts WHERE username=$1`, username).Scan(&hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return false, nil
	}
	_, _ = s.db.ExecContext(ctx, `UPDATE user_accounts SET last_login_at=NOW(), updated_at=NOW() WHERE username=$1`, username)
	return true, nil
}

func (s *Store) GetUserAccountByUsername(ctx context.Context, username string) (UserAccount, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return UserAccount{}, errors.New("username 不能为空")
	}
	var out UserAccount
	err := s.db.QueryRowContext(ctx, `
SELECT username, email, real_name, student_id, advisor, expected_graduation_year, phone, role, last_login_at, created_at, updated_at
FROM user_accounts
WHERE username=$1`, username).Scan(
		&out.Username, &out.Email, &out.RealName, &out.StudentID, &out.Advisor, &out.ExpectedGraduationYear,
		&out.Phone, &out.Role, &out.LastLoginAt, &out.CreatedAt, &out.UpdatedAt,
	)
	return out, err
}

func (s *Store) UpdateUserPassword(ctx context.Context, username string, oldPassword string, newPassword string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return errors.New("username 不能为空")
	}
	if len(newPassword) < 8 {
		return errors.New("新密码至少 8 位")
	}
	var oldHash string
	if err := s.db.QueryRowContext(ctx, `SELECT password_hash FROM user_accounts WHERE username=$1`, username).Scan(&oldHash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("账号不存在")
		}
		return err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(oldHash), []byte(oldPassword)); err != nil {
		return errors.New("旧密码不正确")
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
UPDATE user_accounts
SET password_hash=$2, updated_at=NOW()
WHERE username=$1`, username, string(newHash))
	return err
}

func (s *Store) UpdateAdminPassword(ctx context.Context, username string, oldPassword string, newPassword string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return errors.New("username 不能为空")
	}
	if len(newPassword) < 8 {
		return errors.New("新密码至少 8 位")
	}
	var oldHash string
	if err := s.db.QueryRowContext(ctx, `SELECT password_hash FROM admin_accounts WHERE username=$1`, username).Scan(&oldHash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("账号不存在")
		}
		return err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(oldHash), []byte(oldPassword)); err != nil {
		return errors.New("旧密码不正确")
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
UPDATE admin_accounts
SET password_hash=$2, updated_at=NOW()
WHERE username=$1`, username, string(newHash))
	return err
}

func (s *Store) SetPasswordResetTokenByEmail(ctx context.Context, email string, tokenHash string, expireAt time.Time) (string, bool, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return "", false, errors.New("email 不能为空")
	}
	if tokenHash == "" {
		return "", false, errors.New("token_hash 不能为空")
	}
	var username string
	err := s.db.QueryRowContext(ctx, `
UPDATE user_accounts
SET reset_token_hash=$2, reset_token_expire_at=$3, updated_at=NOW()
WHERE email=$1
RETURNING username`, email, tokenHash, expireAt).Scan(&username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, err
	}
	return username, true, nil
}

func (s *Store) ResetPasswordByToken(ctx context.Context, username string, tokenHash string, newPassword string, now time.Time) error {
	username = strings.TrimSpace(username)
	if username == "" || tokenHash == "" {
		return errors.New("username/token 不能为空")
	}
	if len(newPassword) < 8 {
		return errors.New("新密码至少 8 位")
	}
	var dbHash sql.NullString
	var expireAt sql.NullTime
	err := s.db.QueryRowContext(ctx, `
SELECT reset_token_hash, reset_token_expire_at
FROM user_accounts
WHERE username=$1`, username).Scan(&dbHash, &expireAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("账号不存在")
		}
		return err
	}
	if !dbHash.Valid || !expireAt.Valid || strings.TrimSpace(dbHash.String) == "" {
		return errors.New("重置链接无效")
	}
	if now.After(expireAt.Time) {
		return errors.New("重置链接已过期")
	}
	if dbHash.String != tokenHash {
		return errors.New("重置链接无效")
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
UPDATE user_accounts
SET password_hash=$2, reset_token_hash=NULL, reset_token_expire_at=NULL, updated_at=NOW()
WHERE username=$1`, username, string(newHash))
	return err
}

func (s *Store) GetMailSettings(ctx context.Context, cfg Config) (MailSettings, error) {
	out := MailSettings{
		SMTPHost:  strings.TrimSpace(cfg.SMTPHost),
		SMTPPort:  cfg.SMTPPort,
		SMTPUser:  strings.TrimSpace(cfg.SMTPUser),
		SMTPPass:  strings.TrimSpace(cfg.SMTPPass),
		FromEmail: strings.TrimSpace(cfg.FromEmail),
		FromName:  strings.TrimSpace(cfg.FromName),
	}
	if out.FromName == "" {
		out.FromName = "HIT-AIOT-OPS团队"
	}
	rows, err := s.db.QueryContext(ctx, `SELECT key, value FROM app_settings WHERE key = ANY($1)`, pq.Array([]string{
		appSettingSMTPHost, appSettingSMTPPort, appSettingSMTPUser, appSettingSMTPPass, appSettingFromEmail, appSettingFromName,
	}))
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return out, err
		}
		switch key {
		case appSettingSMTPHost:
			out.SMTPHost = strings.TrimSpace(value)
		case appSettingSMTPPort:
			if n, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
				out.SMTPPort = n
			}
		case appSettingSMTPUser:
			out.SMTPUser = strings.TrimSpace(value)
		case appSettingSMTPPass:
			out.SMTPPass = strings.TrimSpace(value)
		case appSettingFromEmail:
			out.FromEmail = strings.TrimSpace(value)
		case appSettingFromName:
			out.FromName = strings.TrimSpace(value)
		}
	}
	if err := rows.Err(); err != nil {
		return out, err
	}
	if out.FromName == "" {
		out.FromName = "HIT-AIOT-OPS团队"
	}
	return out, nil
}

func (s *Store) UpsertMailSettings(ctx context.Context, settings MailSettings, updatePassword bool) error {
	settings.SMTPHost = strings.TrimSpace(settings.SMTPHost)
	settings.SMTPUser = strings.TrimSpace(settings.SMTPUser)
	settings.SMTPPass = strings.TrimSpace(settings.SMTPPass)
	settings.FromEmail = strings.TrimSpace(settings.FromEmail)
	settings.FromName = strings.TrimSpace(settings.FromName)
	if settings.FromName == "" {
		settings.FromName = "HIT-AIOT-OPS团队"
	}
	if settings.SMTPPort < 0 || settings.SMTPPort > 65535 {
		return errors.New("smtp_port 不合法")
	}

	type kv struct {
		k string
		v string
	}
	items := []kv{
		{k: appSettingSMTPHost, v: settings.SMTPHost},
		{k: appSettingSMTPPort, v: strconv.Itoa(settings.SMTPPort)},
		{k: appSettingSMTPUser, v: settings.SMTPUser},
		{k: appSettingFromEmail, v: settings.FromEmail},
		{k: appSettingFromName, v: settings.FromName},
	}
	if updatePassword {
		items = append(items, kv{k: appSettingSMTPPass, v: settings.SMTPPass})
	}
	for _, it := range items {
		if _, err := s.db.ExecContext(ctx, `
INSERT INTO app_settings(key, value, updated_at)
VALUES($1,$2,NOW())
ON CONFLICT (key) DO UPDATE
SET value=EXCLUDED.value, updated_at=NOW()`, it.k, it.v); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) GetUserEmailByUsername(ctx context.Context, username string) (string, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return "", errors.New("username 不能为空")
	}
	var email string
	if err := s.db.QueryRowContext(ctx, `SELECT email FROM user_accounts WHERE username=$1`, username).Scan(&email); err != nil {
		return "", err
	}
	return strings.TrimSpace(email), nil
}

func (s *Store) ListUsageSummaryByUser(ctx context.Context, from time.Time, to time.Time, limit int) ([]UsageUserSummary, error) {
	if limit <= 0 || limit > 10000 {
		limit = 1000
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT username,
       COUNT(1) AS usage_records,
       SUM(CASE WHEN gpu_count > 0 THEN 1 ELSE 0 END) AS gpu_process_records,
       SUM(CASE WHEN gpu_count = 0 THEN 1 ELSE 0 END) AS cpu_process_records,
       COALESCE(SUM(cpu_percent), 0) AS total_cpu_percent,
       COALESCE(SUM(memory_mb), 0) AS total_memory_mb,
       COALESCE(SUM(cost), 0) AS total_cost
FROM usage_records
WHERE timestamp >= $1 AND timestamp <= $2
GROUP BY username
ORDER BY total_cost DESC
LIMIT $3`, from, to, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]UsageUserSummary, 0)
	for rows.Next() {
		var x UsageUserSummary
		if err := rows.Scan(&x.Username, &x.UsageRecords, &x.GPUProcessRecords, &x.CPUProcessRecords, &x.TotalCPUPercent, &x.TotalMemoryMB, &x.TotalCost); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *Store) ListUsageMonthlyByUser(ctx context.Context, from time.Time, to time.Time, limit int) ([]UsageMonthlySummary, error) {
	if limit <= 0 || limit > 200000 {
		limit = 20000
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT to_char(date_trunc('month', timestamp), 'YYYY-MM') AS month,
       username,
       COUNT(1) AS usage_records,
       SUM(CASE WHEN gpu_count > 0 THEN 1 ELSE 0 END) AS gpu_process_records,
       SUM(CASE WHEN gpu_count = 0 THEN 1 ELSE 0 END) AS cpu_process_records,
       COALESCE(SUM(cpu_percent), 0) AS total_cpu_percent,
       COALESCE(SUM(memory_mb), 0) AS total_memory_mb,
       COALESCE(SUM(cost), 0) AS total_cost
FROM usage_records
WHERE timestamp >= $1 AND timestamp <= $2
GROUP BY 1,2
ORDER BY month DESC, total_cost DESC
LIMIT $3`, from, to, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]UsageMonthlySummary, 0)
	for rows.Next() {
		var x UsageMonthlySummary
		if err := rows.Scan(&x.Month, &x.Username, &x.UsageRecords, &x.GPUProcessRecords, &x.CPUProcessRecords, &x.TotalCPUPercent, &x.TotalMemoryMB, &x.TotalCost); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *Store) ListRechargeSummary(ctx context.Context, from time.Time, to time.Time, limit int) ([]RechargeSummary, error) {
	if limit <= 0 || limit > 10000 {
		limit = 1000
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT username,
       COUNT(1) AS recharge_count,
       COALESCE(SUM(amount), 0) AS recharge_total,
       MAX(created_at) AS last_recharge
FROM recharge_records
WHERE created_at >= $1 AND created_at <= $2
GROUP BY username
ORDER BY recharge_total DESC
LIMIT $3`, from, to, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]RechargeSummary, 0)
	for rows.Next() {
		var x RechargeSummary
		if err := rows.Scan(&x.Username, &x.RechargeCount, &x.RechargeTotal, &x.LastRecharge); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *Store) queryUsageRows(
	ctx context.Context,
	username string,
	hasFrom bool,
	from time.Time,
	hasTo bool,
	to time.Time,
	limit int,
) (*sql.Rows, error) {
	if limit <= 0 || limit > 200000 {
		limit = 20000
	}

	conds := []string{}
	args := []any{}
	argN := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	if strings.TrimSpace(username) != "" {
		conds = append(conds, "username="+argN(username))
	}
	if hasFrom {
		conds = append(conds, "timestamp>="+argN(from))
	}
	if hasTo {
		conds = append(conds, "timestamp<="+argN(to))
	}

	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}

	query := fmt.Sprintf(`
SELECT node_id, username, timestamp, cpu_percent, memory_mb, gpu_usage::text, cost
FROM usage_records
%s
ORDER BY timestamp ASC
LIMIT %s
`, where, argN(limit))

	return s.db.QueryContext(ctx, query, args...)
}
