# 上线运行手册（一步步）

本文面向管理员/运维，目标是把系统按“先试点、再全量”的方式安全上线。

范围说明（你可以把它当成“可上线交付物”）：
- 后端：Agent 上报 → 幂等去重 → 落库 → GPU+CPU 计费 → 余额状态 → 限制/终止 → 可查询/可观测
- 前端：Vue3 管理端（登录界面 + 管理页面），由控制器直接托管（不需要单独部署前端服务）
- 真实 GPU 分配/通知/支付/高可用/完整监控栈属于后续增强项，但不影响当前“能上线、能运营”

## 0. 上线前你需要准备什么（不要跳步）

1) 一台控制节点（中转机）用于部署控制器（建议内网 IP）
2) 一个 PostgreSQL（同机或独立实例均可，建议独立实例/主备）
3) 所有计算节点具备：
- NVIDIA 驱动（有 GPU 的节点才需要），并可执行 `nvidia-smi`
- 具备以下之一（用于 CPU 限流）：
  - systemd（推荐）
  - 或 cgroup v2
  - 或 cgroup v1 的 cpu controller

提示：在每台计算节点上运行 `scripts/node_prereq_check.sh` 做前置检查（不修改系统）。

## 0.1 你要在哪台机器“构建”？

为了避免“scp 依赖 SSH 免密/密钥”的不确定性，推荐两种方式（二选一）：

方式 A（推荐，最稳）：在控制节点本机构建并部署
- 你需要在控制节点安装：Go、Node.js、pnpm、git、构建工具
- 好处：不需要从外部机器拷贝文件，不依赖 SSH key/scp

方式 B（可用）：在独立构建机构建，然后把产物复制到控制节点
- 你需要确保“能登录控制节点”（密码或密钥均可）
- 复制方式你自己选：`scp` / `rsync` / 共享盘 / 手工上传

本 runbook 默认按方式 A 写，方式 B 在每一步都给出等价替代。

## 0.2 安装 Go（用于构建 controller/node-agent）

目标版本：Go 1.21+（建议 1.21 或 1.22）

在要构建的机器上执行检查：

```bash
go version || true
```

若没有 Go，请按系统选择一种安装方式（不要混装）：

Ubuntu/Debian（推荐官方 tarball，版本可控）：
```bash
sudo apt-get update
sudo apt-get install -y curl ca-certificates tar
curl -fsSLo /tmp/go.tar.gz https://go.dev/dl/go1.22.5.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf /tmp/go.tar.gz
echo 'export PATH=/usr/local/go/bin:$PATH' | sudo tee /etc/profile.d/go.sh
source /etc/profile.d/go.sh
go version
```

Rocky/RHEL（同样建议官方 tarball）：
```bash
sudo dnf install -y curl ca-certificates tar
curl -fsSLo /tmp/go.tar.gz https://go.dev/dl/go1.22.5.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf /tmp/go.tar.gz
echo 'export PATH=/usr/local/go/bin:$PATH' | sudo tee /etc/profile.d/go.sh
source /etc/profile.d/go.sh
go version
```

注意：
- 若你是 arm64 机器，请把下载包替换成 `linux-arm64`。

## 0.3 安装 Node.js + pnpm（用于构建前端）

目标：Node.js 18+（建议 20+），并使用 pnpm（本仓库使用 corepack/pnpm）。

检查：
```bash
node -v || true
corepack --version || true
pnpm -v || true
```

安装 Node.js（Ubuntu/Debian，NodeSource 方式）：
```bash
sudo apt-get update
sudo apt-get install -y curl ca-certificates
curl -fsSL https://deb.nodesource.com/setup_22.x | sudo -E bash -
sudo apt-get install -y nodejs
node -v
corepack enable
corepack prepare pnpm@10.28.0 --activate
pnpm -v
```

安装 Node.js（Rocky/RHEL，NodeSource 方式）：
```bash
sudo dnf install -y curl ca-certificates
curl -fsSL https://rpm.nodesource.com/setup_22.x | sudo bash -
sudo dnf install -y nodejs
node -v
corepack enable
corepack prepare pnpm@10.28.0 --activate
pnpm -v
```

## 0.4 获取代码（控制节点本机）

如果你的控制节点能访问 git 仓库：
```bash
git clone <你的仓库地址> hit-aiot-ops
cd hit-aiot-ops
```

