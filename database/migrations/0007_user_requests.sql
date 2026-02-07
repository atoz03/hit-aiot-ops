-- 0007_user_requests.sql：用户自助登记/开号申请（管理员审核）

CREATE TABLE IF NOT EXISTS user_requests (
    request_id SERIAL PRIMARY KEY,
    request_type VARCHAR(20) NOT NULL, -- bind(绑定登记), open(开号申请)
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

CREATE INDEX IF NOT EXISTS idx_user_requests_status ON user_requests(status);
CREATE INDEX IF NOT EXISTS idx_user_requests_billing ON user_requests(billing_username);

