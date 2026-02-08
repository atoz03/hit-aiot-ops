#!/bin/bash
# 批量部署 node-agent（示例脚本）
#
# 特性：
# - 支持 Ubuntu 22.04 自动安装运行依赖（curl/jq 等）
# - 支持可选安装 Go（某些节点后续需要本地调试/编译时）
# - 支持非 root SSH 用户（要求该用户可 sudo）
# - 保留 SSH Guard（登记/白名单登录校验）部署逻辑

set -euo pipefail

AGENT_BIN="${AGENT_BIN:-./node-agent/node-agent}"
CONTROLLER_URL="${CONTROLLER_URL:-http://controller:8000}"
AGENT_TOKEN="${AGENT_TOKEN:-}"
NODES="${NODES:-}"

# SSH 连接
SSH_USER="${SSH_USER:-root}"
SSH_OPTS="${SSH_OPTS:--o StrictHostKeyChecking=no}"

# 依赖安装
INSTALL_PREREQS="${INSTALL_PREREQS:-1}"   # 1: 安装运行依赖
INSTALL_GO="${INSTALL_GO:-0}"             # 1: 额外安装 Go
GO_VERSION="${GO_VERSION:-1.22.5}"

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
  echo "提示：请先在控制器侧编译，例如：cd node-agent && go build -o node-agent ." >&2
  exit 2
fi

if [[ "${SSH_USER}" == "root" ]]; then
  REMOTE_SUDO=""
else
  REMOTE_SUDO="sudo"
fi

install_prereqs() {
  local target="$1"
  if [[ "${INSTALL_PREREQS}" != "1" ]]; then
    return 0
  fi

  echo "==> [${target}] 安装运行依赖"
  ssh ${SSH_OPTS} "${target}" "${REMOTE_SUDO} bash -lc '
set -euo pipefail
if ! command -v apt-get >/dev/null 2>&1; then
  echo "apt-get 不存在，当前脚本仅支持 Ubuntu/Debian" >&2
  exit 2
fi
export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y --no-install-recommends ca-certificates curl jq procps pciutils
'"

  if [[ "${INSTALL_GO}" == "1" ]]; then
    echo "==> [${target}] 检查/安装 Go ${GO_VERSION}"
    ssh ${SSH_OPTS} "${target}" "${REMOTE_SUDO} bash -lc '
set -euo pipefail
if command -v go >/dev/null 2>&1; then
  exit 0
fi
cd /tmp
rm -f go.tgz
curl -fL "https://mirrors.tuna.tsinghua.edu.cn/golang/go${GO_VERSION}.linux-amd64.tar.gz" -o go.tgz \
|| curl -fL "https://mirrors.aliyun.com/golang/go${GO_VERSION}.linux-amd64.tar.gz" -o go.tgz \
|| curl -fL "https://mirrors.cloud.tencent.com/golang/go${GO_VERSION}.linux-amd64.tar.gz" -o go.tgz
rm -rf /usr/local/go
tar -C /usr/local -xzf /tmp/go.tgz
ln -sf /usr/local/go/bin/go /usr/local/bin/go
go version
'"
  fi
}

