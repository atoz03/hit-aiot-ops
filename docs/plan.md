# GPU集群管理系统设计方案

## 0. 当前实现状态（代码落地，2026-02-05）

说明：本仓库已把“阶段 1（核心组件）”落地为可运行代码，并补齐 CPU 计费与控制；其余阶段（如真实排队分配、支付对接、高可用、完整监控栈）属于上线增强项，需要按实际资源与优先级继续推进。

### 已实现（可用于试点/全量上线的核心闭环）
1. 节点 Agent（Golang）：采集 GPU 计算进程（`nvidia-smi`）与 CPU 占用（按进程采样差分），每分钟上报到控制器
2. 上报幂等：每次上报携带 `report_id`，控制器通过 `metric_reports` 表去重，避免网络重试导致重复扣费
3. 控制器（Golang + Gin）：接收上报、落库、计费（GPU + CPU）、更新余额/状态、下发动作（通知/限制/终止/CPU 限流）
4. CPU 控制三段兜底：优先 `systemd CPUQuota`，其次 `cgroup v2 cpu.max`，最后 `cgroup v1 cpu.cfs_*`（兼容无法升级到 cgroup v2 的机器）
5. Bash Hook：在用户启动“疑似 GPU 任务”前检查余额状态（尽量不误伤）
6. Web 管理界面（Vue3 + TypeScript + Element Plus）：`pnpm build` 输出到 `web/dist/`，由控制器直接托管（无需单独前端服务）
7. 节点状态：控制器落库 nodes 表，可通过管理员接口查看节点在线/上报情况
8. 使用记录 CSV 导出：管理员接口支持导出 CSV 便于对账与计费归档
9. 数据库迁移脚本：`database/migrations/`（含幂等表与 nodes 表）

落地操作手册：
- `docs/runbook.md`（一步步上线运行手册）

### 未实现/待增强（不影响核心闭环上线，但建议排期）
1. 更完善的权限体系：RBAC（多角色）、审计日志、2FA/SSO 等（当前已实现管理员账号 + Web 登录会话，满足上线运营）
2. 真实 GPU 资源排队与分配（当前仅记录排队，不做分配）
3. 通知渠道（邮件/企业微信/飞书）与支付对接
4. 监控栈（Prometheus/Grafana/DCGM Exporter）与告警规则
5. 控制器高可用（主备/负载均衡）

## 一、需求总结

### 核心需求（按优先级）
1. **保留SSH便携体验**：用户习惯SSH登录直接干活，不想学习复杂的Web/CLI流程
2. **资源调度和公平性**：解决"谁先上谁占GPU"问题，需要排队机制
3. **统一环境管理**：代码/环境/路径在不同服务器上一致
4. **存储性能**：担心NFS+HDD被训练I/O打爆

### 环境约束
- 24台异构Linux GPU服务器（全是NVIDIA显卡）
- 以机器/节点为最小管理单元
- 需要统一记账系统，长期目标是按实际使用计费

---

## 二、技术路线选择

### 推荐方案：轻量级Agent + 中转机控制器（Golang实现）

**核心理念：用户完全无感知**
- 用户照常SSH登录任何机器，无需学习新命令
- 系统在后台自动监控和计费
- 配额管理分级处理，不影响正常使用

**选择理由：**
- ✅ **用户体验最佳**：不需要学习salloc/srun等命令，保持现有SSH习惯
- ✅ **高性能**：Golang实现，比Python快10-100倍，内存占用低
- ✅ **部署简单**：编译成单个二进制文件，无需安装依赖
- ✅ **轻量级**：无需Slurm等重量级调度器
- ✅ **增量迁移**：可以逐台节点部署，不影响现有用户

**对比其他方案：**
- Slurm：需要学习新命令（salloc/srun），用户学习成本高
- Docker/K8s：SSH体验差，学习成本高，不适合"整机分配"场景
- KVM虚拟机：资源开销大，GPU直通限制多，迁移成本极高

---

## 三、系统整体架构

