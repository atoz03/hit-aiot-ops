<template>
  <el-card>
    <template #header>
      <div class="row">
        <div>
          <div class="title">我的余额</div>
          <div class="sub">登录态自动识别账号</div>
        </div>
        <el-button :loading="loading" type="primary" @click="query">刷新</el-button>
      </div>
    </template>

    <el-alert v-if="error" :title="error" type="error" show-icon />

    <el-descriptions v-if="resp" :column="2" border>
      <el-descriptions-item label="用户名">{{ resp.username }}</el-descriptions-item>
      <el-descriptions-item label="余额">{{ resp.balance }}</el-descriptions-item>
      <el-descriptions-item label="状态">
        <el-tag :type="tagType(resp.status)">{{ resp.status }}</el-tag>
      </el-descriptions-item>
    </el-descriptions>
  </el-card>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { ApiClient, type BalanceResp } from "../../lib/api";
import { settingsState } from "../../lib/settingsStore";

const loading = ref(false);
const error = ref("");
const resp = ref<BalanceResp | null>(null);

function tagType(status: string) {
  if (status === "normal") return "success";
  if (status === "warning") return "warning";
  if (status === "limited") return "danger";
  if (status === "blocked") return "danger";
  return "info";
}

async function query() {
  loading.value = true;
  error.value = "";
  try {
    const client = new ApiClient(settingsState.baseUrl);
    resp.value = await client.userMyBalance();
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