如果控制节点不能访问 git：
- 你需要在能访问 git 的机器打包代码并传到控制节点（共享盘/手工上传/rsync/scp）
- 传完后在控制节点解压并进入目录

## 1. 生成并保存 Token（必须）

控制器有两类 token：
- `agent_token`：Agent 上报鉴权（`X-Agent-Token`）
- `admin_token`：管理员接口鉴权（`Authorization: Bearer ...`）

建议在控制节点生成：

```bash
openssl rand -hex 32
openssl rand -hex 32
```

把这两个值写入控制器配置文件（见下一节），并妥善保存。

## 2. 配置 PostgreSQL

你需要一个 DSN，例如：

```text
postgres://gpuops:gpuops@127.0.0.1:5432/gpuops?sslmode=disable
```

建议做两件事：
1) 确认数据库可从控制器机器访问
2) 规划数据保留策略（`usage_records` 会持续增长）

## 3. 部署控制器（controller）

### 3.1 构建 Linux 二进制（控制器+Agent）

在任意有 Go 的机器上执行：

```bash
export GOPROXY=https://goproxy.cn,direct
export GOSUMDB=off

scripts/build_linux.sh
```

输出：
- `bin/controller`
- `bin/node-agent`

### 3.2 构建前端（Vue3，必须）

控制器会自动托管 `web/dist/`。请在构建机执行：

```bash
cd web
pnpm install
pnpm build
```

如果你是“方式 A（控制节点本机构建）”：构建产物就在本机 `web/dist/`。

如果你是“方式 B（独立构建机构建）”：你需要把 `web/dist/` 复制到控制节点某个目录，例如：
- `/opt/gpu-controller/web/dist`

复制前先确认你能登录控制节点（不要假设有 SSH key）：
```bash
ssh <user>@<controller-host> 'echo ok'
```

若你没有 SSH key，需要先配置（示例）：
```bash
ssh-keygen -t ed25519 -C "gpuops-deploy"
ssh-copy-id <user>@<controller-host>
```

确认可登录后再复制（示例，二选一）：
```bash
scp -r web/dist <user>@<controller-host>:/opt/gpu-controller/web/dist
```
或：
```bash
rsync -av web/dist/ <user>@<controller-host>:/opt/gpu-controller/web/dist/
```

### 3.3 准备控制器配置

拷贝 `config/controller.yaml` 为生产配置（建议放在控制节点）：

```bash
cp config/controller.yaml /opt/gpu-controller/controller.yaml
```

修改至少以下字段：
- `listen_addr`：建议绑定内网地址，例如 `10.0.0.10:8000`
- `database_dsn`：你的 PostgreSQL DSN
- `agent_token`、`admin_token`：替换为强随机值
- `auth_secret`：替换为强随机值（用于 Web 登录会话签名）
- `dry_run`：建议先设置 `true` 试运行 1-3 天
- `web_dir`：指向前端构建产物目录（例如 `/opt/gpu-controller/web/dist`）

### 3.4 安装并启动 systemd 服务

在控制节点：

```bash
sudo mkdir -p /opt/gpu-controller
sudo cp bin/controller /opt/gpu-controller/controller
sudo chmod +x /opt/gpu-controller/controller
sudo cp /opt/gpu-controller/controller.yaml /opt/gpu-controller/controller.yaml

sudo cp systemd/gpu-controller.service /etc/systemd/system/gpu-controller.service
sudo systemctl daemon-reload
sudo systemctl enable gpu-controller
sudo systemctl restart gpu-controller
```

### 3.5 验证控制器

```bash
curl -fsS http://<controller-ip>:8000/healthz
curl -fsS http://<controller-ip>:8000/metrics | head -n 20
```

打开 Web 管理页：
- `http://<controller-ip>:8000/`

节点状态（管理员接口）：
- `GET /api/admin/nodes`

### 3.6 初始化管理员账号（必须，否则无法登录）

首次上线需要创建管理员账号（用于 Web 登录）。该步骤只允许执行一次。

使用 `admin_token` 调用 bootstrap 接口创建账号：

```bash
curl -fsS -H "Authorization: Bearer <admin_token>" \
  -H "Content-Type: application/json" \
  -X POST http://<controller-ip>:8000/api/admin/bootstrap \
  -d '{"username":"admin","password":"ChangeMe_123456"}'
```

