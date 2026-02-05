<template>
  <el-card>
    <template #header>
      <div class="row">
        <div>
          <div class="title">用户管理</div>
          <div class="sub">需要管理员 Token：GET /api/admin/users，POST /api/users/:username/recharge</div>
        </div>
        <div class="row">
          <el-button :loading="loading" type="primary" @click="reload">刷新</el-button>
        </div>
      </div>
    </template>

    <el-space direction="vertical" fill style="width: 100%">
      <el-alert v-if="error" :title="error" type="error" show-icon />

      <el-table :data="rows" stripe height="520">
        <el-table-column prop="username" label="用户名" width="220" />
        <el-table-column prop="balance" label="余额" width="140" />
        <el-table-column prop="status" label="状态" width="140" />
        <el-table-column label="操作" width="200">
          <template #default="{ row }">
            <el-button size="small" @click="openRecharge(row.username)">充值</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-space>

    <el-dialog v-model="rechargeVisible" title="充值" width="420px">
      <el-form label-width="90px">
        <el-form-item label="用户名">
          <el-input v-model="rechargeUser" disabled />
        </el-form-item>
        <el-form-item label="金额">
          <el-input-number v-model="rechargeAmount" :min="0.01" :max="100000" :step="10" />
        </el-form-item>
        <el-form-item label="方式">
          <el-input v-model="rechargeMethod" placeholder="admin/wechat/alipay" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="rechargeVisible = false">取消</el-button>
        <el-button :loading="rechargeLoading" type="primary" @click="doRecharge">确认</el-button>
      </template>
    </el-dialog>
  </el-card>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { ApiClient } from "../../lib/api";
import { loadSettings } from "../../lib/settings";

type UserRow = { username: string; balance: number; status: string };

const loading = ref(false);
const error = ref("");
const rows = ref<UserRow[]>([]);

const rechargeVisible = ref(false);
const rechargeLoading = ref(false);
const rechargeUser = ref("");
const rechargeAmount = ref(100);
const rechargeMethod = ref("admin");

const settings = loadSettings();

function toUserRow(u: any): UserRow {
  return {
    username: (u?.username ?? u?.Username ?? "").toString(),
    balance: Number(u?.balance ?? u?.Balance ?? 0),
    status: (u?.status ?? u?.Status ?? "").toString(),
  };
}

async function reload() {
  loading.value = true;
  error.value = "";
  rows.value = [];
  try {
    const client = new ApiClient(settings.baseUrl, settings.adminToken);
    const r = await client.adminUsers();
    rows.value = (r.users ?? []).map(toUserRow);
  } catch (e: any) {
    error.value = e?.body ? `${e.message}\n${e.body}` : (e?.message ?? String(e));
  } finally {
    loading.value = false;
  }
}

function openRecharge(username: string) {
  rechargeUser.value = username;
  rechargeAmount.value = 100;
  rechargeMethod.value = "admin";
  rechargeVisible.value = true;
}

async function doRecharge() {
  rechargeLoading.value = true;
  error.value = "";
  try {
    const client = new ApiClient(settings.baseUrl, settings.adminToken);
    await client.adminRecharge(rechargeUser.value, rechargeAmount.value, rechargeMethod.value);
    rechargeVisible.value = false;
    await reload();
  } catch (e: any) {
    error.value = e?.body ? `${e.message}\n${e.body}` : (e?.message ?? String(e));
  } finally {
    rechargeLoading.value = false;
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