for item in ${NODES}; do
  node_id="${item}"
  host="${item}"
  if [[ "${item}" == *:* ]]; then
    node_id="${item%%:*}"
    host="${item#*:}"
  fi
  target="${SSH_USER}@${host}"

  echo "==> 部署到 ${target}（NODE_ID=${node_id}）"

  install_prereqs "${target}"

  scp ${SSH_OPTS} "${AGENT_BIN}" "${target}:/tmp/node-agent"
  ssh ${SSH_OPTS} "${target}" "${REMOTE_SUDO} mv /tmp/node-agent /usr/local/bin/node-agent"
  ssh ${SSH_OPTS} "${target}" "${REMOTE_SUDO} chmod +x /usr/local/bin/node-agent"

  ssh ${SSH_OPTS} "${target}" "${REMOTE_SUDO} mkdir -p /etc/systemd/system"
  ssh ${SSH_OPTS} "${target}" "${REMOTE_SUDO} bash -lc 'cat > /etc/systemd/system/gpu-node-agent.service <<SVC
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
SVC'"

  ssh ${SSH_OPTS} "${target}" "${REMOTE_SUDO} systemctl daemon-reload && ${REMOTE_SUDO} systemctl enable gpu-node-agent && ${REMOTE_SUDO} systemctl restart gpu-node-agent"

  if [[ "${ENABLE_SSH_GUARD}" == "1" ]]; then
    echo "==> 安装 SSH 登录拦截（仅允许已登记用户；排除：${SSH_GUARD_EXCLUDE_USERS}）"
    ssh ${SSH_OPTS} "${target}" "${REMOTE_SUDO} mkdir -p /opt/gpu-cluster /etc/gpu-cluster /var/lib/gpu-cluster /etc/systemd/system"

    ssh ${SSH_OPTS} "${target}" "${REMOTE_SUDO} bash -lc 'cat > /etc/gpu-cluster/ssh_guard.conf <<CONF
CONTROLLER_URL=\"${CONTROLLER_URL}\"
NODE_ID=\"${node_id}\"
EXCLUDE_USERS=\"${SSH_GUARD_EXCLUDE_USERS}\"
FAIL_OPEN=\"${SSH_GUARD_FAIL_OPEN}\"
ALLOWLIST_FILE=\"/var/lib/gpu-cluster/registered_users.txt\"
CONF'"

    ssh ${SSH_OPTS} "${target}" "${REMOTE_SUDO} bash -lc 'cat > /opt/gpu-cluster/sync_registered_users.sh <<\"EOF2\"
#!/bin/bash
set -euo pipefail

CONF=\"/etc/gpu-cluster/ssh_guard.conf\"
if [[ -f \"${CONF}\" ]]; then
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
EOF2'"
    ssh ${SSH_OPTS} "${target}" "${REMOTE_SUDO} chmod +x /opt/gpu-cluster/sync_registered_users.sh"

    ssh ${SSH_OPTS} "${target}" "${REMOTE_SUDO} bash -lc 'cat > /opt/gpu-cluster/ssh_login_check.sh <<\"EOF2\"
#!/bin/bash
set -euo pipefail

CONF=\"/etc/gpu-cluster/ssh_guard.conf\"
if [[ -f \"${CONF}\" ]]; then
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
EOF2'"
    ssh ${SSH_OPTS} "${target}" "${REMOTE_SUDO} chmod +x /opt/gpu-cluster/ssh_login_check.sh"

    ssh ${SSH_OPTS} "${target}" "${REMOTE_SUDO} bash -lc 'cat > /etc/systemd/system/gpu-ssh-guard-sync.service <<\"EOF2\"
[Unit]
Description=GPU SSH Guard Allowlist Sync
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
ExecStart=/opt/gpu-cluster/sync_registered_users.sh
EOF2'"

    ssh ${SSH_OPTS} "${target}" "${REMOTE_SUDO} bash -lc 'cat > /etc/systemd/system/gpu-ssh-guard-sync.timer <<\"EOF2\"
[Unit]
Description=GPU SSH Guard Allowlist Sync Timer

[Timer]
OnBootSec=30
OnUnitActiveSec=2min
Unit=gpu-ssh-guard-sync.service

[Install]
WantedBy=timers.target
EOF2'"

    ssh ${SSH_OPTS} "${target}" "${REMOTE_SUDO} systemctl daemon-reload && ${REMOTE_SUDO} systemctl enable --now gpu-ssh-guard-sync.timer && ${REMOTE_SUDO} systemctl start gpu-ssh-guard-sync.service || true"
    ssh ${SSH_OPTS} "${target}" "${REMOTE_SUDO} bash -lc 'if [[ -f /etc/pam.d/sshd ]] && ! grep -q \"/opt/gpu-cluster/ssh_login_check.sh\" /etc/pam.d/sshd; then echo \"account required pam_exec.so quiet /opt/gpu-cluster/ssh_login_check.sh\" >> /etc/pam.d/sshd; fi'"
  fi

done
