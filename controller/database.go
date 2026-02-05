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
	"strings"
	"time"

	_ "github.com/lib/pq"
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
INSERT INTO usage_records(node_id, username, timestamp, cpu_percent, memory_mb, gpu_usage, cost)
VALUES($1,$2,$3,$4,$5,$6,$7)`,
		nodeID, proc.Username, ts, proc.CPUPercent, proc.MemoryMB, string(gpuJSON), cost)
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
SELECT node_id, username, timestamp, cpu_percent, memory_mb, gpu_usage, cost
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
		if err := rows.Scan(&r.NodeID, &r.Username, &r.Timestamp, &r.CPUPercent, &r.MemoryMB, &r.GPUUsage, &r.Cost); err != nil {
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
SELECT node_id, username, timestamp, cpu_percent, memory_mb, gpu_usage, cost
FROM usage_records
ORDER BY timestamp DESC
LIMIT $1`, limit)
	} else {
		rows, err = s.db.QueryContext(ctx, `
SELECT node_id, username, timestamp, cpu_percent, memory_mb, gpu_usage, cost
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
		if err := rows.Scan(&r.NodeID, &r.Username, &r.Timestamp, &r.CPUPercent, &r.MemoryMB, &r.GPUUsage, &r.Cost); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) UpsertNodeStatusTx(
	ctx context.Context,
	tx *sql.Tx,
	nodeID string,
	lastSeenAt time.Time,
	reportID string,
	reportTS time.Time,
	intervalSeconds int,
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

	_, err := tx.ExecContext(ctx, `
INSERT INTO nodes(
  node_id, last_seen_at, last_report_id, last_report_ts, interval_seconds,
  gpu_process_count, cpu_process_count, usage_records_count, cost_total, updated_at
)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,NOW())
ON CONFLICT (node_id) DO UPDATE SET
  last_seen_at=EXCLUDED.last_seen_at,
  last_report_id=EXCLUDED.last_report_id,
  last_report_ts=EXCLUDED.last_report_ts,
  interval_seconds=EXCLUDED.interval_seconds,
  gpu_process_count=EXCLUDED.gpu_process_count,
  cpu_process_count=EXCLUDED.cpu_process_count,
  usage_records_count=EXCLUDED.usage_records_count,
  cost_total=EXCLUDED.cost_total,
  updated_at=NOW()
`, nodeID, lastSeenAt, reportID, reportTS, intervalSeconds, gpuProcCount, cpuProcCount, usageRecordsCount, costTotal)
	return err
}

func (s *Store) ListNodes(ctx context.Context, limit int) ([]NodeStatus, error) {
	if limit <= 0 || limit > 2000 {
		limit = 200
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT node_id, last_seen_at, last_report_id, last_report_ts, interval_seconds,
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