```
┌─────────────────────────────────────────────────────────────────┐
│                         用户层                                    │
│  用户直接SSH到任意节点 → 运行任务（完全无感知）                    │
└─────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────┐
│                   中转机（控制节点）                              │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐          │
│  │ 控制器API     │  │  计费引擎     │  │  Web管理界面  │          │
│  │ (Golang)     │  │  (Golang)    │  │  (Vue.js)    │          │
│  └──────────────┘  └──────────────┘  └──────────────┘          │
│  ┌──────────────┐  ┌──────────────┐                            │
│  │ PostgreSQL   │  │  排队系统     │                            │
│  │ (用户/计费)   │  │  (可选)      │                            │
│  └──────────────┘  └──────────────┘                            │
└─────────────────────────────────────────────────────────────────┘
              ↑ 上报数据（每分钟）
              ↓ 下发限制指令
┌─────────────────────────────────────────────────────────────────┐
│                      计算节点层（24台）                           │
│  ┌────────────────────────────────────────────────────────┐    │
│  │ 节点1: Node Agent (Golang) + Bash Hook                 │    │
│  │ GPU: 4x RTX 3090 (24GB)                                │    │
│  │ 功能：监控GPU使用 + 执行限制指令                         │    │
│  └────────────────────────────────────────────────────────┘    │
│  ┌────────────────────────────────────────────────────────┐    │
│  │ 节点2: Node Agent (Golang) + Bash Hook                 │    │
│  │ GPU: 8x A100 (80GB)                                    │    │
│  │ 功能：监控GPU使用 + 执行限制指令                         │    │
│  └────────────────────────────────────────────────────────┘    │
│  ... (共24台)                                                   │
└─────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────┐
│                      存储层                                       │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐          │
│  │  共享存储     │  │  本地缓存     │  │  对象存储     │          │
│  │  (NFS/BeeGFS)│  │  (NVMe SSD)  │  │  (MinIO)     │          │
│  └──────────────┘  └──────────────┘  └──────────────┘          │
└─────────────────────────────────────────────────────────────────┘
```

---

## 四、核心组件设计

### 4.1 配额限制分级策略

**级别1：正常状态（余额充足）**
- **条件**：余额 > 预警阈值（如100元）
- **行为**：无限制，正常使用

**级别2：预警状态（余额不足）**
- **条件**：10元 < 余额 < 100元
- **行为**：
  - 用户登录时显示余额预警
  - 每次启动GPU任务时提示剩余额度
  - 不限制使用

**级别3：限制新任务（余额临界）**
- **条件**：0元 < 余额 < 10元
- **行为**：
  - 阻止启动新的GPU任务（通过bash hook拦截）
  - 已运行的任务不受影响
  - 允许SSH登录和CPU任务
  - 同时下发 CPU 限流（例如限制到 50%），避免“欠费但用 CPU 继续跑大任务”

**级别4：强制终止（余额耗尽）**
- **条件**：余额 < 0元（欠费）
- **行为**：
  - 发送警告通知（邮件/消息）
  - 10分钟后kill所有GPU进程
  - 禁止启动新任务
  - 同时强限制 CPU 使用（例如限制到 10%）

**关于cgroup的说明：**
- 不使用动态cgroup限制（会让任务卡死）
- 在任务启动前通过bash hook拦截
- 对于已运行的任务，只在欠费时才kill

### 4.2 节点Agent（Golang实现）

**功能：**
1. 监控本机资源使用情况（CPU、内存、GPU）
2. 每分钟上报数据到中转机
3. 接收并执行限制指令
4. 上报携带 `report_id` 幂等字段，控制器可去重避免重复扣费

**核心代码结构：**
```go
// node-agent/main.go
type NodeAgent struct {
    nodeID        string
    controllerURL string
}

// 收集指标
func (a *NodeAgent) CollectMetrics() (*MetricsData, error)

// 获取GPU使用情况（调用nvidia-smi）
func (a *NodeAgent) getGPUUsageMap() (map[int32][]GPUUsage, error)

// 上报到中转机
func (a *NodeAgent) ReportToController(metrics *MetricsData) (*ControllerResponse, error)

// 执行限制指令
func (a *NodeAgent) ExecuteAction(action Action) error

// 主循环（每分钟）
func (a *NodeAgent) Run()
```

**部署方式：**
```bash
# 编译（在开发机上）
cd node-agent
go build -o node-agent main.go

# 部署到节点（单个二进制文件）
scp node-agent root@node01:/usr/local/bin/
ssh root@node01 "chmod +x /usr/local/bin/node-agent"

# 创建systemd服务
cat > /etc/systemd/system/gpu-node-agent.service <<EOF
[Unit]
Description=GPU Cluster Node Agent
After=network.target

[Service]
Type=simple
User=root
Environment="NODE_ID=node01"
Environment="CONTROLLER_URL=http://controller:8000"
ExecStart=/usr/local/bin/node-agent
Restart=always

[Install]
WantedBy=multi-user.target
EOF

systemctl enable gpu-node-agent
systemctl start gpu-node-agent
```

### 4.3 中转机控制器（Golang实现）

**功能：**
1. 接收所有节点的上报数据
2. 计算用户配额和扣费
3. 决策限制策略
4. 下发指令到节点
5. 提供REST API

**核心代码结构：**
```go
// controller/main.go
// 使用Gin框架

// 接收节点上报
POST /api/metrics

// 用户查询余额
GET /api/users/:username/balance

// 用户充值
POST /api/users/:username/recharge

// 管理员查询所有用户
GET /api/admin/users

// 管理员设置价格
POST /api/admin/prices
```

