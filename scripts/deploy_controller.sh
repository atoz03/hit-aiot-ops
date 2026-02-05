#!/bin/bash
# 批量/单机部署 controller（示例脚本）
#
# 说明：
# - 需要你自行准备服务器地址与 SSH 权限
# - 该脚本不会删除任何远端数据，仅覆盖二进制与配置文件

set -euo pipefail

CONTROLLER_BIN="${CONTROLLER_BIN:-./controller/controller}"
CONFIG_SRC="${CONFIG_SRC:-./config/controller.yaml}"
HOST="${HOST:-}"
DEST_DIR="${DEST_DIR:-/opt/gpu-controller}"

if [[ -z "${HOST}" ]]; then
  echo "请设置环境变量 HOST，例如：HOST=controller-host" >&2
  exit 2
fi
if [[ ! -f "${CONTROLLER_BIN}" ]]; then
  echo "未找到控制器二进制：${CONTROLLER_BIN}（请先在 controller/ 下 go build）" >&2
  exit 2
fi
if [[ ! -f "${CONFIG_SRC}" ]]; then
  echo "未找到配置文件：${CONFIG_SRC}" >&2
  exit 2
fi

echo "==> 创建目录 ${DEST_DIR}"
ssh "root@${HOST}" "mkdir -p \"${DEST_DIR}\""

echo "==> 上传二进制与配置"
scp "${CONTROLLER_BIN}" "root@${HOST}:${DEST_DIR}/controller"
scp "${CONFIG_SRC}" "root@${HOST}:${DEST_DIR}/controller.yaml"

echo "==> 安装 systemd unit"
scp "./systemd/gpu-controller.service" "root@${HOST}:/etc/systemd/system/gpu-controller.service"

echo "==> 启动服务"
ssh "root@${HOST}" "chmod +x \"${DEST_DIR}/controller\" && systemctl daemon-reload && systemctl enable gpu-controller && systemctl restart gpu-controller"

echo "部署完成：${HOST}"

