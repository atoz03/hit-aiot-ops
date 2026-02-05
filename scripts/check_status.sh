#!/bin/bash
# 系统自检（控制器 + 数据库 + 基本 API）

set -euo pipefail

CONTROLLER_URL="${CONTROLLER_URL:-http://127.0.0.1:8000}"
ADMIN_TOKEN="${ADMIN_TOKEN:-dev-admin-token}"

echo "==> healthz"
curl -fsS "${CONTROLLER_URL}/healthz" && echo

echo "==> metrics"
curl -fsS "${CONTROLLER_URL}/metrics" | head -n 20
echo

echo "==> prices"
curl -fsS -H "Authorization: Bearer ${ADMIN_TOKEN}" "${CONTROLLER_URL}/api/admin/prices" && echo

echo "==> users"
curl -fsS -H "Authorization: Bearer ${ADMIN_TOKEN}" "${CONTROLLER_URL}/api/admin/users" && echo

echo "==> nodes"
curl -fsS -H "Authorization: Bearer ${ADMIN_TOKEN}" "${CONTROLLER_URL}/api/admin/nodes?limit=5" && echo

echo "==> usage"
curl -fsS -H "Authorization: Bearer ${ADMIN_TOKEN}" "${CONTROLLER_URL}/api/admin/usage?limit=5" && echo
