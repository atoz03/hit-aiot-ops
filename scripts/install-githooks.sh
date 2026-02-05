#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${repo_root}"

if [[ ! -d ".git" ]]; then
  echo "[install-githooks] 当前目录不是 git 仓库根目录：${repo_root}" >&2
  exit 1
fi

if [[ ! -d ".githooks" ]]; then
  echo "[install-githooks] 缺少 .githooks 目录，请先拉取完整仓库内容。" >&2
  exit 1
fi

if [[ ! -f ".githooks/pre-commit" ]]; then
  echo "[install-githooks] 缺少 .githooks/pre-commit，请先拉取完整仓库内容。" >&2
  exit 1
fi

chmod +x ".githooks/pre-commit"

# 让 Git 使用仓库内可版本化的 hooks（团队共享）
git config core.hooksPath ".githooks"

echo "[install-githooks] 已安装 Git hooks：core.hooksPath=.githooks"
echo "[install-githooks] 以后每次 git commit 都会自动运行 go test（失败会阻止提交）。"
echo "[install-githooks] 如需临时跳过：git commit --no-verify"