**计费逻辑：**
```go
func calculateCost(userData UserProcess) float64 {
    cost := 0.0

    // GPU费用（按型号）
    for _, gpu := range userData.GPUUsage {
        if strings.Contains(gpu.GPUModel, "A100") {
            cost += 0.5  // 0.5元/分钟
        } else if strings.Contains(gpu.GPUModel, "RTX 4090") {
            cost += 0.3
        } else if strings.Contains(gpu.GPUModel, "RTX 3090") {
            cost += 0.2
        } else {
            cost += 0.1
        }
    }

    return cost
}

func decideAction(username string, balance float64, userData UserProcess) *Action {
    if balance < 0 {
        // 欠费：kill进程
        return &Action{
            Type:     "kill_process",
            Username: username,
            PIDs:     []int32{userData.PID},
            Reason:   fmt.Sprintf("余额不足（当前余额：%.2f元），请充值", balance),
        }
    } else if balance < 10 {
        // 余额不足：阻止新任务
        return &Action{
            Type:     "block_user",
            Username: username,
            Reason:   fmt.Sprintf("余额不足（当前余额：%.2f元），无法启动新任务", balance),
        }
    } else if balance < 100 {
        // 预警：仅通知
        return &Action{
            Type:     "notify",
            Username: username,
            Message:  fmt.Sprintf("余额预警：当前余额 %.2f元，请及时充值", balance),
        }
    }

    return nil
}
```

**CPU 计费补充（核分钟，单价更低）：**
- 计费按“核分钟”计算：`cpu_percent/100 * cpu_price_per_core_minute * (interval_seconds/60)`
- 单价来源：
  - 优先从 `resource_prices` 表读取模型 `CPU_CORE`
  - 否则使用配置 `cpu_price_per_core_minute` 兜底
- CPU 控制（限流）与余额状态联动：`limited/blocked` 状态下下发 `set_cpu_quota`

### 4.4 用户侧拦截脚本（Bash Hook）

**目标：** 在用户启动GPU任务前检查配额

**实现方式：** 修改用户的`.bashrc`，hook常用命令

```bash
# /opt/gpu-cluster/check_quota.sh
#!/bin/bash

USERNAME=$(whoami)
CONTROLLER_URL="http://controller:8000"

# 检查余额
check_balance() {
    RESPONSE=$(curl -s "$CONTROLLER_URL/api/users/$USERNAME/balance")
    BALANCE=$(echo $RESPONSE | jq -r '.balance')
    STATUS=$(echo $RESPONSE | jq -r '.status')

    if [ "$STATUS" == "limited" ]; then
        echo "❌ 余额不足（当前余额：$BALANCE 元），无法启动GPU任务"
        echo "请联系管理员充值或访问：http://controller:8000/recharge"
        return 1
    elif [ "$STATUS" == "warning" ]; then
        echo "⚠️  余额预警：当前余额 $BALANCE 元，请及时充值"
        return 0
    fi

    return 0
}

# Hook python命令（如果使用GPU）
python() {
    # 检查是否使用GPU（简单判断：是否导入torch/tensorflow）
    if grep -q "import torch\|import tensorflow\|from torch\|from tensorflow" "$@" 2>/dev/null; then
        check_balance || return 1
    fi

    command python "$@"
}

# Hook nvidia-smi（用户查看GPU时提示余额）
nvidia-smi() {
    check_balance
    command nvidia-smi "$@"
}

export -f python
export -f nvidia-smi
```

**自动部署到所有用户：**
```bash
# 在每个节点上执行
for user in $(ls /home); do
    if [ -d "/home/$user" ]; then
        echo "source /opt/gpu-cluster/check_quota.sh" >> /home/$user/.bashrc
    fi
done
```

### 4.5 数据库设计（PostgreSQL）

```sql
-- 用户账户表
CREATE TABLE users (
    user_id SERIAL PRIMARY KEY,
    username VARCHAR(50) UNIQUE NOT NULL,
    balance DECIMAL(10,2) DEFAULT 100.0,
    status VARCHAR(20) DEFAULT 'normal',  -- 'normal', 'warning', 'limited', 'blocked'
    last_charge_time TIMESTAMP DEFAULT NOW(),
    created_at TIMESTAMP DEFAULT NOW()
);

-- 使用记录表
CREATE TABLE usage_records (
    record_id SERIAL PRIMARY KEY,
    node_id VARCHAR(50),
    username VARCHAR(50),
    timestamp TIMESTAMP,
    cpu_percent FLOAT,
    memory_mb FLOAT,
    gpu_usage JSONB,  -- JSON格式存储GPU使用详情
    cost DECIMAL(10,4),
    created_at TIMESTAMP DEFAULT NOW()
);

-- 充值记录表
CREATE TABLE recharge_records (
    recharge_id SERIAL PRIMARY KEY,
    username VARCHAR(50),
    amount DECIMAL(10,2),
    method VARCHAR(50),  -- 'alipay', 'wechat', 'admin'
    created_at TIMESTAMP DEFAULT NOW()
);

-- 资源价格表
CREATE TABLE resource_prices (
    price_id SERIAL PRIMARY KEY,
    gpu_model VARCHAR(50),  -- 'A100', 'RTX 4090', 'RTX 3090'
    price_per_minute DECIMAL(10,4),  -- 每分钟价格
    updated_at TIMESTAMP DEFAULT NOW()
);

-- 索引
CREATE INDEX idx_usage_username ON usage_records(username);
CREATE INDEX idx_usage_timestamp ON usage_records(timestamp);
CREATE INDEX idx_usage_node ON usage_records(node_id);
```

