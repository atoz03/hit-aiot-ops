<template>
  <el-card>
    <template #header>
      <div class="row">
        <div>
          <div class="title">注册 / 开号申请审核</div>
          <div class="sub">需要管理员登录：GET /api/admin/requests，POST /api/admin/requests/:id/approve|reject</div>
        </div>
        <div class="row">
          <el-select v-model="status" style="width: 160px" @change="reload">
            <el-option label="待审核" value="pending" />
            <el-option label="已通过" value="approved" />
            <el-option label="已拒绝" value="rejected" />
            <el-option label="全部" value="" />
          </el-select>
          <el-button :loading="loading" type="primary" @click="reload">刷新</el-button>
        </div>
      </div>
    </template>

    <el-space direction="vertical" fill style="width: 100%">
      <el-alert v-if="error" :title="error" type="error" show-icon />

      <el-table :data="rows" stripe height="560">
        <el-table-column prop="request_id" label="ID" width="80" />
        <el-table-column prop="request_type" label="类型" width="100" />
        <el-table-column prop="billing_username" label="计费账号" width="160" />
        <el-table-column prop="node_id" label="端口" width="110" />
        <el-table-column prop="local_username" label="机器用户名" width="160" />
        <el-table-column prop="status" label="状态" width="120" />
        <el-table-column prop="created_at" label="提交时间" min-width="180" />
        <el-table-column prop="message" label="备注" min-width="220" />
        <el-table-column label="操作" width="220" fixed="right">
          <template #default="{ row }">
            <el-space>
              <el-button
                size="small"
                type="success"
                :disabled="row.status !== 'pending'"
                :loading="actionLoadingId === row.request_id"
                @click="approve(row.request_id)"
              >
                通过
              </el-button>
              <el-button
                size="small"
                type="danger"
                :disabled="row.status !== 'pending'"
                :loading="actionLoadingId === row.request_id"
                @click="reject(row.request_id)"
              >
                拒绝
              </el-button>
            </el-space>
          </template>
        </el-table-column>
      </el-table>
    </el-space>
  </el-card>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { ApiClient, type UserRequest } from "../../lib/api";
import { settingsState } from "../../lib/settingsStore";
import { authState } from "../../lib/authStore";

const loading = ref(false);
const error = ref("");
const status = ref("pending");
const rows = ref<UserRequest[]>([]);
const actionLoadingId = ref<number | null>(null);

async function reload() {
  loading.value = true;
  error.value = "";
  rows.value = [];
  try {
    const client = new ApiClient(settingsState.baseUrl, { csrfToken: authState.csrfToken });
    const r = await client.adminRequests({ status: status.value, limit: 500 });
    rows.value = r.requests ?? [];
  } catch (e: any) {
    error.value = e?.body ? `${e.message}\n${e.body}` : (e?.message ?? String(e));
  } finally {
    loading.value = false;
  }
}

async function approve(id: number) {
  actionLoadingId.value = id;
  error.value = "";
  try {
    const client = new ApiClient(settingsState.baseUrl, { csrfToken: authState.csrfToken });
    await client.adminApproveRequest(id);
    await reload();
  } catch (e: any) {
    error.value = e?.body ? `${e.message}\n${e.body}` : (e?.message ?? String(e));
  } finally {
    actionLoadingId.value = null;
  }
}

async function reject(id: number) {
  actionLoadingId.value = id;
  error.value = "";
  try {
    const client = new ApiClient(settingsState.baseUrl, { csrfToken: authState.csrfToken });
    await client.adminRejectRequest(id);
    await reload();
  } catch (e: any) {
    error.value = e?.body ? `${e.message}\n${e.body}` : (e?.message ?? String(e));
  } finally {
    actionLoadingId.value = null;
  }
}

reload();
</script>

<style scoped>
.row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.title {
  font-weight: 700;
}
.sub {
  margin-top: 4px;
  font-size: 12px;
  color: #6b7280;
}
</style>

