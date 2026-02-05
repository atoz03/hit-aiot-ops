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

CREATE INDEX IF NOT EXISTS idx_usage_username ON usage_records(username);
CREATE INDEX IF NOT EXISTS idx_usage_timestamp ON usage_records(timestamp);
CREATE INDEX IF NOT EXISTS idx_usage_node ON usage_records(node_id);