### 4.6 排队机制（可选）

**用户命令：**
```bash
# 用户申请GPU资源（如果没有空闲GPU，自动排队）
gpu-request --gpu-type=rtx3090 --count=2

# 输出：
# 当前无可用GPU，已加入排队（队列位置：3）
# 预计等待时间：约15分钟
# 可用时会通过邮件/消息通知您
```

**排队逻辑（在控制器中实现）：**
```go
// controller/queue.go
type QueueItem struct {
    Username  string
    GPUType   string
    Count     int
    Timestamp time.Time
}

var queue = make([]QueueItem, 0)

func RequestGPU(username, gpuType string, count int) Response {
    // 1. 检查是否有空闲GPU
    availableNodes := findAvailableGPU(gpuType, count)

    if len(availableNodes) > 0 {
        // 有空闲GPU，直接分配
        return Response{
            Status:  "allocated",
            Node:    availableNodes[0],
            Message: fmt.Sprintf("已分配节点：%s，请SSH登录使用", availableNodes[0]),
        }
    } else {
        // 无空闲GPU，加入排队
        item := QueueItem{
            Username:  username,
            GPUType:   gpuType,
            Count:     count,
            Timestamp: time.Now(),
        }
        queue = append(queue, item)

        position := len(queue)
        estimatedWait := estimateWaitTime(position)

        return Response{
            Status:           "queued",
            Position:         position,
            EstimatedMinutes: estimatedWait,
            Message:          fmt.Sprintf("当前无可用GPU，已加入排队（队列位置：%d）", position),
        }
    }
}
```

### 4.5 存储方案

**三层存储架构：**

1. **共享存储（NFS/BeeGFS）**
   - 用途：代码、环境、小文件
   - 挂载点：`/shared/home`, `/shared/datasets`
   - 优化：启用缓存，限制并发连接数

2. **本地缓存（NVMe SSD）**
   - 用途：训练时的热数据、checkpoint
   - 路径：`/local/scratch/$USER/$JOB_ID`
   - 自动清理：作业结束后删除

3. **对象存储（MinIO）**
   - 用途：大模型、最终结果归档
   - 访问：通过S3 API

**使用建议（写入用户文档）：**
```bash
# 训练前：将数据集复制到本地
cp -r /shared/datasets/imagenet /local/scratch/$USER/$SLURM_JOB_ID/

# 训练时：checkpoint写入本地
python train.py --checkpoint_dir /local/scratch/$USER/$SLURM_JOB_ID/ckpt

# 训练后：重要结果上传到MinIO
mc cp /local/scratch/$USER/$SLURM_JOB_ID/final_model.pth minio/models/
```

---

## 五、技术栈选择

### 5.1 核心组件
- **节点Agent**：Golang 1.21+（编译成单个二进制文件）
- **中转机控制器**：Golang 1.21+ + Gin框架
- **数据库**：PostgreSQL 15+（用户余额、使用记录、计费）
- **监控**：Prometheus + Grafana + Node Exporter + NVIDIA DCGM Exporter
- **Web管理界面**：Vue.js 3 + TypeScript + Element Plus

### 5.2 Golang依赖包
```go
// node-agent
github.com/shirou/gopsutil/v3  // 进程监控
encoding/json                   // JSON处理
net/http                        // HTTP客户端

// controller
github.com/gin-gonic/gin        // Web框架
github.com/lib/pq               // PostgreSQL驱动
github.com/golang-jwt/jwt       // JWT认证（可选）
```

### 5.3 基础设施
- **操作系统**：Ubuntu 22.04 LTS 或 Rocky Linux 9
- **网络**：10GbE（最低）
- **存储**：NFS 4.2 或 BeeGFS 7.4+（共享存储）+ NVMe SSD（本地缓存）

---

## 六、实施路线图（增量迁移）

### 阶段1：开发核心组件（2-3周）

**目标：** 开发并测试节点Agent和控制器

**步骤：**
1. 开发节点Agent（Golang）
   - 实现GPU监控功能（nvidia-smi解析）
   - 实现进程监控功能（gopsutil）
   - 实现上报功能（HTTP POST）
   - 实现限制指令执行（kill进程、阻止用户）

