-- 0005_admin_accounts.sql：管理员账号（Web 登录）

CREATE TABLE IF NOT EXISTS admin_accounts (
    username VARCHAR(50) PRIMARY KEY,
    password_hash TEXT NOT NULL,
    last_login_at TIMESTAMP NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

