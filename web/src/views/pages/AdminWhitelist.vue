<template>
  <el-card>
    <template #header>
      <div class="head"><span>SSH 白名单</span><el-button :loading="loading" type="primary" @click="reload">刷新</el-button></div>
    </template>
    <el-alert v-if="error" :title="error" type="error" show-icon class="mb" />
    <el-alert title="node_id 填 * 表示所有机器生效。用户名可一次输入多个（逗号分隔）。" type="info" show-icon class="mb" />
    <el-form inline>
      <el-form-item label="机器编号">
        <el-select v-model="nodeId" filterable style="width: 220px">
          <el-option label="所有机器 (*)" value="*" />
          <el-option v-for="id in nodeOptions" :key="id" :label="id" :value="id" />
        </el-select>
      </el-form-item>
      <el-form-item label="用户名列表">
        <el-input v-model="usernamesText" placeholder="alice,bob,charlie" style="width: 360px" />
      </el-form-item>
      <el-form-item><el-button type="primary" @click="save">新增/覆盖</el-button></el-form-item>
    </el-form>

    <el-form inline>
      <el-form-item label="按机器筛选">
        <el-input v-model="filterNode" placeholder="留空全部" />
      </el-form-item>
      <el-form-item><el-button @click="reload">查询</el-button></el-form-item>
    </el-form>

    <el-table :data="rows" stripe>
      <el-table-column prop="node_id" label="机器编号" width="120" />
      <el-table-column prop="local_username" label="用户名" width="180" />
      <el-table-column prop="created_by" label="创建人" width="160" />
      <el-table-column prop="updated_at" label="更新时间" min-width="180" />
      <el-table-column label="操作" width="120">
        <template #default="{ row }">
          <el-button size="small" type="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>
  </el-card>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { ApiClient, type SSHWhitelistEntry } from "../../lib/api";
import { settingsState } from "../../lib/settingsStore";
import { authState } from "../../lib/authStore";

const loading = ref(false);
const error = ref("");
const rows = ref<SSHWhitelistEntry[]>([]);
const nodeOptions = ref<string[]>([]);

const nodeId = ref("*");
const usernamesText = ref("");
const filterNode = ref("");

async function reload() {
  loading.value = true;
  error.value = "";
  try {
    const client = new ApiClient(settingsState.baseUrl, { csrfToken: authState.csrfToken });
    const r = await client.adminWhitelist(filterNode.value.trim());
    rows.value = r.entries ?? [];
  } catch (e: any) {
    error.value = e?.body ? `${e.message}\n${e.body}` : (e?.message ?? String(e));
  } finally {
    loading.value = false;
  }
}

async function loadNodes() {
  try {
    const client = new ApiClient(settingsState.baseUrl, { csrfToken: authState.csrfToken });
    const r = await client.adminNodes(2000);
    nodeOptions.value = (r.nodes ?? []).map((x) => x.node_id).filter(Boolean);
  } catch {
    nodeOptions.value = [];
  }
}

async function save() {
  error.value = "";
  const names = usernamesText.value.split(",").map((x) => x.trim()).filter(Boolean);
  if (names.length === 0) {
    error.value = "请至少输入一个用户名";
    return;
  }
  try {
    const client = new ApiClient(settingsState.baseUrl, { csrfToken: authState.csrfToken });
    await client.adminUpsertWhitelist(nodeId.value.trim(), names);
    usernamesText.value = "";
    await reload();
  } catch (e: any) {
    error.value = e?.body ? `${e.message}\n${e.body}` : (e?.message ?? String(e));
  }
}

async function remove(row: SSHWhitelistEntry) {
  error.value = "";
  try {
    const client = new ApiClient(settingsState.baseUrl, { csrfToken: authState.csrfToken });
    await client.adminDeleteWhitelist(row.node_id, row.local_username);
    await reload();
  } catch (e: any) {
    error.value = e?.body ? `${e.message}\n${e.body}` : (e?.message ?? String(e));
  }
}

reload();
loadNodes();
</script>

<style scoped>
.head { display: flex; justify-content: space-between; align-items: center; }
.mb { margin-bottom: 12px; }
</style>
