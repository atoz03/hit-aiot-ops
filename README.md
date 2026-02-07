# hit-aiot-ops（GPU 集群管理：轻量 Agent + 控制器）

本仓库实现 `docs/plan.md` 中“阶段 1：核心组件（节点 Agent + 控制器 + 计费/配额）”的可运行版本：

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

补充说明：
- 建议优先使用 `docker compose ...`（不加 `sudo`）。如果必须使用 `sudo`，请确保全程一致（`sudo docker ...` / `sudo docker compose ...`），避免因为 Docker 上下文/环境差异导致行为不一致。
- 如果你所在网络无法直接访问 Docker Hub，可通过环境变量替换镜像来源（例如公司内网镜像仓库/镜像加速器）：

```bash
export POSTGRES_IMAGE="postgres:18.1"       # 也可以改成你的镜像仓库地址
docker compose up -d --pull missing        # 可选：always / missing / never
```

常见排障（遇到 “Pulled 但 No such image” 这类现象时）：

```bash
docker compose pull postgres
docker image ls postgres
docker image ls | grep -E "postgres\\s+18\\.1|docker\\.io/library/postgres:18\\.1" || true
docker image inspect postgres:18.1 >/dev/null
docker context show
docker version
```

如果 `docker compose pull` 或 `docker pull postgres:18.1` 本身失败，请优先检查：
- 代理/镜像加速器/私有仓库配置（`/etc/docker/daemon.json` 的 `registry-mirrors` 等）
- 磁盘空间（`df -h`、`docker system df`）
- Docker 守护进程状态（如 `systemctl status docker` / `systemctl restart docker`）

### 2) 启动控制器

```bash
cd controller
go test ./...
go run . --config ../config/controller.yaml
```

（可选但推荐）构建前端，让控制器托管完整 Web 管理端：

```bash
cd web
pnpm install
pnpm build
```

注意：控制器启动时才会探测 `web/dist/`，因此建议先构建前端再启动控制器；如果你是启动后才构建，请重启控制器。

健康检查：

```bash
curl -s http://127.0.0.1:8000/healthz
```

监控指标：
- `http://127.0.0.1:8000/metrics`

Web 管理页：
- 浏览器打开 `http://127.0.0.1:8000/`
- 需要先创建管理员账号并在 `/login` 登录（详见 `docs/runbook.md`）

开发环境快速初始化管理员账号（只需做一次）：

```bash
curl -fsS -H "Authorization: Bearer dev-admin-token" \
  -H "Content-Type: application/json" \
  -X POST http://127.0.0.1:8000/api/admin/bootstrap \
  -d '{"username":"admin","password":"ChangeMe_123456"}'
```

然后访问：
- `http://127.0.0.1:8000/login`

### 3) 启动节点 Agent（同机模拟）

```bash
cd node-agent
go test ./...
NODE_ID=dev-node01 CONTROLLER_URL=http://127.0.0.1:8000 AGENT_TOKEN=dev-agent-token go run .
```

说明：
- 没有 NVIDIA 驱动或没有 `nvidia-smi` 时，Agent 仍会心跳上报与 CPU 计费/控制（但不会上报 GPU 进程）。
- CPU 控制优先使用 `systemd CPUQuota`；否则按 `cgroup v2` 再到 `cgroup v1(cpu.cfs_*)` 兜底。
- 为防止网络重试导致重复扣费，Agent 每次上报携带 `report_id`，控制器做幂等去重。

## API 速查

- `POST /api/metrics`（Agent 上报；需要 `X-Agent-Token`）
- `GET /api/users/:username/balance`（余额查询；用于 Hook）
- `GET /api/users/:username/usage`（用户使用记录）
- `POST /api/users/:username/recharge`（充值；需要 `Authorization: Bearer <adminToken>`）
- `GET /api/admin/users`（管理员查询；需要管理员 token）
- `POST /api/admin/prices`（设置 GPU 单价；需要管理员 token）
- `GET /api/admin/usage`（管理员查询使用记录）
- `GET /api/admin/gpu/queue`（管理员查看排队）

更完整的字段说明见：`docs/api-reference.md`。

## 文档

- `docs/plan.md`：总体方案与实现对照
- `docs/api-reference.md`：API 参考
- `docs/user-guide.md`：用户指南
- `docs/admin-guide.md`：管理员指南
- `docs/go-live-checklist.md`：上线检查清单
- `docs/runbook.md`：一步步上线运行手册

## 测试说明

本仓库为多 Go module（`controller/`、`node-agent/`），请在仓库根目录按如下方式运行测试：

```bash
go test ./controller/... ./node-agent/...
```

CI（GitHub Actions）：
- 已配置在每次 `push` / `pull_request` 自动运行同一套测试：`.github/workflows/go-test.yml`

## 提交前自动测试（Git hooks）

如果你希望每次 `git commit` 都自动跑测试（失败则阻止提交），先在仓库根目录执行一次：

```bash
bash "scripts/install-githooks.sh"
```

说明：
- 安装后，每次提交会自动执行：`go test ./controller/... ./node-agent/...`
- 如需临时跳过（不推荐常用）：`git commit --no-verify`

## 构建与部署（生产）

- 构建 Linux 二进制：`scripts/build_linux.sh`（输出到 `bin/`，可配 `GOARCH=arm64`）
- 部署控制器：`scripts/deploy_controller.sh`（示例）
- 部署 Agent：`scripts/deploy_agent.sh`（示例）
- 部署 Hook：`scripts/deploy_hook.sh`（示例）

## 前端开发（Vue3）

前端位于 `web/`，使用 `pnpm` 管理依赖：

```bash
cd web
pnpm install
pnpm dev
```

构建产物输出到 `web/dist/`，控制器会自动托管（访问 `http://<controller>/`）。

## 目录结构

与 `docs/plan.md` 保持一致（核心实现已落地）：

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
