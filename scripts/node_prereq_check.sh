#!/bin/bash
# 计算节点上线前置检查（不修改系统）

set -euo pipefail

echo "==> 基本信息"
echo "hostname: $(hostname)"
echo "user: $(id -un) uid=$(id -u)"
if [[ -f /etc/os-release ]]; then
  . /etc/os-release
  echo "os: ${PRETTY_NAME:-unknown}"
fi

echo
echo "==> systemd"
if [[ -d /run/systemd/system ]]; then
  echo "systemd: yes"
else
  echo "systemd: no"
fi

echo
echo "==> 常用依赖"
for c in curl jq go sudo; do
  if command -v "${c}" >/dev/null 2>&1; then
    echo "${c}: yes ($(command -v "${c}"))"
  else
    echo "${c}: no"
  fi
done

echo
echo "==> GPU / nvidia-smi"
if command -v nvidia-smi >/dev/null 2>&1; then
  echo "nvidia-smi: yes"
  nvidia-smi -L 2>/dev/null | head -n 5 || true
else
  echo "nvidia-smi: no（无 GPU 或未安装驱动）"
fi

echo
echo "==> cgroup 版本"
if [[ -f /sys/fs/cgroup/cgroup.controllers ]]; then
  echo "cgroup: v2"
else
  echo "cgroup: v1 或混合（未检测到 /sys/fs/cgroup/cgroup.controllers）"
fi

echo
echo "==> cgroup v2 关键文件"
found_v2=""
for g in "/sys/fs/cgroup/user.slice/user-*.slice/cpu.max" "/sys/fs/cgroup/user-*.slice/cpu.max"; do
  if compgen -G "$g" >/dev/null; then
    found_v2="$(compgen -G "$g" | head -n 1)"
    break
  fi
done
if [[ -n "${found_v2}" ]]; then
  echo "found: ${found_v2}"
else
  echo "not found"
fi

echo
echo "==> cgroup v1 cpu controller"
if [[ -r /proc/mounts ]]; then
  if awk '{print $3,$2,$4}' /proc/mounts | grep -E '^cgroup ' | grep -Eq ',cpu(,|$)'; then
    echo "cgroup v1 cpu: mounted"
    awk '{print $3,$2,$4}' /proc/mounts | grep -E '^cgroup ' | grep ',cpu' | head -n 3 || true
  else
    echo "cgroup v1 cpu: not found"
  fi
else
  echo "/proc/mounts 不可读"
fi

echo
echo "==> 建议"
echo "- 若需要 CPU 限流：推荐 systemd；否则至少保证 cgroup v2 或 cgroup v1(cpu) 可用"
echo "- Agent 需要 root 才能执行 kill / CPU 限流动作"
