# hit-aiot-ops（GPU 集群管理：轻量 Agent + 控制器）

本仓库实现 `doc/plan.md` 中“阶段 1：核心组件（节点 Agent + 控制器 + 计费/配额）”的可运行版本：

- 节点侧：`node-agent/`（Golang）每分钟采集 GPU 计算进程并上报
- 控制器：`controller/`（Golang + Gin + PostgreSQL）接收上报、落库、计费（GPU + CPU）、下发限制动作（含 CPU 限流）
- 用户侧：`tools/check_quota.sh`（Bash Hook）在用户启动疑似 GPU 任务前检查余额状态

## 快速开始（本机开发）

### 0) 依赖下载（网络受限场景）

如果你所在网络无法访问 `proxy.golang.org` / `golang.org`，建议临时使用国内 Go Proxy：

```bash
export GOPROXY=https://goproxy.cn,direct
export GOSUMDB=off
```

### 1) 启动 PostgreSQL（可选：使用 docker compose）

```bash
cd /Volumes/disk/hit-aiot-ops
docker compose up -d
```

默认会创建数据库 `gpuops`，用户名/密码均为 `gpuops`，端口 `5432`。

### 2) 启动控制器

```bash
cd controller
go test ./...
go run . --config ../config/controller.yaml
```

健康检查：

```bash
curl -s http://127.0.0.1:8000/healthz
```

### 3) 启动节点 Agent（同机模拟）

```bash
cd node-agent
go test ./...
NODE_ID=dev-node01 CONTROLLER_URL=http://127.0.0.1:8000 AGENT_TOKEN=dev-agent-token go run .
```

说明：
- 没有 NVIDIA 驱动或没有 `nvidia-smi` 时，Agent 会优雅降级为只心跳上报（不报 GPU 进程）。

## API 速查

- `POST /api/metrics`（Agent 上报；需要 `X-Agent-Token`）
- `GET /api/users/:username/balance`（余额查询；用于 Hook）
- `POST /api/users/:username/recharge`（充值；需要 `Authorization: Bearer <adminToken>`）
- `GET /api/admin/users`（管理员查询；需要管理员 token）
- `POST /api/admin/prices`（设置 GPU 单价；需要管理员 token）

更完整的字段说明见：`docs/api-reference.md`。

## 目录结构

与 `doc/plan.md` 保持一致（核心实现已落地）：

```
hit-aiot-ops/
├── node-agent/
├── controller/
├── database/
├── tools/
├── scripts/
├── config/
├── systemd/
└── docs/
```
