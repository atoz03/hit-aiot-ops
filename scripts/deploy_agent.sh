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
ENABLE_SSH_GUARD="${ENABLE_SSH_GUARD:-0}"
SSH_GUARD_EXCLUDE_USERS="${SSH_GUARD_EXCLUDE_USERS:-root baojh xqt}"
SSH_GUARD_FAIL_OPEN="${SSH_GUARD_FAIL_OPEN:-1}"

if [[ -z "${NODES}" ]]; then
  echo "请设置环境变量 NODES，例如：" >&2
  echo "  - 旧格式（不推荐）：NODES=\"node01 node02\"" >&2
  echo "  - 推荐格式（机器编号:IP/主机名）：NODES=\"60000:192.168.1.104 60001:192.168.1.220\"" >&2
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

for item in ${NODES}; do
  node_id="${item}"
  host="${item}"
  if [[ "${item}" == *:* ]]; then
    node_id="${item%%:*}"
    host="${item#*:}"
  fi

  echo "==> 部署到 ${host}（NODE_ID=${node_id}）"
  scp "${AGENT_BIN}" "root@${host}:/usr/local/bin/node-agent"
  ssh "root@${host}" "chmod +x /usr/local/bin/node-agent"
  ssh "root@${host}" "mkdir -p /etc/systemd/system"
  ssh "root@${host}" "cat > /etc/systemd/system/gpu-node-agent.service <<EOF
[Unit]
Description=GPU Cluster Node Agent
After=network.target

[Service]
Type=simple
User=root
Environment=NODE_ID=${node_id}
Environment=CONTROLLER_URL=${CONTROLLER_URL}
Environment=AGENT_TOKEN=${AGENT_TOKEN}
ExecStart=/usr/local/bin/node-agent
Restart=always

[Install]
WantedBy=multi-user.target
EOF"
  ssh "root@${host}" "systemctl daemon-reload && systemctl enable gpu-node-agent && systemctl restart gpu-node-agent"

  if [[ "${ENABLE_SSH_GUARD}" == "1" ]]; then
    echo "==> 安装 SSH 登录拦截（仅允许已登记用户；排除：${SSH_GUARD_EXCLUDE_USERS}）"
    ssh "root@${host}" "mkdir -p /opt/gpu-cluster /etc/gpu-cluster /var/lib/gpu-cluster /etc/systemd/system"

    ssh "root@${host}" "cat > /etc/gpu-cluster/ssh_guard.conf <<EOF
CONTROLLER_URL=\"${CONTROLLER_URL}\"
NODE_ID=\"${node_id}\"
EXCLUDE_USERS=\"${SSH_GUARD_EXCLUDE_USERS}\"
FAIL_OPEN=\"${SSH_GUARD_FAIL_OPEN}\"
ALLOWLIST_FILE=\"/var/lib/gpu-cluster/registered_users.txt\"
EOF"

    ssh "root@${host}" "cat > /opt/gpu-cluster/sync_registered_users.sh <<'EOF'
#!/bin/bash
set -euo pipefail

