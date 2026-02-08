-- 0008_user_auth_and_mail.sql：普通用户登录/找回密码 + 邮件配置

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

CREATE TABLE IF NOT EXISTS app_settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