2. 开发中转机控制器（Golang）
   - 实现接收上报API
   - 实现计费逻辑
   - 实现决策逻辑
   - 实现数据库操作

3. 搭建PostgreSQL数据库
   - 创建表结构
   - 初始化测试数据

**验证：**
```bash
# 测试Agent上报
curl http://controller:8000/api/metrics

# 测试查询余额
curl http://controller:8000/api/users/testuser/balance
```

### 阶段2：试点部署（1-2周）

**目标：** 在2-3台节点上试点部署

**步骤：**
1. 选择2-3台空闲/低负载节点
2. 部署节点Agent（systemd服务）
3. 部署bash hook脚本
4. 邀请1-2个用户试用
5. 收集反馈，修复bug

**验证：**
```bash
# 检查Agent运行状态
systemctl status gpu-node-agent

# 检查上报数据
tail -f /var/log/gpu-node-agent.log

# 用户测试
ssh testuser@node01
python train.py  # 应该能看到余额提示
```

### 阶段3：全量部署（3-4周）

**目标：** 将所有24台节点纳入管理

**步骤：**
1. 逐台部署节点Agent（每周6-8台）
2. 部署bash hook到所有用户
3. 通知用户新系统上线
4. 监控系统运行状态

**部署脚本：**
```bash
#!/bin/bash
# deploy.sh - 批量部署脚本

NODES="node01 node02 node03 ... node24"
CONTROLLER_URL="http://controller:8000"

for node in $NODES; do
    echo "Deploying to $node..."

    # 复制Agent二进制文件
    scp node-agent root@$node:/usr/local/bin/

    # 创建systemd服务
    ssh root@$node "cat > /etc/systemd/system/gpu-node-agent.service <<EOF
[Unit]
Description=GPU Cluster Node Agent
After=network.target

[Service]
Type=simple
User=root
Environment=\"NODE_ID=$node\"
Environment=\"CONTROLLER_URL=$CONTROLLER_URL\"
ExecStart=/usr/local/bin/node-agent
Restart=always

[Install]
WantedBy=multi-user.target
EOF"

    # 启动服务
    ssh root@$node "systemctl enable gpu-node-agent && systemctl start gpu-node-agent"

    echo "$node deployed successfully"
done
```

### 阶段4：Web管理界面开发（2-3周）

**目标：** 开发用户和管理员Web界面

**功能：**
- 用户端：
  - 查看余额和使用历史
  - 查看费用明细
  - 在线充值（支付宝/微信）
  - 查看排队状态（如果启用）

- 管理员端：
  - 查看所有节点状态
  - 查看所有用户余额
  - 手动充值/扣费
  - 设置GPU价格
  - 查看系统统计

### 阶段5：排队系统开发（可选，2周）

**目标：** 实现GPU资源排队机制

**步骤：**
1. 在控制器中实现排队逻辑
2. 开发`gpu-request`命令行工具
3. 实现通知机制（邮件/消息）
4. 测试排队和分配流程

### 阶段6：监控和优化（持续）

**目标：** 部署监控系统，持续优化

**步骤：**
1. 部署Prometheus + Grafana
2. 配置NVIDIA DCGM Exporter
3. 创建监控仪表盘
4. 设置告警规则
5. 根据监控数据优化系统

---

## 七、关键技术实现细节

### 7.1 用户使用流程（完全无感知）

**正常使用场景：**
```bash
# 1. 用户直接SSH到任意节点
ssh user@node05

# 2. 直接运行任务（无需任何新命令）
python train.py

# 3. 系统后台自动计费
# （用户完全无感知）
```

**余额不足场景：**
```bash
# 1. SSH登录
ssh user@node05

# 2. 尝试运行GPU任务
python train.py

# 输出：
# ⚠️  余额预警：当前余额 8.5 元，请及时充值
# （任务正常运行）

# 3. 余额耗尽后
python train.py

# 输出：
# ❌ 余额不足（当前余额：-2.3 元），无法启动GPU任务
# 请联系管理员充值或访问：http://controller:8000/recharge
```

### 7.2 GPU监控实现（nvidia-smi解析）

**Golang实现：**
```go
func (a *NodeAgent) getGPUUsageMap() (map[int32][]GPUUsage, error) {
    result := make(map[int32][]GPUUsage)

    // 调用nvidia-smi获取GPU使用情况
    cmd := exec.Command("nvidia-smi",
        "--query-compute-apps=pid,gpu_name,gpu_bus_id,used_memory",
        "--format=csv,noheader,nounits")

    output, err := cmd.Output()
    if err != nil {
        return nil, fmt.Errorf("nvidia-smi failed: %w", err)
    }

    // 解析输出
    lines := strings.Split(string(output), "\n")
    for _, line := range lines {
        line = strings.TrimSpace(line)
        if line == "" {
            continue
        }

        parts := strings.Split(line, ",")
        if len(parts) < 4 {
            continue
        }

        // 解析PID
        pidStr := strings.TrimSpace(parts[0])
        pid, err := strconv.ParseInt(pidStr, 10, 32)
        if err != nil {
            continue
        }

        // 解析GPU信息
        gpuName := strings.TrimSpace(parts[1])
        memoryStr := strings.TrimSpace(parts[3])
        memoryMB, _ := strconv.ParseFloat(memoryStr, 64)

        gpuUsage := GPUUsage{
            GPUID:    0,  // 可以从bus_id映射
            GPUModel: gpuName,
            MemoryMB: memoryMB,
        }

        result[int32(pid)] = append(result[int32(pid)], gpuUsage)
    }

    return result, nil
}
```

