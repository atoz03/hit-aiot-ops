<template>
  <el-card>
    <template #header>
      <div class="row">
        <div>
          <div class="title">节点状态</div>
          <div class="sub">需要管理员登录：GET /api/admin/nodes</div>
        </div>
        <div class="row">
          <el-button :loading="loading" type="primary" @click="reload">刷新</el-button>
        </div>
      </div>
    </template>

    <el-space direction="vertical" fill style="width: 100%">
      <el-alert v-if="error" :title="error" type="error" show-icon />

      <el-table :data="rows" stripe height="520">
        <el-table-column prop="node_id" label="节点" width="160" />
        <el-table-column prop="last_seen_at" label="最后在线" width="190" />
        <el-table-column prop="gpu_process_count" label="GPU进程" width="110" />
        <el-table-column prop="cpu_process_count" label="CPU进程" width="110" />
        <el-table-column prop="usage_records_count" label="记录数" width="90" />
        <el-table-column prop="cost_total" label="当次成本" width="110" />
        <el-table-column prop="interval_seconds" label="周期(s)" width="90" />
        <el-table-column prop="last_report_id" label="report_id" />
      </el-table>
    </el-space>
  </el-card>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { ApiClient, type NodeStatus } from "../../lib/api";
import { settingsState } from "../../lib/settingsStore";
import { authState } from "../../lib/authStore";

const loading = ref(false);
const error = ref("");
const rows = ref<NodeStatus[]>([]);

async function reload() {
  loading.value = true;
  error.value = "";
  rows.value = [];
  try {
    const client = new ApiClient(settingsState.baseUrl, { csrfToken: authState.csrfToken });
    const r = await client.adminNodes(200);
    rows.value = r.nodes ?? [];
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
