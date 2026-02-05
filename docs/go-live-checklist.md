# 上线检查清单（建议按试点→全量）

本清单用于判断“是否达到可上线（核心闭环）”标准。

推荐配合：`docs/runbook.md`。

## 1. 环境与依赖

- PostgreSQL 可用（建议独立实例/主备）
- 控制器与节点可互通（HTTP），并已设置 `agent_token`
- 计算节点具备以下之一：
  - systemd 可用（推荐）
  - 或 cgroup v2（`/sys/fs/cgroup/.../cpu.max`）
  - 或 cgroup v1 cpu controller（`cpu.cfs_*` + `tasks`）
 - 节点建议执行：`scripts/node_prereq_check.sh`（不修改系统）

## 2. 数据库

- 已执行迁移（控制器启动会自动执行）：
  - `0001_init.sql`
  - `0002_seed_prices.sql`（含 `CPU_CORE`）
  - `0003_metric_reports.sql`（幂等去重）
- 确认价格表包含集群 GPU 型号关键词与 `CPU_CORE`

## 3. 安全与隔离（最低要求）

- 控制器仅对内网开放，或经网关 ACL 保护
- `agent_token` 与 `admin_token` 已更换为强随机值，并安全保存
- 管理员接口不要暴露到公网
- 余额查询接口默认不鉴权（为了 Hook 低侵入），建议仅内网可达或通过网关保护

## 4. 功能闭环验证（上线前必做）

1) 控制器健康检查
- `GET /healthz` 返回 `{"ok":true}`
- `GET /metrics` 有输出（便于上线后观测）
- `GET /api/admin/nodes` 可看到节点上报（上线后用于判断离线节点）

2) Agent 上报与幂等
- 观察控制器日志与数据库 `metric_reports` 行数增长
- 人为重放同一条上报（相同 `report_id`）不应重复扣费/落库

3) GPU 计费
- 运行一个 GPU 进程（`nvidia-smi --query-compute-apps` 可见）
- 控制器应记录 `usage_records`，并扣减 `users.balance`

4) CPU 计费
- 运行明显占用 CPU 的进程（例如多线程压测）
- 控制器应记录 CPU-only usage 并扣减余额

5) 限制动作
- 将用户余额调到 `limited`：应阻止新 GPU 任务（Hook 生效）并下发 `set_cpu_quota`
- 将用户余额调到 `blocked`：超过宽限期应下发 `kill_process`，并强限制 CPU
 - 验证 CPUQuota：systemd 场景可检查 `systemctl show user-<uid>.slice -p CPUQuota`

## 5. 运营参数建议

- 初期建议 `dry_run=true`（记录但不扣费）跑 1-3 天校验数据后再切 `false`
- `warning_threshold / limited_threshold / kill_grace_period_seconds` 建议结合用户习惯调整
- 初期建议对部分用户先试点，确认 CPU/GPU 单价匹配集群实际硬件与预期扣费
