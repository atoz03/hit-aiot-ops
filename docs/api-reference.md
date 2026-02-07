# API 参考

本文面向开发/运维，描述控制器对外 API（用于 Agent、Hook、管理员脚本）。

## 鉴权约定

- Agent 上报：`X-Agent-Token: <token>`
- 管理员接口：`Authorization: Bearer <adminToken>`
- Web 登录：`POST /api/auth/login` 后由控制器下发 HttpOnly cookie（同站点请求自动携带）
- 用户积分/余额查询：默认不鉴权（用于 Bash Hook）；部署到生产前建议放到内网并增加网关/ACL

CSRF 说明（Web 登录场景）：
- 通过 cookie 会话访问管理员接口时，非 GET 请求需要携带 `X-CSRF-Token`（从 `GET /api/auth/me` 返回的 `csrf_token` 获取）

## Web 登录与初始化

### `POST /api/admin/bootstrap`（管理员，仅首次）

用途：首次上线初始化管理员账号（用于 Web 登录）。

限制：
- 只允许执行一次（当 `admin_accounts` 表为空时才允许）
- 必须使用 `Authorization: Bearer <adminToken>` 调用（禁止用 session 自举）

请求：
```json
{"username":"admin","password":"ChangeMe_123456"}
```

### `GET /api/auth/me`

用途：查询当前 cookie 会话是否已登录；并获取 CSRF token。

返回（已登录示例）：
```json
{"authenticated":true,"username":"admin","role":"admin","expires_at":"2026-02-05T16:00:00Z","csrf_token":"..."}
```

### `POST /api/auth/login`

请求：
```json
{"username":"admin","password":"..."}
```

返回：`{"ok":true}`（并在响应头下发 HttpOnly cookie）。

### `POST /api/auth/logout`

返回：`{"ok":true}`（并清除 cookie）。

## 健康检查

### `GET /healthz`

返回 `{"ok":true}`。

## 监控指标

### `GET /metrics`

返回控制器内置的最小监控指标（Prometheus 文本格式子集），用于快速接入 Prometheus 抓取与上线自检。

## Agent 上报

### `POST /api/metrics`

幂等说明：
- `report_id` 必填且全局唯一；控制器用其做去重，避免 Agent 重试导致重复扣费

请求体（示例）：

```json
{
  "node_id": "60000",
  "timestamp": "2026-02-05T16:00:00Z",
  "report_id": "2f6c7b3b3c3b4a8b8f1c5c3c1b2a9d10",
  "interval_seconds": 60,
  "users": [
    {
      "username": "alice",
      "pid": 12345,
      "cpu_percent": 120.5,
      "memory_mb": 2048,
      "gpu_usage": [
        {"gpu_id": 0, "gpu_model": "NVIDIA A100-SXM4-80GB", "gpu_bus_id":"00000000:3B:00.0", "memory_mb": 4096}
      ]
    }
  ]
}
```

响应体：

```json
{
  "actions": [
    {"type":"notify","username":"alice","message":"余额预警：当前余额 80.00 元，请及时充值"},
    {"type":"set_cpu_quota","username":"alice","cpu_quota_percent":50,"reason":"余额不足，限制 CPU 使用"}
  ]
}
```

说明：
- `node_id` 约定为**机器编号**（推荐直接使用 SSH 端口号，例如 `60000`），用于把“节点本地账号”映射到“计费账号”进行扣费与限制。
- 当存在节点账号绑定（见下文）时：控制器会把 `(node_id, local_username)` 映射到 `billing_username` 进行扣费；但下发动作（block/kill/cpu_quota）仍会针对本地用户名，保证 Agent 能生效。

## 用户接口

### `GET /api/users/:username/balance`

返回：

```json
{"username":"alice","balance":80.00,"status":"warning"}
```

### `GET /api/users/:username/usage`

参数：
- `limit`：返回条数（默认 200，最大 5000）

返回：