### 7.3 进程监控实现（gopsutil）

**Golang实现：**
```go
func (a *NodeAgent) CollectMetrics() (*MetricsData, error) {
    metrics := &MetricsData{
        NodeID:    a.nodeID,
        Timestamp: time.Now().Format(time.RFC3339),
        Users:     []UserProcess{},
    }

    // 获取所有进程
    processes, err := process.Processes()
    if err != nil {
        return nil, fmt.Errorf("failed to get processes: %w", err)
    }

    // 获取GPU使用情况
    gpuUsageMap, _ := a.getGPUUsageMap()

    // 遍历所有进程
    for _, proc := range processes {
        username, err := proc.Username()
        if err != nil || username == "root" {
            continue
        }

        cpuPercent, _ := proc.CPUPercent()
        memInfo, _ := proc.MemoryInfo()
        memoryMB := float64(memInfo.RSS) / 1024 / 1024

        pid := proc.Pid
        gpuUsage, hasGPU := gpuUsageMap[pid]

        // 只记录使用GPU的进程
        if hasGPU {
            userProc := UserProcess{
                Username:   username,
                PID:        pid,
                CPUPercent: cpuPercent,
                MemoryMB:   memoryMB,
                GPUUsage:   gpuUsage,
            }
            metrics.Users = append(metrics.Users, userProc)
        }
    }

    return metrics, nil
}
```

### 7.4 限制指令执行

**Kill进程：**
```go
func (a *NodeAgent) killProcesses(username string, pids []int32, reason string) error {
    log.Printf("Killing processes for user %s: %v (reason: %s)", username, pids, reason)

    for _, pid := range pids {
        proc, err := process.NewProcess(pid)
        if err != nil {
            continue
        }

        // 验证用户名
        procUsername, err := proc.Username()
        if err != nil || procUsername != username {
            continue
        }

        // 终止进程
        if err := proc.Kill(); err != nil {
            log.Printf("Failed to kill process %d: %v", pid, err)
        } else {
            log.Printf("Successfully killed process %d", pid)
        }
    }

    return nil
}
```

**阻止用户启动新任务：**
```go
func (a *NodeAgent) blockUserGPUAccess(username string, reason string) error {
    log.Printf("Blocking GPU access for user %s (reason: %s)", username, reason)

    // 在用户的home目录创建标记文件
    homeDir := fmt.Sprintf("/home/%s", username)
    flagFile := fmt.Sprintf("%s/.gpu_blocked", homeDir)

    file, err := os.Create(flagFile)
    if err != nil {
        return fmt.Errorf("failed to create block flag: %w", err)
    }
    defer file.Close()

    // 写入原因
    if _, err := file.WriteString(reason); err != nil {
        return fmt.Errorf("failed to write reason: %w", err)
    }

    return nil
}
```

**CPU 限流（余额联动，兼容 cgroup v1/v2）：**
- action：`set_cpu_quota`，字段 `cpu_quota_percent`（0 表示解除限制）
- 兜底顺序：
  1) systemd：`systemctl set-property --runtime user-<uid>.slice CPUQuota=...`
  2) cgroup v2：写 `cpu.max`（并尝试写 `cgroup.procs` 迁移该用户进程）
  3) cgroup v1：写 `cpu.cfs_period_us/cpu.cfs_quota_us`（并把用户进程 PID 写入 `tasks`）
- 要求：Agent 以 root 运行（写 cgroup 与迁移进程需要权限）

### 7.5 异构GPU支持

**价格配置（数据库）：**
```sql
INSERT INTO resource_prices (gpu_model, price_per_minute) VALUES
('A100', 0.50),
('RTX 4090', 0.30),
('RTX 3090', 0.20),
('RTX 3080', 0.15),
('V100', 0.40);
```

**计费逻辑（自动识别GPU型号）：**
```go
func calculateCost(userData UserProcess, prices map[string]float64) float64 {
    cost := 0.0

    for _, gpu := range userData.GPUUsage {
        // 从数据库价格表中查找
        for model, price := range prices {
            if strings.Contains(gpu.GPUModel, model) {
                cost += price
                break
            }
        }
    }

    return cost
}
```

### 7.6 数据上报和通信

