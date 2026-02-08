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
	PID        int32     `json:"pid"`
	CPUPercent float64   `json:"cpu_percent"`
	MemoryMB   float64   `json:"memory_mb"`
	GPUCount   int       `json:"gpu_count"`
	Command    string    `json:"command"`
	GPUUsage   string    `json:"gpu_usage"` // JSON 字符串（原样返回）
	Cost       float64   `json:"cost"`
}

type NodeStatus struct {
	NodeID            string    `json:"node_id"`
	LastSeenAt        time.Time `json:"last_seen_at"`
	LastReportID      string    `json:"last_report_id"`
	LastReportTS      time.Time `json:"last_report_ts"`
	IntervalSeconds   int       `json:"interval_seconds"`
	CPUModel          string    `json:"cpu_model"`
	CPUCount          int       `json:"cpu_count"`
	GPUModel          string    `json:"gpu_model"`
	GPUCount          int       `json:"gpu_count"`
	NetRxMBMonth      float64   `json:"net_rx_mb_month"`
	NetTxMBMonth      float64   `json:"net_tx_mb_month"`
	GPUProcessCount   int       `json:"gpu_process_count"`
	CPUProcessCount   int       `json:"cpu_process_count"`
	UsageRecordsCount int       `json:"usage_records_count"`
	CostTotal         float64   `json:"cost_total"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// UserNodeAccount 表示“节点本地账号”到“计费账号”的映射。
// 约定：node_id 为机器编号（可用端口号，例如 60000）；local_username 为该节点上的 Linux 用户名。
type UserNodeAccount struct {
	NodeID          string    `json:"node_id"`
	LocalUsername   string    `json:"local_username"`
	BillingUsername string    `json:"billing_username"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type SSHWhitelistEntry struct {
	NodeID        string    `json:"node_id"`
	LocalUsername string    `json:"local_username"`
	CreatedBy     string    `json:"created_by"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type UserRequest struct {
	RequestID       int        `json:"request_id"`
	RequestType     string     `json:"request_type"` // bind/open
	BillingUsername string     `json:"billing_username"`
	NodeID          string     `json:"node_id"`
	LocalUsername   string     `json:"local_username"`
	Message         string     `json:"message"`
	Status          string     `json:"status"` // pending/approved/rejected
	ReviewedBy      *string    `json:"reviewed_by,omitempty"`
	ReviewedAt      *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type UserAccount struct {
	Username               string     `json:"username"`
	Email                  string     `json:"email"`
	RealName               string     `json:"real_name"`
	StudentID              string     `json:"student_id"`
	Advisor                string     `json:"advisor"`
	ExpectedGraduationYear int        `json:"expected_graduation_year"`
	Phone                  string     `json:"phone"`
	Role                   string     `json:"role"`
	LastLoginAt            *time.Time `json:"last_login_at,omitempty"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
}

type MailSettings struct {
	SMTPHost  string `json:"smtp_host"`
	SMTPPort  int    `json:"smtp_port"`
	SMTPUser  string `json:"smtp_user"`
	SMTPPass  string `json:"smtp_pass,omitempty"`
	FromEmail string `json:"from_email"`
	FromName  string `json:"from_name"`
}

type UsageUserSummary struct {
	Username          string  `json:"username"`
	UsageRecords      int     `json:"usage_records"`
	GPUProcessRecords int     `json:"gpu_process_records"`
	CPUProcessRecords int     `json:"cpu_process_records"`
	TotalCPUPercent   float64 `json:"total_cpu_percent"`
	TotalMemoryMB     float64 `json:"total_memory_mb"`
	TotalCost         float64 `json:"total_cost"`
}

type UsageMonthlySummary struct {
	Month             string  `json:"month"`
	Username          string  `json:"username"`
	UsageRecords      int     `json:"usage_records"`
	GPUProcessRecords int     `json:"gpu_process_records"`
	CPUProcessRecords int     `json:"cpu_process_records"`
	TotalCPUPercent   float64 `json:"total_cpu_percent"`
	TotalMemoryMB     float64 `json:"total_memory_mb"`
	TotalCost         float64 `json:"total_cost"`
}

type RechargeSummary struct {
	Username      string    `json:"username"`
	RechargeCount int       `json:"recharge_count"`
	RechargeTotal float64   `json:"recharge_total"`
	LastRecharge  time.Time `json:"last_recharge"`
}
