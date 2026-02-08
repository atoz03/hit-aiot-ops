-- 0010_whitelist_and_node_hardware.sql：SSH 白名单 + 节点硬件/流量统计

CREATE TABLE IF NOT EXISTS ssh_whitelist (
    node_id VARCHAR(50) NOT NULL,         -- 具体节点或 "*" 表示所有节点
    local_username VARCHAR(50) NOT NULL,
    created_by VARCHAR(50) NOT NULL DEFAULT 'admin',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (node_id, local_username)
);

ALTER TABLE nodes
    ADD COLUMN IF NOT EXISTS cpu_model TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS cpu_count INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS gpu_model TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS gpu_count INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS net_rx_bytes BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS net_tx_bytes BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS net_rx_mb_month DOUBLE PRECISION NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS net_tx_mb_month DOUBLE PRECISION NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS traffic_month VARCHAR(7) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_ssh_whitelist_user ON ssh_whitelist(local_username);
