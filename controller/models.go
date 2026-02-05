package main

import "time"

// MetricsData 为 Agent 每次上报的数据结构。
type MetricsData struct {
	NodeID    string `json:"node_id"`
	Timestamp string `json:"timestamp"` // RFC3339
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
