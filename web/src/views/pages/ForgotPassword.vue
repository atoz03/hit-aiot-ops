<template>
  <div class="page-wrap">
    <el-card class="card">
      <template #header><h2>找回密码</h2></template>
      <el-alert v-if="error" :title="error" type="error" show-icon class="mb" />
      <el-alert v-if="success" :title="success" type="success" show-icon class="mb" />
      <el-form label-position="top">
        <el-form-item label="注册邮箱">
          <el-input v-model="email" placeholder="name@example.com" />
        </el-form-item>
      </el-form>
      <el-button type="primary" :loading="loading" @click="submit" style="width: 100%">发送重置邮件</el-button>
      <div class="links"><router-link to="/login">返回登录</router-link></div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { ApiClient } from "../../lib/api";
import { settingsState } from "../../lib/settingsStore";

const loading = ref(false);
const error = ref("");
const success = ref("");
const email = ref("");

async function submit() {
  error.value = "";
  success.value = "";
  loading.value = true;
  try {
    const client = new ApiClient(settingsState.baseUrl);
    await client.authForgotPassword(email.value.trim());
    success.value = "如果邮箱存在，系统会发送重置链接，请检查收件箱。";
  } catch (e: any) {
    error.value = e?.body ? `${e.message}\n${e.body}` : (e?.message ?? String(e));
  } finally {
    loading.value = false;
  }
}
</script>

<style scoped>
.page-wrap { min-height: 100vh; display: grid; place-items: center; background: #eef4ff; }
.card { width: 92%; max-width: 480px; }
.mb { margin-bottom: 12px; }
.links { margin-top: 12px; }
.links a { color: #1d4ed8; text-decoration: none; }
</style>
