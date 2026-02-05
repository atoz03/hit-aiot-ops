#!/bin/bash
# 批量部署 Bash Hook 到所有用户（示例脚本）

set -euo pipefail

HOOK_SRC="${HOOK_SRC:-./tools/check_quota.sh}"
HOOK_DST_DIR="${HOOK_DST_DIR:-/opt/gpu-cluster}"
HOOK_DST="${HOOK_DST_DIR}/check_quota.sh"

if [[ ! -f "${HOOK_SRC}" ]]; then
  echo "未找到 Hook 脚本：${HOOK_SRC}" >&2
  exit 2
fi

sudo mkdir -p "${HOOK_DST_DIR}"
sudo cp "${HOOK_SRC}" "${HOOK_DST}"
sudo chmod +x "${HOOK_DST}"

for userHome in /home/*; do
  if [[ -d "${userHome}" ]]; then
    bashrc="${userHome}/.bashrc"
    if ! grep -q "${HOOK_DST}" "${bashrc}" 2>/dev/null; then
      echo "source ${HOOK_DST}" | sudo tee -a "${bashrc}" >/dev/null
    fi
  fi
done

echo "Hook 已部署：${HOOK_DST}"

