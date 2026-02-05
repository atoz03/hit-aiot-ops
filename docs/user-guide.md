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

