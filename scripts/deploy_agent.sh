#!/bin/bash
# 批量部署 node-agent（示例脚本）
#
# 说明：
# - 该脚本不会执行破坏性命令（不会删除文件/清理目录）
# - 需要你自行准备节点列表与 SSH 免密/凭证

set -euo pipefail

AGENT_BIN="${AGENT_BIN:-./node-agent/node-agent}"
CONTROLLER_URL="${CONTROLLER_URL:-http://controller:8000}"
AGENT_TOKEN="${AGENT_TOKEN:-}"
NODES="${NODES:-}"

if [[ -z "${NODES}" ]]; then
  echo "请设置环境变量 NODES，例如：NODES=\"node01 node02 node03\"" >&2
  exit 2
fi
if [[ -z "${AGENT_TOKEN}" ]]; then
  echo "请设置环境变量 AGENT_TOKEN（用于 X-Agent-Token）" >&2
  exit 2
fi
if [[ ! -f "${AGENT_BIN}" ]]; then
  echo "未找到 Agent 二进制：${AGENT_BIN}" >&2
  exit 2
fi

for node in ${NODES}; do
  echo "==> 部署到 ${node}"
  scp "${AGENT_BIN}" "root@${node}:/usr/local/bin/node-agent"
  ssh "root@${node}" "chmod +x /usr/local/bin/node-agent"
  ssh "root@${node}" "mkdir -p /etc/systemd/system"
  ssh "root@${node}" "cat > /etc/systemd/system/gpu-node-agent.service <<EOF
[Unit]
Description=GPU Cluster Node Agent
After=network.target

[Service]
Type=simple
User=root
Environment=NODE_ID=${node}
Environment=CONTROLLER_URL=${CONTROLLER_URL}
Environment=AGENT_TOKEN=${AGENT_TOKEN}
ExecStart=/usr/local/bin/node-agent
Restart=always

[Install]
WantedBy=multi-user.target
EOF"
  ssh "root@${node}" "systemctl daemon-reload && systemctl enable gpu-node-agent && systemctl restart gpu-node-agent"
done

