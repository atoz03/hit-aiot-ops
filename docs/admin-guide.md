# 管理员指南（部署与运营）

## 1. 组件清单

- 控制器：`controller/`（建议部署在中转机/控制节点）
- 节点 Agent：`node-agent/`（部署在所有 GPU 计算节点，建议以 root 运行）
- 数据库：PostgreSQL（建议独立实例/主备）
- Hook：`tools/check_quota.sh`（部署到各节点 `/opt/gpu-cluster/` 并写入用户 `.bashrc`）

## 2. 关键配置

配置文件示例：`config/controller.yaml`

重点字段：
- `database_dsn`：PostgreSQL 连接串
- `agent_token`：保护 `/api/metrics`，防止伪造上报
- `admin_token`：保护管理员接口与充值
- `cpu_price_per_core_minute`：CPU 单价（核分钟）
- `enable_cpu_control`：是否启用 CPU 限流动作
- `cpu_limit_percent_limited / cpu_limit_percent_blocked`：CPU 限流百分比（0 表示解除限制）
- `kill_grace_period_seconds`：欠费 kill 宽限期（秒）

## 3. CPU 控制兼容性（重点）

Agent 的 CPU 限流实现为三段兜底：
1) systemd：`systemctl set-property --runtime user-<uid>.slice CPUQuota=...`
2) cgroup v2：写 `cpu.max`（并尝试把该用户进程写入 `cgroup.procs`）
3) cgroup v1：写 `cpu.cfs_period_us/cpu.cfs_quota_us`（并把该用户进程 PID 写入 `tasks`）

要求：
- Agent 需要以 root 运行（写 cgroup 与迁移进程需要权限）
- 机器上需要至少具备 systemd 或 cgroup cpu controller（v1/v2 任一）

## 4. 计费幂等（防重复扣费）

Agent 每次上报携带 `report_id`（随机 128bit），控制器写入 `metric_reports` 表做幂等：
- 若同一个 `report_id` 重复上报，控制器直接忽略，不落库、不扣费、不下发动作

数据库迁移：`database/migrations/0003_metric_reports.sql`

## 5. 建议上线步骤（试点到全量）

1) 中转机部署控制器与数据库（先 `dry_run=true` 观察 1-3 天）
2) 选择 2-3 台节点部署 Agent + Hook，邀请少量用户试用
3) 根据数据校验与用户反馈调整单价/阈值
4) 全量部署到 24 台节点

## 6. 常用脚本

- `scripts/deploy_agent.sh`：批量部署 Agent（示例）
- `scripts/deploy_hook.sh`：部署 Hook 到所有用户（示例）
- `scripts/check_status.sh`：接口自检（示例）

