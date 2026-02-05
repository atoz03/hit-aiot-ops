<template>
  <el-card>
    <template #header>
      <div class="row">
        <div>
          <div class="title">用户用量</div>
          <div class="sub">接口：GET /api/users/:username/usage</div>
        </div>
        <div class="row">
          <el-button :loading="loading" type="primary" @click="query">查询</el-button>
        </div>
      </div>
    </template>

    <el-space direction="vertical" fill style="width: 100%">
      <el-alert v-if="error" :title="error" type="error" show-icon />

      <el-form inline>
        <el-form-item label="用户名">
          <el-input v-model="username" placeholder="例如 alice" @keyup.enter="query" />
        </el-form-item>
        <el-form-item label="条数">
          <el-input-number v-model="limit" :min="1" :max="5000" />
        </el-form-item>
      </el-form>

      <el-table :data="records" stripe height="520">
        <el-table-column prop="timestamp" label="时间" width="190" />
        <el-table-column prop="node_id" label="节点" width="120" />
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
import { ApiClient, type UsageRecord } from "../../lib/api";
import { setDefaultUsername, settingsState } from "../../lib/settingsStore";

const loading = ref(false);
const error = ref("");
const records = ref<UsageRecord[]>([]);

const username = ref(settingsState.defaultUsername ?? "");
const limit = ref(200);

async function query() {
  loading.value = true;
  error.value = "";
  records.value = [];
  try {
    const client = new ApiClient(settingsState.baseUrl);
    const r = await client.userUsage(username.value.trim(), limit.value);
    records.value = r.records ?? [];
    setDefaultUsername(username.value.trim());
  } catch (e: any) {
    error.value = e?.body ? `${e.message}\n${e.body}` : (e?.message ?? String(e));
  } finally {
    loading.value = false;
  }
}
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
