<template>
  <el-card>
    <template #header>
      <div class="head"><span>账号映射管理（管理员）</span><el-button :loading="loading" type="primary" @click="reload">刷新</el-button></div>
    </template>
    <el-alert v-if="error" :title="error" type="error" show-icon class="mb" />
    <el-form inline>
      <el-form-item label="计费账号"><el-input v-model="billing" placeholder="alice" /></el-form-item>
      <el-form-item label="机器编号"><el-input v-model="nodeId" placeholder="60000" /></el-form-item>
      <el-form-item label="机器用户名"><el-input v-model="localUsername" placeholder="alice" /></el-form-item>
      <el-form-item><el-button type="primary" @click="save">新增/覆盖</el-button></el-form-item>
    </el-form>
    <el-table :data="rows" stripe>
      <el-table-column prop="billing_username" label="计费账号" width="170" />
      <el-table-column prop="node_id" label="机器编号" width="130" />
      <el-table-column prop="local_username" label="机器用户名" width="170" />
      <el-table-column prop="updated_at" label="更新时间" min-width="180" />
      <el-table-column label="操作" width="220">
        <template #default="{ row }">
          <el-button size="small" @click="prefill(row)">编辑</el-button>
          <el-button size="small" type="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>
  </el-card>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { ApiClient, type UserNodeAccount } from "../../lib/api";
import { settingsState } from "../../lib/settingsStore";
import { authState } from "../../lib/authStore";

const loading = ref(false);
const error = ref("");
const rows = ref<UserNodeAccount[]>([]);
const billing = ref("");
const nodeId = ref("");
const localUsername = ref("");
const old = ref<{ billing: string; node: string; local: string } | null>(null);

async function reload() {
  if (!billing.value.trim()) return;
  loading.value = true;
  error.value = "";
  try {
    const client = new ApiClient(settingsState.baseUrl, { csrfToken: authState.csrfToken });
    const r = await client.adminAccounts(billing.value.trim());
    rows.value = r.accounts ?? [];
  } catch (e: any) {
    error.value = e?.body ? `${e.message}\n${e.body}` : (e?.message ?? String(e));
  } finally {
    loading.value = false;
  }
}

function prefill(row: UserNodeAccount) {
  old.value = { billing: row.billing_username, node: row.node_id, local: row.local_username };
  billing.value = row.billing_username;
  nodeId.value = row.node_id;
  localUsername.value = row.local_username;
}

async function save() {
  error.value = "";
  try {
    const client = new ApiClient(settingsState.baseUrl, { csrfToken: authState.csrfToken });
    if (old.value) {
      await client.adminUpdateAccount({
        old_billing_username: old.value.billing,
        old_node_id: old.value.node,
        old_local_username: old.value.local,
        new_billing_username: billing.value.trim(),
        new_node_id: nodeId.value.trim(),
        new_local_username: localUsername.value.trim(),
      });
      old.value = null;
    } else {
      await client.adminUpsertAccount({
        billing_username: billing.value.trim(),
        node_id: nodeId.value.trim(),
        local_username: localUsername.value.trim(),
      });
    }
    nodeId.value = "";
    localUsername.value = "";
    await reload();
  } catch (e: any) {
    error.value = e?.body ? `${e.message}\n${e.body}` : (e?.message ?? String(e));
  }
}

async function remove(row: UserNodeAccount) {
  error.value = "";
  try {
    const client = new ApiClient(settingsState.baseUrl, { csrfToken: authState.csrfToken });
    await client.adminDeleteAccount({
      billing_username: row.billing_username,
      node_id: row.node_id,
      local_username: row.local_username,
    });
    await reload();
  } catch (e: any) {
    error.value = e?.body ? `${e.message}\n${e.body}` : (e?.message ?? String(e));
  }
}
</script>

<style scoped>
.head { display: flex; justify-content: space-between; align-items: center; }
.mb { margin-bottom: 12px; }
</style>
