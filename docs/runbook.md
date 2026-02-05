# 上线运行手册（一步步）

本文面向管理员/运维，目标是把系统按“先试点、再全量”的方式安全上线。

范围说明：
- 本仓库已实现核心闭环：Agent 上报 → 落库 → GPU+CPU 计费 → 余额状态 → 限制/终止 → 可查询/可观测。
- `doc/plan.md` 中的增强项（完整 Vue 管理端、真实 GPU 分配/排队、支付/通知、高可用、完整监控栈）不属于本手册的“上线必需条件”。

## 0. 上线前你需要准备什么

1) 一台控制节点（中转机）用于部署控制器（建议内网 IP）
2) 一个 PostgreSQL（同机或独立实例均可，建议独立实例/主备）
3) 所有计算节点具备：
- NVIDIA 驱动（有 GPU 的节点才需要），并可执行 `nvidia-smi`
- 具备以下之一（用于 CPU 限流）：
  - systemd（推荐）
  - 或 cgroup v2
  - 或 cgroup v1 的 cpu controller

提示：可以在计算节点上运行 `scripts/node_prereq_check.sh` 快速检查。

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

### 3.1 构建 Linux 二进制

在任意有 Go 的机器上执行：

```bash
export GOPROXY=https://goproxy.cn,direct
export GOSUMDB=off

scripts/build_linux.sh
```

输出：
- `bin/controller`
- `bin/node-agent`

### 3.2 准备控制器配置

拷贝 `config/controller.yaml` 为生产配置（建议放在控制节点）：

```bash
cp config/controller.yaml /opt/gpu-controller/controller.yaml
```

修改至少以下字段：
- `listen_addr`：建议绑定内网地址，例如 `10.0.0.10:8000`
- `database_dsn`：你的 PostgreSQL DSN
- `agent_token`、`admin_token`：替换为强随机值
- `dry_run`：建议先设置 `true` 试运行 1-3 天

### 3.3 安装并启动 systemd 服务

在控制节点：

```bash
mkdir -p /opt/gpu-controller
cp bin/controller /opt/gpu-controller/controller
chmod +x /opt/gpu-controller/controller
cp /opt/gpu-controller/controller.yaml /opt/gpu-controller/controller.yaml

cp systemd/gpu-controller.service /etc/systemd/system/gpu-controller.service
systemctl daemon-reload
systemctl enable gpu-controller
systemctl restart gpu-controller
```

### 3.4 验证控制器

```bash
curl -fsS http://<controller-ip>:8000/healthz
curl -fsS http://<controller-ip>:8000/metrics | head -n 20
```

打开 Web 管理页（最小可用）：
- `http://<controller-ip>:8000/`

节点状态（管理员接口）：
- `GET /api/admin/nodes`

## 4. 部署节点 Agent（node-agent）

### 4.1 在每台计算节点安装二进制

把 `bin/node-agent` 复制到节点：

```bash
scp bin/node-agent root@node01:/usr/local/bin/node-agent
ssh root@node01 "chmod +x /usr/local/bin/node-agent"
```

### 4.2 配置并启动 systemd 服务

复制服务文件并改 token（推荐通过环境变量注入）：

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