然后访问 Web 登录页：
- `http://<controller-ip>:8000/login`

## 4. 部署节点 Agent（node-agent）

### 4.1 在每台计算节点安装二进制（不要假设 scp 免密）

你必须先确认能登录节点（密码或密钥均可）：

```bash
ssh root@node01 'echo ok'
```

确认能登录后，再选择一种复制方式（示例）：
```bash
scp bin/node-agent root@node01:/usr/local/bin/node-agent
ssh root@node01 "chmod +x /usr/local/bin/node-agent"
```

如果你没有 SSH key，请先用 `ssh-copy-id` 或让管理员开通访问（不要继续往下做）。

### 4.2 配置并启动 systemd 服务

复制服务文件并改 token（推荐通过环境变量注入）：

同样先确认能登录，再复制：

```bash
scp systemd/gpu-node-agent.service root@node01:/etc/systemd/system/gpu-node-agent.service
ssh root@node01 "systemctl daemon-reload"
```

最简单做法：编辑 `/etc/systemd/system/gpu-node-agent.service`，设置：
- `NODE_ID=node01`
- `CONTROLLER_URL=http://<controller-ip>:8000`
- `AGENT_TOKEN=<你的 agent_token>`

启动：

```bash
ssh root@node01 "systemctl enable gpu-node-agent && systemctl restart gpu-node-agent"
```

### 4.3 验证 Agent 上报

在控制节点查看：
- `/metrics` 中 `gpuops_controller_reports_total` 是否增长
- Web 管理页中查看用户余额/使用记录是否产生

## 5. 部署 Bash Hook（拦截“新 GPU 任务”）

在每台节点：

```bash
sudo mkdir -p /opt/gpu-cluster
sudo cp tools/check_quota.sh /opt/gpu-cluster/check_quota.sh
sudo chmod +x /opt/gpu-cluster/check_quota.sh
```

把以下内容追加到用户 `~/.bashrc`（建议脚本批量写入，见 `scripts/deploy_hook.sh`）：

```bash
source /opt/gpu-cluster/check_quota.sh
```

注意：
- Hook 主要用于阻止“余额不足时启动新的 GPU 任务”（尽量不误伤 CPU 日常使用）。
- CPU 限流由 Agent 执行，不依赖 Hook。

## 6. 试点上线建议（强烈推荐）

1) 控制器先 `dry_run=true`（只记账不扣费），跑 1-3 天
2) 选择 2-3 台节点、1-2 个用户试用
3) 校验：
- `usage_records` 中 cost 是否符合预期
- `CPU_CORE` 单价是否合理
- limited/blocked 时 CPUQuota 是否生效
4) 确认无误后切 `dry_run=false` 开始扣费

## 7. 常用运营操作（管理员）

### 7.1 设置单价

GPU 单价按字符串包含匹配；CPU 单价使用特殊模型名 `CPU_CORE`（核分钟）。

```bash
curl -fsS -H "Authorization: Bearer <admin_token>" \
  -X POST http://<controller-ip>:8000/api/admin/prices \
  -d '{"gpu_model":"CPU_CORE","price_per_minute":0.02}'
```

### 7.2 充值

```bash
curl -fsS -H "Authorization: Bearer <admin_token>" \
  -H "Content-Type: application/json" \
  -X POST http://<controller-ip>:8000/api/users/alice/recharge \
  -d '{"amount":100,"method":"admin"}'
```

### 7.3 查询使用记录

用户自查：
- `GET /api/users/:username/usage`

管理员：
- `GET /api/admin/usage?username=alice&limit=200`

导出 CSV（便于对账/计费）：
- `GET /api/admin/usage/export.csv?username=alice&from=2026-02-01&to=2026-02-05`

## 8. 常见问题排查

1) Agent 上报 401
- 检查 `AGENT_TOKEN` 是否与控制器配置一致

2) CPU 限流不生效
- 确认 Agent 以 root 运行
- 节点运行 `scripts/node_prereq_check.sh` 看 systemd/cgroup 情况
- systemd 场景可用 `systemctl show user-<uid>.slice -p CPUQuota` 查看是否设置成功

3) 没有 GPU 记录
- 节点是否存在 `nvidia-smi`
- `nvidia-smi --query-compute-apps=pid,gpu_name --format=csv,noheader` 是否有输出
