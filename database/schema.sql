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
    cpu_percent FLOAT NOT NULL,
    memory_mb FLOAT NOT NULL,
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

CREATE INDEX IF NOT EXISTS idx_usage_username ON usage_records(username);
CREATE INDEX IF NOT EXISTS idx_usage_timestamp ON usage_records(timestamp);
CREATE INDEX IF NOT EXISTS idx_usage_node ON usage_records(node_id);
