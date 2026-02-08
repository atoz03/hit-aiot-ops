<template>
  <el-card class="card">
    <template #header>
      <div class="head">
        <el-icon><Key /></el-icon>
        <span>修改密码</span>
      </div>
    </template>

    <el-alert v-if="error" :title="error" type="error" show-icon class="mb" />
    <el-alert v-if="success" :title="success" type="success" show-icon class="mb" />

    <el-form label-position="top" style="max-width: 520px">
      <el-form-item label="当前密码">
        <el-input v-model="currentPassword" type="password" show-password />
      </el-form-item>
      <el-form-item label="新密码">
        <el-input v-model="newPassword" type="password" show-password />
      </el-form-item>
    </el-form>

    <el-button type="primary" :loading="loading" @click="submit">保存</el-button>
  </el-card>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { ApiClient } from "../../lib/api";
import { settingsState } from "../../lib/settingsStore";
import { Key } from "@element-plus/icons-vue";

const loading = ref(false);
const error = ref("");
const success = ref("");
const currentPassword = ref("");
const newPassword = ref("");

async function submit() {
  error.value = "";
  success.value = "";
  loading.value = true;
  try {
    const client = new ApiClient(settingsState.baseUrl);
    await client.authChangePassword(currentPassword.value, newPassword.value);
    success.value = "密码修改成功";
    currentPassword.value = "";
    newPassword.value = "";
  } catch (e: any) {
    error.value = e?.body ? `${e.message}\n${e.body}` : (e?.message ?? String(e));
  } finally {
    loading.value = false;
  }
}
</script>

<style scoped>
.card { max-width: 760px; }
.head { display: flex; align-items: center; gap: 8px; font-weight: 700; }
.mb { margin-bottom: 12px; }
</style>
