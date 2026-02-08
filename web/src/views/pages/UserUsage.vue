<template>
  <el-card>
    <template #header>
      <div class="row">
        <div>
          <div class="title">我的用量</div>
          <div class="sub">仅展示当前登录账号的账单记录</div>
        </div>
        <el-button :loading="loading" type="primary" @click="query">刷新</el-button>
      </div>
    </template>

    <el-alert v-if="error" :title="error" type="error" show-icon />

    <el-form inline>
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
  </el-card>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { ApiClient, type UsageRecord } from "../../lib/api";
import { settingsState } from "../../lib/settingsStore";

const loading = ref(false);
const error = ref("");
const records = ref<UsageRecord[]>([]);
const limit = ref(200);

async function query() {
  loading.value = true;
  error.value = "";
  try {
    const client = new ApiClient(settingsState.baseUrl);
    const r = await client.userMyUsage(limit.value);
    records.value = r.records ?? [];
  } catch (e: any) {
    error.value = e?.body ? `${e.message}\n${e.body}` : (e?.message ?? String(e));
  } finally {
    loading.value = false;
  }
}

query();
</script>

<style scoped>
.row { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.title { font-weight: 700; }
.sub { margin-top: 4px; font-size: 12px; color: #64748b; }
</style>
