<template>
  <div class="login-container">
    <div class="login-background">
      <div class="gradient-orb orb-1"></div>
      <div class="gradient-orb orb-2"></div>
      <div class="gradient-orb orb-3"></div>
    </div>

    <el-card class="login-card glass-effect">
      <template #header>
        <div class="login-header">
          <div class="login-icon">
            <el-icon :size="40"><Lock /></el-icon>
          </div>
          <h1 class="login-title gradient-text">管理员登录</h1>
          <p class="login-subtitle">使用管理员账号访问控制面板</p>
        </div>
      </template>

      <el-space direction="vertical" fill style="width: 100%">
        <el-alert v-if="error" :title="error" type="error" show-icon class="login-alert" />

        <el-form label-position="top" class="login-form">
          <el-form-item label="用户名">
            <el-input
              v-model="username"
              autocomplete="username"
              @keyup.enter="doLogin"
              size="large"
              :prefix-icon="User"
              placeholder="请输入用户名"
            />
          </el-form-item>
          <el-form-item label="密码">
            <el-input
              v-model="password"
              type="password"
              show-password
              autocomplete="current-password"
              @keyup.enter="doLogin"
              size="large"
              :prefix-icon="Key"
              placeholder="请输入密码"
            />
          </el-form-item>
        </el-form>

        <el-button
          :loading="loading"
          type="primary"
          @click="doLogin"
          size="large"
          style="width: 100%; margin-top: 12px"
          round
        >
          <el-icon v-if="!loading"><Right /></el-icon>
          {{ loading ? "登录中..." : "立即登录" }}
        </el-button>

        <div class="login-tip">
          <el-icon><InfoFilled /></el-icon>
          <span>首次部署需使用管理员 token 调用 /api/admin/bootstrap 创建账号</span>
        </div>
      </el-space>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { useRouter } from "vue-router";
import { login } from "../../lib/authStore";
import { Lock, User, Key, Right, InfoFilled } from "@element-plus/icons-vue";

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
.login-container {
  position: relative;
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
  overflow: hidden;
}

.login-background {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: var(--bg-primary);
  overflow: hidden;
}

.gradient-orb {
  position: absolute;
  border-radius: 50%;
  filter: blur(80px);
  opacity: 0.3;
  animation: float 20s ease-in-out infinite;
}

.orb-1 {
  width: 400px;
  height: 400px;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  top: -200px;
  left: -200px;
  animation-delay: 0s;
}

.orb-2 {
  width: 500px;
  height: 500px;
  background: linear-gradient(135deg, #f093fb 0%, #f5576c 100%);
  bottom: -250px;
  right: -250px;
  animation-delay: 7s;
}

.orb-3 {
  width: 350px;
  height: 350px;
  background: linear-gradient(135deg, #4facfe 0%, #00f2fe 100%);
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  animation-delay: 14s;
}

@keyframes float {
  0%, 100% {
    transform: translate(0, 0) scale(1);
  }
  33% {
    transform: translate(30px, -30px) scale(1.1);
  }
  66% {
    transform: translate(-20px, 20px) scale(0.9);
  }
}

.login-card {
  position: relative;
  z-index: 1;
  width: 100%;
  max-width: 440px;
  backdrop-filter: blur(20px);
  animation: slideUp 0.6s ease;
}

@keyframes slideUp {
  from {
    opacity: 0;
    transform: translateY(30px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.login-header {
  text-align: center;
}

.login-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 80px;
  height: 80px;
  border-radius: 20px;
  background: var(--primary-gradient);
  box-shadow: var(--shadow-lg);
  margin-bottom: 20px;
}

.login-title {
  font-size: 28px;
  font-weight: 700;
  margin: 0 0 8px 0;
}

.login-subtitle {
  font-size: 14px;
  color: var(--text-tertiary);
  margin: 0;
}

.login-alert {
  margin-bottom: 8px;
}

.login-form {
  margin-top: 8px;
}

.login-tip {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 12px;
  background: rgba(59, 130, 246, 0.1);
  border-radius: var(--radius-md);
  color: var(--info-color);
  font-size: 12px;
  line-height: 1.5;
  margin-top: 16px;
}

.login-tip .el-icon {
  flex-shrink: 0;
  margin-top: 2px;
}
</style>

