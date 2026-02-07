-- 0006_user_node_accounts.sql：用户“节点账号”绑定（用于按节点维度映射扣费与 SSH 登录校验）

CREATE TABLE IF NOT EXISTS user_node_accounts (
    node_id VARCHAR(50) NOT NULL,
    local_username VARCHAR(50) NOT NULL,
    billing_username VARCHAR(50) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (node_id, local_username)
);

CREATE INDEX IF NOT EXISTS idx_user_node_accounts_billing ON user_node_accounts(billing_username);