```json
{"records":[{"node_id":"60000","username":"alice","timestamp":"2026-02-05T16:00:00Z","cpu_percent":120.5,"memory_mb":2048,"gpu_usage":"[]","cost":0.6}]}
```

### `POST /api/users/:username/recharge`（管理员）

请求：

```json
{"amount": 100, "method":"admin"}
```

## 管理员接口

### `GET /api/admin/users`

### `POST /api/admin/prices`

请求：

```json
{"gpu_model":"RTX 3090","price_per_minute":0.2}
```

说明：
- CPU 计费使用特殊模型名 `CPU_CORE`（按核分钟：100% CPU ≈ 1 核）。
- `set_cpu_quota` 需要节点支持 systemd CPUQuota 或 cgroup（v2 或 v1 的 cpu controller），且 Agent 以 root 运行。

## 排队接口（可选）

### `POST /api/gpu/request`

请求：

```json
{"username":"alice","gpu_type":"rtx3090","count":2}
```

响应（当前实现为“只排队不分配”的可运行版本）：

```json
{"status":"queued","position":3,"estimated_minutes":30,"message":"当前无可用 GPU，已加入排队"}
```

### `GET /api/admin/gpu/queue`（管理员）

### `GET /api/admin/usage`（管理员）

参数：
- `username`：可选，按用户过滤
- `limit`：返回条数（默认 200，最大 5000）

### `GET /api/admin/usage/export.csv`（管理员）

参数：
- `username`：可选
- `from`：可选（RFC3339 或 YYYY-MM-DD）
- `to`：可选（RFC3339 或 YYYY-MM-DD）
- `limit`：可选（默认 20000，最大 200000）

返回：CSV 文件（列：timestamp,node_id,username,cpu_percent,memory_mb,cost,gpu_usage_json）。

### `GET /api/admin/nodes`（管理员）

参数：
- `limit`：可选（默认 200，最大 2000）

返回：节点上报状态（last_seen、gpu/cpu 进程数、当次上报成本等）。

## 用户注册 / 账号绑定 / 开号申请

### `POST /api/requests/bind`（用户自助：绑定登记，需审核）

用途：用户登记“我在某台机器(node_id)上的本地用户名(local_username)”，并关联到“计费账号(billing_username)”。

请求：
```json
{
  "billing_username":"alice",
  "items":[
    {"node_id":"60000","local_username":"alice"},
    {"node_id":"60005","local_username":"alice2"}
  ],
  "message":"可选备注"
}
```

返回：
```json
{"ok":true,"request_ids":[1,2]}
```

### `POST /api/requests/open`（用户自助：开号申请，需审核）

请求：
```json
{"billing_username":"alice","node_id":"60000","local_username":"alice","message":"可选备注"}
```

返回：
```json
{"ok":true,"request_id":3}
```

### `GET /api/requests?billing_username=...`（用户自助：查看申请记录）

参数：
- `billing_username`：必填
- `limit`：可选（默认 200，最大 5000）

返回：
```json
{"requests":[{"request_id":1,"request_type":"bind","billing_username":"alice","node_id":"60000","local_username":"alice","status":"pending"}]}
```

### `GET /api/admin/requests`（管理员：查看申请）

参数：
- `status`：可选（`pending/approved/rejected`）；为空表示全部
- `limit`：可选（默认 200，最大 5000）

### `POST /api/admin/requests/:id/approve`（管理员：通过）

说明：当申请类型为 `bind` 时，通过会自动写入 `user_node_accounts`，用于扣费映射与 SSH 登录校验。

### `POST /api/admin/requests/:id/reject`（管理员：拒绝）

## SSH 登录校验（节点侧拉取 allowlist）

### `GET /api/registry/nodes/:node_id/users.txt`

用途：返回该节点(node_id)已登记通过的本地用户名列表（每行一个），供节点侧 PAM/SSH 校验缓存定期同步。

### `GET /api/registry/resolve?node_id=...&local_username=...`

用途：查询某个本地用户名在指定节点上是否已绑定，并返回对应计费账号。
