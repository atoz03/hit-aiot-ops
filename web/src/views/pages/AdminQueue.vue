<template>
  <el-card>
    <template #header>
      <div class="row">
        <div>
          <div class="title">排队队列</div>
          <div class="sub">需要管理员登录：GET /api/admin/gpu/queue</div>
        </div>
        <div class="row">
          <el-button :loading="loading" type="primary" @click="reload">刷新</el-button>
        </div>
      </div>
    </template>

    <el-space direction="vertical" fill style="width: 100%">
      <el-alert v-if="error" :title="error" type="error" show-icon />

      <el-table :data="rows" stripe height="520">
        <el-table-column prop="username" label="用户" width="180" />
        <el-table-column prop="gpu_type" label="GPU类型" width="180" />
        <el-table-column prop="count" label="数量" width="100" />
        <el-table-column prop="timestamp" label="时间" />
      </el-table>
    </el-space>
  </el-card>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { ApiClient } from "../../lib/api";
import { settingsState } from "../../lib/settingsStore";
import { authState } from "../../lib/authStore";

const loading = ref(false);
const error = ref("");
const rows = ref<Array<{ username: string; gpu_type: string; count: number; timestamp: string }>>([]);

async function reload() {
  loading.value = true;
  error.value = "";
  rows.value = [];
  try {
    const client = new ApiClient(settingsState.baseUrl, { csrfToken: authState.csrfToken });
    const r = await client.adminQueue();
    rows.value = r.queue ?? [];
  } catch (e: any) {
    error.value = e?.body ? `${e.message}\n${e.body}` : (e?.message ?? String(e));
  } finally {
    loading.value = false;
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