**节点Agent上报（HTTP POST）：**
```go
func (a *NodeAgent) ReportToController(metrics *MetricsData) (*ControllerResponse, error) {
    jsonData, err := json.Marshal(metrics)
    if err != nil {
        return nil, err
    }

    url := fmt.Sprintf("%s/api/metrics", a.controllerURL)
    resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var controllerResp ControllerResponse
    if err := json.NewDecoder(resp.Body).Decode(&controllerResp); err != nil {
        return nil, err
    }

    return &controllerResp, nil
}
```

**控制器接收（Gin框架）：**
```go
func (c *Controller) ReceiveMetrics(ctx *gin.Context) {
    var data MetricsData
    if err := ctx.ShouldBindJSON(&data); err != nil {
        ctx.JSON(400, gin.H{"error": err.Error()})
        return
    }

    // 存储数据
    c.storeMetrics(&data)

    // 计算费用并决策
    actions := []Action{}
    for _, user := range data.Users {
        cost := c.calculateCost(user)
        newBalance := c.deductBalance(user.Username, cost)

        action := c.decideAction(user.Username, newBalance, user)
        if action != nil {
            actions = append(actions, *action)
        }
    }

    ctx.JSON(200, gin.H{"actions": actions})
}
```

---

## 八、关键文件清单

### 8.1 节点Agent代码
- `node-agent/main.go` - 主程序入口
- `node-agent/metrics.go` - 指标收集
- `node-agent/gpu.go` - GPU监控
- `node-agent/action.go` - 限制指令执行
- `node-agent/cpu_quota.go` - CPU 限流（systemd / cgroup v2 / cgroup v1）
- `node-agent/report.go` - 上报与本地补报（jsonl）
- `node-agent/go.mod` - Go模块依赖

### 8.2 控制器代码
- `controller/main.go` - 主程序入口
- `controller/api.go` - REST API
- `controller/billing.go` - 计费逻辑
- `controller/database.go` - 数据库操作
- `controller/queue.go` - 排队系统（可选）
- `controller/go.mod` - Go模块依赖

### 8.3 数据库脚本
- `database/schema.sql` - 表结构定义
- `database/init_data.sql` - 初始化数据
- `database/migrations/` - 数据库迁移脚本

### 8.4 部署脚本
- `scripts/deploy_agent.sh` - 批量部署Agent
- `scripts/deploy_controller.sh` - 部署控制器（示例）
- `scripts/deploy_hook.sh` - 部署bash hook
- `scripts/check_status.sh` - 检查系统状态
- `scripts/build_linux.sh` - 构建 Linux 可部署二进制

### 8.5 用户工具
- `tools/check_quota.sh` - Bash hook脚本
- `tools/gpu-request` - GPU申请工具（可选）
- `tools/balance-query` - 余额查询工具

### 8.6 Web界面
- `web/src/` - Vue3 + TypeScript 源码（Element Plus）
- `web/package.json` - 前端依赖与脚本（pnpm）
- `web/dist/` - 构建产物（控制器直接托管）

### 8.7 配置文件
- `config/controller.yaml` - 控制器配置
- `database/init_data.sql` - 默认价格（含 `CPU_CORE`）
- `systemd/gpu-node-agent.service` - systemd服务文件
- `systemd/gpu-controller.service` - 控制器 systemd 服务文件

---

## 九、风险与应对

### 9.1 性能风险
**风险：** Golang Agent占用资源，影响训练任务
**应对：**
- Agent每分钟只采集一次，CPU占用<0.1%
- 使用goroutine并发处理，内存占用<10MB
- 监控Agent性能，必要时优化采集频率

### 9.2 网络风险
**风险：** 中转机故障，节点无法上报
**应对：**
- Agent本地缓存数据，网络恢复后补报
- 中转机部署高可用（主备）
- 设置合理的超时和重试机制

### 9.3 计费准确性风险
**风险：** 计费错误导致用户投诉
**应对：**
- 初期设置"试运行模式"（记录但不扣费）
- 提供详细的费用明细查询
- 设置申诉机制，管理员可手动调整
- 上报幂等：使用 `report_id` + `metric_reports` 去重，避免网络重试导致重复扣费

### 9.4 用户抵触风险
**风险：** 用户不习惯新系统，影响工作
**应对：**
- 用户完全无感知，无需学习新命令
- 提供详细文档和FAQ
- 设置缓冲期，初期只预警不限制

### 9.5 存储风险
**风险：** NFS性能瓶颈导致训练变慢
**应对：**
- 短期：引导用户使用本地缓存
- 中期：部署BeeGFS或Lustre
- 长期：考虑全闪存存储

---

## 十、成功标准

### 10.1 功能指标
- ✅ 所有24台节点部署Agent并正常上报
- ✅ 用户可以直接SSH登录任意节点，无需学习新命令
- ✅ 系统准确记录所有GPU使用情况
- ✅ 计费系统自动扣费，余额不足时阻止新任务
- ✅ Web界面可以查询余额和使用历史

