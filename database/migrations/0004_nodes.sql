-- 0004_nodes.sql：节点状态表（用于运维查看在线/上报情况）

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

