<template>
  <el-card>
    <template #header>
      <div class="row">
        <div>
          <div class="title">用户余额</div>
          <div class="sub">接口：GET /api/users/:username/balance</div>
        </div>
        <el-button :loading="loading" type="primary" @click="query">查询</el-button>
      </div>
    </template>

    <el-space direction="vertical" fill style="width: 100%">
      <el-alert v-if="error" :title="error" type="error" show-icon />

      <el-form label-width="90px">
        <el-form-item label="用户名">
          <el-input v-model="username" placeholder="例如 alice" @keyup.enter="query" />
        </el-form-item>
      </el-form>

      <el-descriptions v-if="resp" :column="2" border>
        <el-descriptions-item label="用户名">{{ resp.username }}</el-descriptions-item>
        <el-descriptions-item label="余额">{{ resp.balance }}</el-descriptions-item>
        <el-descriptions-item label="状态">
          <el-tag :type="tagType(resp.status)">{{ resp.status }}</el-tag>
        </el-descriptions-item>
      </el-descriptions>
    </el-space>
  </el-card>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { ApiClient, type BalanceResp } from "../../lib/api";
import { loadSettings, saveSettings } from "../../lib/settings";

const loading = ref(false);
const error = ref("");
const resp = ref<BalanceResp | null>(null);

const settings = loadSettings();
const username = ref(settings.defaultUsername ?? "");

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
  resp.value = null;
  try {
    const client = new ApiClient(settings.baseUrl, settings.adminToken);
    const r = await client.userBalance(username.value.trim());
    resp.value = r;
    saveSettings({ ...settings, defaultUsername: username.value.trim() });
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

