<template>
  <el-card style="max-width: 420px; margin: 80px auto">
    <template #header>
      <div class="title">管理员登录</div>
      <div class="sub">使用控制器的管理员账号登录（cookie 会话）</div>
    </template>

    <el-space direction="vertical" fill style="width: 100%">
      <el-alert v-if="error" :title="error" type="error" show-icon />

      <el-form label-width="90px">
        <el-form-item label="用户名">
          <el-input v-model="username" autocomplete="username" @keyup.enter="doLogin" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input v-model="password" type="password" show-password autocomplete="current-password" @keyup.enter="doLogin" />
        </el-form-item>
      </el-form>

      <el-button :loading="loading" type="primary" @click="doLogin">登录</el-button>
      <el-text type="info" size="small">
        首次上线需先用管理员 token 调用 /api/admin/bootstrap 创建管理员账号（见 runbook）。
      </el-text>
    </el-space>
  </el-card>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { useRouter } from "vue-router";
import { login } from "../../lib/authStore";

const router = useRouter();
const loading = ref(false);
const error = ref("");
const username = ref("");
const password = ref("");

async function doLogin() {
  loading.value = true;
  error.value = "";
  try {
    await login(username.value.trim(), password.value);
    await router.push("/");
  } catch (e: any) {
    error.value = e?.body ? `${e.message}\n${e.body}` : (e?.message ?? String(e));
  } finally {
    loading.value = false;
  }
}
</script>

<style scoped>
.title {
  font-weight: 700;
}
.sub {
  margin-top: 4px;
  font-size: 12px;
  color: #6b7280;
}
</style>

