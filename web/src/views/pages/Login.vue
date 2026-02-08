<template>
  <div class="auth-shell">
    <div class="left-hero">
      <div class="hero-card">
        <el-icon :size="44"><Cpu /></el-icon>
        <h1>HIT-AIOT-OPS</h1>
        <p>GPU 资源运维与计费平台</p>
      </div>
    </div>

    <el-card class="login-card">
      <template #header>
        <div class="head">
          <h2>账号登录</h2>
          <p>管理员与普通用户共用登录入口</p>
        </div>
      </template>

      <el-alert v-if="error" :title="error" type="error" show-icon class="mb" />

      <el-form label-position="top">
        <el-form-item label="用户名">
          <el-input v-model="username" autocomplete="username" :prefix-icon="User" @keyup.enter="doLogin" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input v-model="password" type="password" show-password autocomplete="current-password" :prefix-icon="Key" @keyup.enter="doLogin" />
        </el-form-item>
      </el-form>

      <el-button :loading="loading" type="primary" @click="doLogin" style="width: 100%">登录</el-button>

      <div class="actions">
        <router-link to="/register">用户注册</router-link>
        <router-link to="/forgot-password">找回密码</router-link>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { useRouter } from "vue-router";
import { login, authState } from "../../lib/authStore";
import { Cpu, Key, User } from "@element-plus/icons-vue";

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
    if (authState.role === "admin") {
      await router.push("/admin/board");
    } else {
      await router.push("/user/balance");
    }
  } catch (e: any) {
    error.value = e?.body ? `${e.message}\n${e.body}` : (e?.message ?? String(e));
  } finally {
    loading.value = false;
  }
}
</script>

<style scoped>
.auth-shell {
  min-height: 100vh;
  display: grid;
  grid-template-columns: 1fr 460px;
  background: #eef4ff;
}
.left-hero {
  display: grid;
  place-items: center;
  background: radial-gradient(circle at 20% 20%, #0ea5e9 0, #1d4ed8 45%, #0f172a 100%);
}
.hero-card {
  color: #fff;
  text-align: center;
}
.hero-card h1 {
  margin: 10px 0 4px;
  font-size: 40px;
}
.hero-card p {
  margin: 0;
  opacity: 0.9;
}
.login-card {
  margin: auto;
  width: 92%;
  max-width: 420px;
}
.head h2 {
  margin: 0;
}
.head p {
  color: #475569;
  margin: 6px 0 0;
}
.mb {
  margin-bottom: 12px;
}
.actions {
  margin-top: 14px;
  display: flex;
  justify-content: space-between;
}
.actions a {
  color: #1d4ed8;
  text-decoration: none;
}
@media (max-width: 900px) {
  .auth-shell {
    grid-template-columns: 1fr;
  }
  .left-hero {
    min-height: 220px;
  }
}
</style>
