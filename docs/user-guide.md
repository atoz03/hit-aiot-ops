# 用户使用指南（保持 SSH 习惯）

目标：用户照常 SSH 登录、照常运行训练脚本；系统在后台完成计费与必要限制。

## 1. 日常使用

```bash
ssh user@node05
python train.py
```

## 2. 余额提示与限制

- `normal`：正常使用
- `warning`：余额预警，任务可继续运行
- `limited`：限制启动新的 GPU 任务（通过 Bash Hook 拦截）；同时可能被限制 CPU 使用
- `blocked`：欠费状态，超过宽限期后 GPU 进程会被终止；同时强限制 CPU 使用

提示：
- 当系统需要通知你时，节点可能会在你的家目录写入提示文件：`~/.gpu_notice`
- 当系统对你施加 CPU 限制时，节点可能会在你的家目录写入：`~/.cpu_quota`（用于自助确认）

## 3. Bash Hook（GPU 任务启动前检查）

管理员会在你的 `~/.bashrc` 中加入：

```bash
source /opt/gpu-cluster/check_quota.sh
```

Hook 的策略是“尽量不误伤”：
- 控制器可达：以控制器返回的 `status` 为准
- 控制器不可达：仅当本地存在 `~/.gpu_blocked` 标记时才阻止启动疑似 GPU 任务

提示：
- Hook 主要拦截 `python/python3`（检测脚本/代码片段中是否包含 `torch/tensorflow/jax/cuda` 关键词）。

## 4. 自助查询余额

若集群提供 `tools/balance-query`：

```bash
CONTROLLER_URL=http://controller:8000 balance-query
```

## 4.1 自助登记（账号绑定 / 开号申请）

当集群启用“未登记禁止 SSH 登录”策略时，你需要先完成登记并等待审核通过：

1) 浏览器打开控制器 Web：
- `http://<controller>:8000/`

2) 进入菜单：
- `用户功能 -> 用户注册`

3) 选择：
- **账号绑定登记**：填写你在哪些端口(机器编号)有账号，以及对应的机器用户名
- **开号申请**：如果某台机器还没有账号，提交申请给管理员处理

4) 在「我的申请记录」里查看审核状态（pending/approved/rejected）。

## 5. 常见问题

1) 我能登录但跑不了 GPU 任务
- 可能处于 `limited/blocked` 状态（余额不足/欠费）
- 可先查询余额，再联系管理员充值

2) 控制器不可达怎么办
- Hook 会尽量不误伤：只有当本地存在 `~/.gpu_blocked` 标记时才会阻止
- 若你确认自己应当可用但仍被阻止，请联系管理员排障

3) 我无法 SSH 登录
- 可能集群启用了“未登记禁止 SSH 登录”
- 请先到 Web 的「用户注册」提交绑定登记/开号申请，并等待管理员审核通过
