-- 0009_usage_record_details.sql：usage_records 增加统计明细字段

ALTER TABLE usage_records
    ADD COLUMN IF NOT EXISTS pid INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS gpu_count INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS command TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_usage_timestamp_username ON usage_records(timestamp, username);
