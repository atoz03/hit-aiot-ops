<template>
  <div class="page-wrap">
    <el-card class="card">
      <template #header><h2>重置密码</h2></template>
      <el-alert v-if="error" :title="error" type="error" show-icon class="mb" />
      <el-alert v-if="success" :title="success" type="success" show-icon class="mb" />
      <el-form label-position="top">
        <el-form-item label="用户名"><el-input v-model="username" /></el-form-item>
        <el-form-item label="重置令牌"><el-input v-model="token" /></el-form-item>
        <el-form-item label="新密码"><el-input v-model="newPassword" type="password" show-password /></el-form-item>
      </el-form>
      <el-button type="primary" :loading="loading" @click="submit" style="width: 100%">确认重置</el-button>
      <div class="links"><router-link to="/login">返回登录</router-link></div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { useRoute } from "vue-router";
import { ApiClient } from "../../lib/api";
import { settingsState } from "../../lib/settingsStore";

const route = useRoute();
const loading = ref(false);
const error = ref("");
const success = ref("");
const username = ref(String(route.query.username ?? ""));
const token = ref(String(route.query.token ?? ""));
const newPassword = ref("");

async function submit() {
  error.value = "";
  success.value = "";
  loading.value = true;
  try {
    const client = new ApiClient(settingsState.baseUrl);
    await client.authResetPassword({ username: username.value.trim(), token: token.value.trim(), new_password: newPassword.value });
    success.value = "密码重置成功，请使用新密码登录。";
  } catch (e: any) {
    error.value = e?.body ? `${e.message}\n${e.body}` : (e?.message ?? String(e));
  } finally {
    loading.value = false;
  }
}
</script>

<style scoped>
.page-wrap { min-height: 100vh; display: grid; place-items: center; background: #eef4ff; }
.card { width: 92%; max-width: 520px; }
.mb { margin-bottom: 12px; }
.links { margin-top: 12px; }
.links a { color: #1d4ed8; text-decoration: none; }
</style>
