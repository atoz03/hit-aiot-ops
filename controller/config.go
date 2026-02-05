package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config 为控制器配置。
// 说明：字段与 config/controller.yaml 对应，便于运维直接修改。
type Config struct {
	ListenAddr string `yaml:"listen_addr"`

	DatabaseDSN string `yaml:"database_dsn"`

	AgentToken string `yaml:"agent_token"`
	AdminToken string `yaml:"admin_token"`
	// AuthSecret 用于签名 Web 登录会话（cookie）。建议使用强随机值。
	// 若为空，控制器会回退使用 admin_token（不推荐，仅为兼容）。
	AuthSecret string `yaml:"auth_secret"`

	WarningThreshold float64 `yaml:"warning_threshold"`
	LimitedThreshold float64 `yaml:"limited_threshold"`

	CPUPricePerCoreMinute float64 `yaml:"cpu_price_per_core_minute"`
	SampleIntervalSeconds int     `yaml:"sample_interval_seconds"`

	EnableCPUControl       bool    `yaml:"enable_cpu_control"`
	CPULimitPercentLimited float64 `yaml:"cpu_limit_percent_limited"`
	CPULimitPercentBlocked float64 `yaml:"cpu_limit_percent_blocked"`

	KillGracePeriodSeconds int `yaml:"kill_grace_period_seconds"`

	DryRun bool `yaml:"dry_run"`

	DefaultBalance        float64 `yaml:"default_balance"`
	DefaultPricePerMinute float64 `yaml:"default_price_per_minute"`

	// Web 登录会话配置
	SessionHours int  `yaml:"session_hours"`
	CookieSecure bool `yaml:"cookie_secure"`

	MigrationDir string `yaml:"migration_dir"`
	WebDir       string `yaml:"web_dir"`
}

func (c *Config) Validate() error {
	if c.ListenAddr == "" {
		return errors.New("listen_addr 不能为空")
	}
	if c.DatabaseDSN == "" {
		return errors.New("database_dsn 不能为空")
	}
	if c.WarningThreshold <= 0 {
		return errors.New("warning_threshold 必须为正数")
	}
	if c.LimitedThreshold < 0 || c.LimitedThreshold >= c.WarningThreshold {
		return errors.New("limited_threshold 必须在 [0, warning_threshold) 范围内")
	}
	if c.CPUPricePerCoreMinute < 0 {
		return errors.New("cpu_price_per_core_minute 不能为负数")
	}
	if c.SampleIntervalSeconds <= 0 || c.SampleIntervalSeconds > 600 {
		return errors.New("sample_interval_seconds 必须在 (0, 600] 范围内")
	}
	if c.CPULimitPercentLimited < 1 || c.CPULimitPercentLimited > 100 {
		return errors.New("cpu_limit_percent_limited 必须在 [1, 100] 范围内")
	}
	if c.CPULimitPercentBlocked < 1 || c.CPULimitPercentBlocked > 100 {
		return errors.New("cpu_limit_percent_blocked 必须在 [1, 100] 范围内")
	}
	if c.KillGracePeriodSeconds < 0 {
		return errors.New("kill_grace_period_seconds 不能为负数")
	}
	if c.DefaultBalance < 0 {
		return errors.New("default_balance 不能为负数")
	}
	if c.DefaultPricePerMinute < 0 {
		return errors.New("default_price_per_minute 不能为负数")
	}
	if c.AgentToken == "" {
		return errors.New("agent_token 不能为空（用于保护 /api/metrics）")
	}
	if c.AdminToken == "" {
		return errors.New("admin_token 不能为空（用于保护 /api/admin/* 与充值/单价设置）")
	}
	if c.SessionHours < 0 || c.SessionHours > 720 {
		return errors.New("session_hours 必须在 [0, 720]（0 表示禁用会话）")
	}
	return nil
}

type cliArgs struct {
	configPath string
}

func parseArgs() cliArgs {
	var a cliArgs
	flag.StringVar(&a.configPath, "config", "", "配置文件路径（yaml）")
	flag.Parse()
	return a
}

func defaultConfigPath() (string, error) {
	if p := os.Getenv("CONTROLLER_CONFIG"); p != "" {
		if fileExists(p) {
			return p, nil
		}
		return "", fmt.Errorf("环境变量 CONTROLLER_CONFIG 指向的文件不存在：%s", p)
	}

	candidates := []string{
		filepath.FromSlash("../config/controller.yaml"),
		filepath.FromSlash("config/controller.yaml"),
	}
	for _, p := range candidates {
		if fileExists(p) {
			return p, nil
		}
	}
	return "", errors.New("未找到默认配置文件：请使用 --config 或设置 CONTROLLER_CONFIG")
}

func loadConfig(path string) (Config, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
