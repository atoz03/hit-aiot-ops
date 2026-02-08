package main

// 注意：这些结构体与 controller/models.go 的 JSON 字段保持一致，便于直接通信。

type MetricsData struct {
	NodeID          string        `json:"node_id"`
	Timestamp       string        `json:"timestamp"` // RFC3339
	ReportID        string        `json:"report_id"`
	IntervalSeconds int           `json:"interval_seconds,omitempty"`
	CPUModel        string        `json:"cpu_model,omitempty"`
	CPUCount        int           `json:"cpu_count,omitempty"`
	GPUModel        string        `json:"gpu_model,omitempty"`
	GPUCount        int           `json:"gpu_count,omitempty"`
	NetRxBytes      uint64        `json:"net_rx_bytes,omitempty"`
	NetTxBytes      uint64        `json:"net_tx_bytes,omitempty"`
	Users           []UserProcess `json:"users"`
}

type UserProcess struct {
	Username   string     `json:"username"`
	PID        int32      `json:"pid"`
	CPUPercent float64    `json:"cpu_percent"`
	MemoryMB   float64    `json:"memory_mb"`
	GPUUsage   []GPUUsage `json:"gpu_usage"`
	Command    string     `json:"command,omitempty"`
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

type Action struct {
	Type            string  `json:"type"`
	Username        string  `json:"username"`
	PIDs            []int32 `json:"pids,omitempty"`
	Reason          string  `json:"reason,omitempty"`
	Message         string  `json:"message,omitempty"`
	CPUQuotaPercent float64 `json:"cpu_quota_percent,omitempty"`
}
