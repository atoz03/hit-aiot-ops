<template>
  <el-card>
    <template #header>
      <div class="row">
        <div>
          <div class="title">使用记录</div>
          <div class="sub">需要管理员登录：GET /api/admin/usage，GET /api/admin/usage/export.csv</div>
        </div>
        <div class="row">
          <el-button :loading="loading" type="primary" @click="reload">刷新</el-button>
          <el-button :loading="exporting" @click="exportCSV">导出 CSV</el-button>
        </div>
      </div>
    </template>

    <el-space direction="vertical" fill style="width: 100%">
      <el-alert v-if="error" :title="error" type="error" show-icon />

      <el-form inline>
        <el-form-item label="用户名">
          <el-input v-model="username" placeholder="可留空" @keyup.enter="reload" />
        </el-form-item>
        <el-form-item label="条数">
          <el-input-number v-model="limit" :min="1" :max="5000" />
        </el-form-item>
        <el-form-item label="导出From">
          <el-input v-model="from" placeholder="YYYY-MM-DD 或 RFC3339" />
        </el-form-item>
        <el-form-item label="导出To">
          <el-input v-model="to" placeholder="YYYY-MM-DD 或 RFC3339" />
        </el-form-item>
      </el-form>

      <el-table :data="records" stripe height="520">
        <el-table-column prop="timestamp" label="时间" width="190" />
        <el-table-column prop="node_id" label="节点" width="120" />
        <el-table-column prop="username" label="用户" width="160" />
        <el-table-column prop="cpu_percent" label="CPU%" width="90" />
        <el-table-column prop="memory_mb" label="内存MB" width="110" />
        <el-table-column prop="cost" label="费用" width="90" />
        <el-table-column prop="gpu_usage" label="GPU明细(JSON)" />
      </el-table>
    </el-space>
  </el-card>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { ElMessage } from "element-plus";
import { ApiClient, type UsageRecord } from "../../lib/api";
import { settingsState } from "../../lib/settingsStore";
import { authState } from "../../lib/authStore";

const loading = ref(false);
const exporting = ref(false);
const error = ref("");
const records = ref<UsageRecord[]>([]);

const username = ref("");
const limit = ref(200);
const from = ref("");
const to = ref("");

async function reload() {
  loading.value = true;
  error.value = "";
  records.value = [];
  try {
    const client = new ApiClient(settingsState.baseUrl, { csrfToken: authState.csrfToken });
    const r = await client.adminUsage(username.value, limit.value);
    records.value = r.records ?? [];
  } catch (e: any) {
    error.value = e?.body ? `${e.message}\n${e.body}` : (e?.message ?? String(e));
  } finally {
    loading.value = false;
  }
}

async function exportCSV() {
  exporting.value = true;
  error.value = "";
  try {
    const client = new ApiClient(settingsState.baseUrl, { csrfToken: authState.csrfToken });
    const blob = await client.adminExportUsageCSV({
      username: username.value,
      from: from.value,
      to: to.value,
      limit: 20000,
    });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = "usage_export.csv";
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(url);
    ElMessage.success("已开始下载 usage_export.csv");
  } catch (e: any) {
    error.value = e?.body ? `${e.message}\n${e.body}` : (e?.message ?? String(e));
  } finally {
    exporting.value = false;
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
