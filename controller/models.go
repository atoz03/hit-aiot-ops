package main

import "time"

// MetricsData 为 Agent 每次上报的数据结构。
type MetricsData struct {
	NodeID    string `json:"node_id"`
	Timestamp string `json:"timestamp"` // RFC3339
	// ReportID 为单次上报的全局唯一 ID，用于幂等：控制器只处理一次，避免重试导致重复扣费。
	ReportID string `json:"report_id"`
	// IntervalSeconds 为 Agent 上报周期（秒）。为空时由控制器用 sample_interval_seconds 兜底。
	IntervalSeconds int           `json:"interval_seconds,omitempty"`
	Users           []UserProcess `json:"users"`
}

type UserProcess struct {
	Username   string     `json:"username"`
	PID        int32      `json:"pid"`
	CPUPercent float64    `json:"cpu_percent"`
	MemoryMB   float64    `json:"memory_mb"`
	GPUUsage   []GPUUsage `json:"gpu_usage"`
}

type GPUUsage struct {
	GPUID    int32   `json:"gpu_id"`
	GPUModel string  `json:"gpu_model"`
	GPUBusID string  `json:"gpu_bus_id"`
	MemoryMB float64 `json:"memory_mb"`
}

type ControllerResponse struct {
	Actions []Action `json:"actions"`
}

// Action 为控制器下发到节点的动作。
type Action struct {
	Type            string  `json:"type"` // notify, block_user, unblock_user, kill_process
	Username        string  `json:"username"`
	PIDs            []int32 `json:"pids,omitempty"`
	Reason          string  `json:"reason,omitempty"`
	Message         string  `json:"message,omitempty"`
	CPUQuotaPercent float64 `json:"cpu_quota_percent,omitempty"` // set_cpu_quota 使用
}

type User struct {
	Username  string
	Balance   float64
	Status    string
	BlockedAt *time.Time
}

type PriceRow struct {
	Model string
	Price float64
}

type UsageRecord struct {
	NodeID     string    `json:"node_id"`
	Username   string    `json:"username"`
	Timestamp  time.Time `json:"timestamp"`
	CPUPercent float64   `json:"cpu_percent"`
	MemoryMB   float64   `json:"memory_mb"`
	GPUUsage   string    `json:"gpu_usage"` // JSON 字符串（原样返回）
	Cost       float64   `json:"cost"`
}

type NodeStatus struct {
	NodeID            string    `json:"node_id"`
	LastSeenAt        time.Time `json:"last_seen_at"`
	LastReportID      string    `json:"last_report_id"`
	LastReportTS      time.Time `json:"last_report_ts"`
	IntervalSeconds   int       `json:"interval_seconds"`
	GPUProcessCount   int       `json:"gpu_process_count"`
	CPUProcessCount   int       `json:"cpu_process_count"`
	UsageRecordsCount int       `json:"usage_records_count"`
	CostTotal         float64   `json:"cost_total"`
	UpdatedAt         time.Time `json:"updated_at"`
}
