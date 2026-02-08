-- GPU 集群管理系统数据库结构（用于手工初始化/审计）

CREATE TABLE IF NOT EXISTS users (
    user_id SERIAL PRIMARY KEY,
    username VARCHAR(50) UNIQUE NOT NULL,
    balance DECIMAL(10,2) NOT NULL DEFAULT 100.0,
    status VARCHAR(20) NOT NULL DEFAULT 'normal', -- normal, warning, limited, blocked
    blocked_at TIMESTAMP NULL,
    last_charge_time TIMESTAMP NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS usage_records (
    record_id SERIAL PRIMARY KEY,
    node_id VARCHAR(50) NOT NULL,
    username VARCHAR(50) NOT NULL,
    timestamp TIMESTAMP NOT NULL,
    pid INT NOT NULL DEFAULT 0,
    cpu_percent FLOAT NOT NULL,
    memory_mb FLOAT NOT NULL,
    gpu_count INT NOT NULL DEFAULT 0,
    command TEXT NOT NULL DEFAULT '',
    gpu_usage JSONB NOT NULL,
    cost DECIMAL(10,4) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS recharge_records (
    recharge_id SERIAL PRIMARY KEY,
    username VARCHAR(50) NOT NULL,
    amount DECIMAL(10,2) NOT NULL,
    method VARCHAR(50) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS resource_prices (
    price_id SERIAL PRIMARY KEY,
    gpu_model VARCHAR(50) UNIQUE NOT NULL,
    price_per_minute DECIMAL(10,4) NOT NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS schema_migrations (
    filename TEXT PRIMARY KEY,
    applied_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- 上报幂等表：用于防止 Agent 重试导致重复扣费
CREATE TABLE IF NOT EXISTS metric_reports (
    report_id TEXT PRIMARY KEY,
    node_id VARCHAR(50) NOT NULL,
    timestamp TIMESTAMP NOT NULL,
    interval_seconds INT NOT NULL,
    received_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- 节点状态表：用于运维查看在线/上报情况
CREATE TABLE IF NOT EXISTS nodes (
    node_id VARCHAR(50) PRIMARY KEY,
    last_seen_at TIMESTAMP NOT NULL,
    last_report_id TEXT NOT NULL,
    last_report_ts TIMESTAMP NOT NULL,
    interval_seconds INT NOT NULL,
    cpu_model TEXT NOT NULL DEFAULT '',
    cpu_count INT NOT NULL DEFAULT 0,
    gpu_model TEXT NOT NULL DEFAULT '',
    gpu_count INT NOT NULL DEFAULT 0,
    net_rx_bytes BIGINT NOT NULL DEFAULT 0,
    net_tx_bytes BIGINT NOT NULL DEFAULT 0,
    net_rx_mb_month DOUBLE PRECISION NOT NULL DEFAULT 0,
    net_tx_mb_month DOUBLE PRECISION NOT NULL DEFAULT 0,
    traffic_month VARCHAR(7) NOT NULL DEFAULT '',
    gpu_process_count INT NOT NULL DEFAULT 0,
    cpu_process_count INT NOT NULL DEFAULT 0,
    usage_records_count INT NOT NULL DEFAULT 0,
    cost_total DECIMAL(12,4) NOT NULL DEFAULT 0.0,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- 管理员账号（Web 登录）
CREATE TABLE IF NOT EXISTS admin_accounts (
    username VARCHAR(50) PRIMARY KEY,
    password_hash TEXT NOT NULL,
    last_login_at TIMESTAMP NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- 用户“节点账号”绑定：按节点(node_id=机器编号/端口) + 本地账号(local_username) 映射到计费账号(billing_username)
CREATE TABLE IF NOT EXISTS user_node_accounts (
    node_id VARCHAR(50) NOT NULL,
    local_username VARCHAR(50) NOT NULL,
    billing_username VARCHAR(50) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (node_id, local_username)
);

-- 用户自助登记/开号申请（管理员审核）
CREATE TABLE IF NOT EXISTS user_requests (
    request_id SERIAL PRIMARY KEY,
    request_type VARCHAR(20) NOT NULL, -- bind, open
    billing_username VARCHAR(50) NOT NULL,
    node_id VARCHAR(50) NOT NULL,
    local_username VARCHAR(50) NOT NULL,
    message TEXT NOT NULL DEFAULT '',
    status VARCHAR(20) NOT NULL DEFAULT 'pending', -- pending, approved, rejected
    reviewed_by VARCHAR(50) NULL,
    reviewed_at TIMESTAMP NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_usage_username ON usage_records(username);
CREATE INDEX IF NOT EXISTS idx_usage_timestamp ON usage_records(timestamp);
CREATE INDEX IF NOT EXISTS idx_usage_node ON usage_records(node_id);
CREATE INDEX IF NOT EXISTS idx_usage_timestamp_username ON usage_records(timestamp, username);
CREATE INDEX IF NOT EXISTS idx_user_node_accounts_billing ON user_node_accounts(billing_username);
CREATE INDEX IF NOT EXISTS idx_user_requests_status ON user_requests(status);
CREATE INDEX IF NOT EXISTS idx_user_requests_billing ON user_requests(billing_username);

-- 普通用户账号（Web 登录）
CREATE TABLE IF NOT EXISTS user_accounts (
    username VARCHAR(50) PRIMARY KEY,
    email VARCHAR(120) UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    real_name VARCHAR(80) NOT NULL,
    student_id VARCHAR(40) NOT NULL,
    advisor VARCHAR(80) NOT NULL,
    expected_graduation_year INT NOT NULL,
    phone VARCHAR(40) NOT NULL,
    role VARCHAR(20) NOT NULL DEFAULT 'user',
    last_login_at TIMESTAMP NULL,
    reset_token_hash TEXT NULL,
    reset_token_expire_at TIMESTAMP NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_accounts_email ON user_accounts(email);

-- 应用配置（如 SMTP）
CREATE TABLE IF NOT EXISTS app_settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- SSH 白名单（允许不注册直接登录）
CREATE TABLE IF NOT EXISTS ssh_whitelist (
    node_id VARCHAR(50) NOT NULL,  -- 具体节点或 "*" 表示所有节点
    local_username VARCHAR(50) NOT NULL,
    created_by VARCHAR(50) NOT NULL DEFAULT 'admin',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (node_id, local_username)
);

CREATE INDEX IF NOT EXISTS idx_ssh_whitelist_user ON ssh_whitelist(local_username);
