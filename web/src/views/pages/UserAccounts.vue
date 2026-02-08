<template>
  <el-card>
    <template #header>
      <div class="head"><span>我的服务器账号映射</span><el-button :loading="loading" type="primary" @click="reload">刷新</el-button></div>
    </template>
    <el-alert v-if="error" :title="error" type="error" show-icon class="mb" />
    <el-form inline>
      <el-form-item label="机器编号"><el-input v-model="nodeId" placeholder="如 60000" /></el-form-item>
      <el-form-item label="机器用户名"><el-input v-model="localUsername" placeholder="如 alice" /></el-form-item>
      <el-form-item><el-button type="primary" @click="add">新增/覆盖</el-button></el-form-item>
    </el-form>
    <el-table :data="rows" stripe>
      <el-table-column prop="node_id" label="机器编号" width="140" />
      <el-table-column prop="local_username" label="机器用户名" width="180" />
      <el-table-column prop="billing_username" label="计费账号" width="180" />
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

const loading = ref(false);
const error = ref("");
const rows = ref<UserNodeAccount[]>([]);
const nodeId = ref("");
const localUsername = ref("");
const editOldNode = ref("");
const editOldUser = ref("");

async function reload() {
  loading.value = true;
  error.value = "";
  try {
    const client = new ApiClient(settingsState.baseUrl);
    const r = await client.userAccounts();
    rows.value = r.accounts ?? [];
  } catch (e: any) {
    error.value = e?.body ? `${e.message}\n${e.body}` : (e?.message ?? String(e));
  } finally {
    loading.value = false;
  }
}

function prefill(row: UserNodeAccount) {
  editOldNode.value = row.node_id;
  editOldUser.value = row.local_username;
  nodeId.value = row.node_id;
  localUsername.value = row.local_username;
}

async function add() {
  error.value = "";
  try {
    const client = new ApiClient(settingsState.baseUrl);
    if (editOldNode.value && editOldUser.value) {
      await client.userUpdateAccount({
        old_node_id: editOldNode.value,
        old_local_username: editOldUser.value,
        new_node_id: nodeId.value.trim(),
        new_local_username: localUsername.value.trim(),
      });
      editOldNode.value = "";
      editOldUser.value = "";
    } else {
      await client.userUpsertAccount(nodeId.value.trim(), localUsername.value.trim());
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
    const client = new ApiClient(settingsState.baseUrl);
    await client.userDeleteAccount(row.node_id, row.local_username);
    await reload();
  } catch (e: any) {
    error.value = e?.body ? `${e.message}\n${e.body}` : (e?.message ?? String(e));
  }
}

reload();
</script>

<style scoped>
.head { display: flex; justify-content: space-between; align-items: center; }
.mb { margin-bottom: 12px; }
</style>
