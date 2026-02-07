# 管理员指南（部署与运营）

建议先阅读：`docs/runbook.md`（一步步上线运行手册）。

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
- `auth_secret`：Web 登录会话签名密钥（启用会话时必填，建议强随机）
- `session_hours`：Web 登录会话有效期（小时；0 表示禁用会话，仅保留 Bearer admin_token）
- `cpu_price_per_core_minute`：CPU 单价（核分钟）
- `enable_cpu_control`：是否启用 CPU 限流动作
- `cpu_limit_percent_limited / cpu_limit_percent_blocked`：CPU 限流百分比（0 表示解除限制）
- `kill_grace_period_seconds`：欠费 kill 宽限期（秒）

## 2.1 Web 管理端登录（上线必做）

1) 用 `admin_token` 初始化管理员账号（只允许一次）：
- `POST /api/admin/bootstrap`

2) 浏览器访问：
- `/login` 登录
- `/` 进入管理页

说明：
- Web 管理端不需要粘贴 token；使用 HttpOnly cookie 会话。
- cookie 会话访问管理员接口时，非 GET 需要 `X-CSRF-Token`（前端已自动处理；脚本调用见 `docs/api-reference.md`）。

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

- `scripts/build_linux.sh`：构建 Linux 二进制（输出到 `bin/`）
- `scripts/deploy_agent.sh`：批量部署 Agent（示例）
- `scripts/deploy_controller.sh`：部署控制器（示例）
- `scripts/deploy_hook.sh`：部署 Hook 到所有用户（示例）
- `scripts/check_status.sh`：接口自检（示例）
- `scripts/node_prereq_check.sh`：计算节点前置检查（systemd/cgroup/GPU）

## 7. 上线后常用接口

- 节点在线状态：`GET /api/admin/nodes`
- 使用记录查询：`GET /api/admin/usage`
- 使用记录导出：`GET /api/admin/usage/export.csv`

## 7.1 用户注册/开号审核（新增）

用户自助在 Web「用户注册」页面提交：
- 账号绑定登记（bind）：把 `(node_id, local_username)` 绑定到 `billing_username`，用于扣费映射与 SSH 校验
- 开号申请（open）：仅用于记录与流程（不自动创建系统账号）

管理员处理方式（二选一）：
1) Web 管理端：`管理功能 -> 注册审核`
2) API：
   - 查看：`GET /api/admin/requests?status=pending`
   - 通过：`POST /api/admin/requests/:id/approve`
   - 拒绝：`POST /api/admin/requests/:id/reject`

## 8. 前端构建（Vue3）

前端在 `web/`，使用 `pnpm`：

```bash
cd web
pnpm install
pnpm build
```

构建产物在 `web/dist/`，可由控制器托管（配置 `web_dir`）。

## 9. 计算节点：未登记禁止 SSH 登录（可选，强管控）

本仓库示例脚本 `scripts/deploy_agent.sh` 支持可选安装 SSH 登录拦截：
- 规则：若本地用户名未在控制器登记通过（`user_node_accounts`），则禁止 SSH 登录
- 例外：可配置排除用户（默认 `root baojh xqt`）

推荐部署方式：

1) 先确保节点 Agent 上报的 `NODE_ID` 是机器编号（推荐用端口号，例如 `60000`）
2) 启用 SSH Guard 并按“机器编号:IP”方式传入节点列表：

```bash
export CONTROLLER_URL="http://controller:8000"
export AGENT_TOKEN="dev-agent-token"
export NODES="60000:192.168.1.104 60001:192.168.1.220"
export ENABLE_SSH_GUARD=1
export SSH_GUARD_EXCLUDE_USERS="root baojh xqt"
export SSH_GUARD_FAIL_OPEN=1   # 控制器不可达时是否放行（1=放行；0=拒绝）
bash scripts/deploy_agent.sh
```

注意：
- 节点需要 `curl`（同步 allowlist）；且系统启用 PAM（`/etc/pam.d/sshd` 存在，且支持 `pam_exec.so`）。
- 这是高风险变更，建议先在 1-2 台节点试点验证再全量推开。