### 10.2 性能指标
- ✅ Agent CPU占用 < 0.1%
- ✅ Agent内存占用 < 10MB
- ✅ 数据上报延迟 < 1秒
- ✅ 控制器响应时间 < 100ms
- ✅ GPU利用率提升 > 20%（相比现状）

### 10.3 用户体验指标
- ✅ 用户无需培训即可使用（保持SSH习惯）
- ✅ 用户满意度 > 80%（问卷调查）
- ✅ 资源冲突投诉减少 > 90%
- ✅ 计费投诉 < 5%

### 10.4 可靠性指标
- ✅ Agent服务可用性 > 99.9%
- ✅ 控制器服务可用性 > 99.9%
- ✅ 数据上报成功率 > 99%
- ✅ 计费准确率 > 99.9%

---

## 十一、后续扩展方向

1. **容器化支持**：集成Docker/Singularity，统一环境管理
2. **Jupyter集成**：提供Web版Jupyter，保持SSH体验
3. **多集群联邦**：支持多个地点的集群统一管理
4. **AI调度优化**：根据历史数据预测任务运行时间，优化资源分配
5. **自动扩缩容**：集成云GPU资源（AWS/阿里云），高峰期自动扩容
6. **更细粒度的资源隔离**：使用MIG（Multi-Instance GPU）技术，一张A100分成多个实例

---

## 十二、与Slurm方案对比

| 维度 | 轻量级Golang方案 | Slurm方案 |
|------|-----------------|-----------|
| **用户体验** | ⭐⭐⭐⭐⭐ 完全无感知 | ⭐⭐⭐ 需要学习salloc/srun |
| **部署难度** | ⭐⭐⭐⭐⭐ 单个二进制文件 | ⭐⭐ 需要配置多个组件 |
| **性能** | ⭐⭐⭐⭐⭐ Golang高性能 | ⭐⭐⭐⭐ 成熟但较重 |
| **灵活性** | ⭐⭐⭐⭐ 自研可定制 | ⭐⭐⭐ 配置复杂 |
| **成熟度** | ⭐⭐⭐ 需要开发 | ⭐⭐⭐⭐⭐ HPC标准 |
| **学习成本** | ⭐⭐⭐⭐⭐ 用户无需学习 | ⭐⭐ 用户需要培训 |

**推荐理由：**
- 您的核心需求是"用户无感知"，Golang方案完美满足
- 不需要学习新命令，保持SSH习惯
- 部署简单，增量迁移容易
- 性能优秀，资源占用低

---

## 十三、参考资源

- Golang官方文档：https://go.dev/doc/
- Gin框架文档：https://gin-gonic.com/docs/
- gopsutil库：https://github.com/shirou/gopsutil
- PostgreSQL文档：https://www.postgresql.org/docs/
- NVIDIA GPU监控：https://docs.nvidia.com/datacenter/dcgm/
- Prometheus + Grafana：https://prometheus.io/docs/

---

## 十四、项目目录结构

```
hit-aiot-ops/
├── node-agent/              # 节点Agent（Golang）
│   ├── main.go
│   ├── metrics.go
│   ├── gpu.go
│   ├── action.go
│   ├── go.mod
│   └── go.sum
├── controller/              # 中转机控制器（Golang）
│   ├── main.go
│   ├── api.go
│   ├── billing.go
│   ├── database.go
│   ├── queue.go
│   ├── go.mod
│   └── go.sum
├── web/                     # Web管理界面（Vue.js）
│   ├── src/
│   ├── public/
│   ├── package.json
│   └── vite.config.js
├── database/                # 数据库脚本
│   ├── schema.sql
│   ├── init_data.sql
│   └── migrations/
├── scripts/                 # 部署脚本
│   ├── deploy_agent.sh
│   ├── deploy_hook.sh
│   └── check_status.sh
├── tools/                   # 用户工具
│   ├── check_quota.sh
│   ├── gpu-request
│   └── balance-query
├── config/                  # 配置文件
│   ├── controller.yaml
│   └── prices.yaml
├── systemd/                 # systemd服务文件
│   └── gpu-node-agent.service
├── docs/                    # 文档
│   ├── user-guide.md
│   ├── admin-guide.md
│   └── api-reference.md
└── README.md
```

---

## 十五、下一步行动

1. **立即开始**：开发节点Agent和控制器核心功能
2. **第一周**：完成基本的监控和上报功能
3. **第二周**：完成计费逻辑和数据库设计
4. **第三周**：在2-3台节点上试点部署
5. **第四周**：根据反馈优化，准备全量部署

**关键里程碑：**
- Week 3: 试点部署成功
- Week 6: 全量部署完成
- Week 9: Web界面上线
- Week 12: 系统稳定运行，开始收集用户反馈