CONF=\"/etc/gpu-cluster/ssh_guard.conf\"
if [[ -f \"${CONF}\" ]]; then
  # shellcheck disable=SC1090
  source \"${CONF}\"
fi

CONTROLLER_URL=\"${CONTROLLER_URL:-}\"
NODE_ID=\"${NODE_ID:-}\"
ALLOWLIST_FILE=\"${ALLOWLIST_FILE:-/var/lib/gpu-cluster/registered_users.txt}\"

if [[ -z \"${CONTROLLER_URL}\" || -z \"${NODE_ID}\" ]]; then
  echo \"missing CONTROLLER_URL/NODE_ID\" >&2
  exit 2
fi

tmp=\"${ALLOWLIST_FILE}.tmp\"
mkdir -p \"$(dirname \"${ALLOWLIST_FILE}\")\"

curl -fsS \"${CONTROLLER_URL}/api/registry/nodes/${NODE_ID}/users.txt\" -o \"${tmp}\"
mv \"${tmp}\" \"${ALLOWLIST_FILE}\"
chmod 0644 \"${ALLOWLIST_FILE}\"
EOF"
    ssh "root@${host}" "chmod +x /opt/gpu-cluster/sync_registered_users.sh"

    ssh "root@${host}" "cat > /opt/gpu-cluster/ssh_login_check.sh <<'EOF'
#!/bin/bash
set -euo pipefail

CONF=\"/etc/gpu-cluster/ssh_guard.conf\"
if [[ -f \"${CONF}\" ]]; then
  # shellcheck disable=SC1090
  source \"${CONF}\"
fi

user=\"${PAM_USER:-}\"
if [[ -z \"${user}\" ]]; then
  exit 0
fi

EXCLUDE_USERS=\"${EXCLUDE_USERS:-root}\"
for u in ${EXCLUDE_USERS}; do
  if [[ \"${user}\" == \"${u}\" ]]; then
    exit 0
  fi
done

CONTROLLER_URL=\"${CONTROLLER_URL:-}\"
NODE_ID=\"${NODE_ID:-}\"
FAIL_OPEN=\"${FAIL_OPEN:-1}\"
ALLOWLIST_FILE=\"${ALLOWLIST_FILE:-/var/lib/gpu-cluster/registered_users.txt}\"

if [[ -z \"${NODE_ID}\" ]]; then
  exit 0
fi

if [[ -f \"${ALLOWLIST_FILE}\" ]]; then
  if grep -Fxq \"${user}\" \"${ALLOWLIST_FILE}\"; then
    exit 0
  fi
  exit 1
fi

if [[ -z \"${CONTROLLER_URL}\" ]]; then
  if [[ \"${FAIL_OPEN}\" == \"1\" ]]; then
    exit 0
  fi
  exit 1
fi

resp=\"$(curl -fsS \"${CONTROLLER_URL}/api/registry/resolve?node_id=${NODE_ID}&local_username=${user}\" 2>/dev/null || true)\"
if echo \"${resp}\" | grep -q '\"registered\":true'; then
  exit 0
fi

if [[ \"${FAIL_OPEN}\" == \"1\" && -z \"${resp}\" ]]; then
  exit 0
fi
exit 1
EOF"
    ssh "root@${host}" "chmod +x /opt/gpu-cluster/ssh_login_check.sh"

    ssh "root@${host}" "cat > /etc/systemd/system/gpu-ssh-guard-sync.service <<'EOF'
[Unit]
Description=GPU SSH Guard Allowlist Sync
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
ExecStart=/opt/gpu-cluster/sync_registered_users.sh
EOF"

    ssh "root@${host}" "cat > /etc/systemd/system/gpu-ssh-guard-sync.timer <<'EOF'
[Unit]
Description=GPU SSH Guard Allowlist Sync Timer

[Timer]
OnBootSec=30
OnUnitActiveSec=2min
Unit=gpu-ssh-guard-sync.service

[Install]
WantedBy=timers.target
EOF"

    # 启用定时同步（先跑一次，尽量避免首次拦截误伤）
    ssh "root@${host}" "systemctl daemon-reload && systemctl enable --now gpu-ssh-guard-sync.timer && systemctl start gpu-ssh-guard-sync.service || true"

    # PAM 接入（idempotent）：在 /etc/pam.d/sshd 的 account 阶段添加 pam_exec
    ssh "root@${host}" "if [[ -f /etc/pam.d/sshd ]] && ! grep -q \"/opt/gpu-cluster/ssh_login_check.sh\" /etc/pam.d/sshd; then echo \"account required pam_exec.so quiet /opt/gpu-cluster/ssh_login_check.sh\" >> /etc/pam.d/sshd; fi"
  fi
done
