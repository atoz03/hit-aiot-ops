-- 0003_metric_reports.sql：上报幂等表（防止重试重复扣费）

CREATE TABLE IF NOT EXISTS metric_reports (
    report_id TEXT PRIMARY KEY,
    node_id VARCHAR(50) NOT NULL,
    timestamp TIMESTAMP NOT NULL,
    interval_seconds INT NOT NULL,
    received_at TIMESTAMP NOT NULL DEFAULT NOW()
);

