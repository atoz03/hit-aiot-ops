# API 参考

本文面向开发/运维，描述控制器对外 API（用于 Agent、Hook、管理员脚本）。

## 鉴权约定

- Agent 上报：`X-Agent-Token: <token>`
- 管理员接口：`Authorization: Bearer <adminToken>`
- 用户余额查询：默认不鉴权（用于 Bash Hook）；部署到生产前建议放到内网并增加网关/ACL

## 健康检查

### `GET /healthz`

返回 `{"ok":true}`。

## Agent 上报

### `POST /api/metrics`

请求体（示例）：

```json
{
  "node_id": "node01",
  "timestamp": "2026-02-05T16:00:00Z",
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

## 用户接口

### `GET /api/users/:username/balance`

返回：

```json
{"username":"alice","balance":80.00,"status":"warning"}
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
