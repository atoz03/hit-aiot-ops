<template>
  <el-container style="min-height: 100vh">
    <el-aside width="260px" class="aside">
      <div class="brand">
        <div class="brand-icon">
          <el-icon :size="32"><Monitor /></el-icon>
        </div>
        <div class="brand-content">
          <div class="brand-title gradient-text">GPU/CPU 集群</div>
          <div class="brand-sub">智能管理平台</div>
        </div>
      </div>

      <div class="menu-section">
        <div class="menu-label">概览</div>
        <el-menu :default-active="activePath" router class="menu">
          <el-menu-item index="/" class="menu-item-custom">
            <el-icon><DataBoard /></el-icon>
            <span>系统概览</span>
          </el-menu-item>
        </el-menu>
      </div>

      <div class="menu-section">
        <div class="menu-label">用户功能</div>
        <el-menu :default-active="activePath" router class="menu">
          <el-menu-item index="/user/balance" class="menu-item-custom">
            <el-icon><User /></el-icon>
            <span>积分查询</span>
          </el-menu-item>
          <el-menu-item index="/user/usage" class="menu-item-custom">
            <el-icon><Tickets /></el-icon>
            <span>用户用量</span>
          </el-menu-item>
          <el-menu-item index="/user/register" class="menu-item-custom">
            <el-icon><EditPen /></el-icon>
            <span>用户注册</span>
          </el-menu-item>
        </el-menu>
      </div>

      <div class="menu-section">
        <div class="menu-label">管理功能</div>
        <el-menu :default-active="activePath" router class="menu">
          <el-menu-item index="/admin/nodes" class="menu-item-custom">
            <el-icon><Monitor /></el-icon>
            <span>节点状态</span>
          </el-menu-item>
          <el-menu-item index="/admin/users" class="menu-item-custom">
            <el-icon><UserFilled /></el-icon>
            <span>用户管理</span>
          </el-menu-item>
          <el-menu-item index="/admin/prices" class="menu-item-custom">
            <el-icon><Coin /></el-icon>
            <span>价格配置</span>
          </el-menu-item>
          <el-menu-item index="/admin/usage" class="menu-item-custom">
            <el-icon><Document /></el-icon>
            <span>使用记录</span>
          </el-menu-item>
          <el-menu-item index="/admin/requests" class="menu-item-custom">
            <el-icon><EditPen /></el-icon>
            <span>注册审核</span>
          </el-menu-item>
          <el-menu-item index="/admin/queue" class="menu-item-custom">
            <el-icon><Timer /></el-icon>
            <span>排队队列</span>
          </el-menu-item>
        </el-menu>
      </div>
    </el-aside>

    <el-container>
      <el-header class="header glass-effect">
        <div class="header-left">
          <el-icon :size="20" style="color: var(--primary-color)"><Connection /></el-icon>
          <el-text size="default" style="color: var(--text-secondary)">控制器地址</el-text>
          <el-input
            v-model="settingsState.baseUrl"
            placeholder="留空表示当前站点"
            style="max-width: 320px"
            @change="persist"
            clearable
          />
        </div>
        <div class="header-right">
          <el-space :size="12">
            <div v-if="authState.authenticated" class="user-badge">
              <el-icon><UserFilled /></el-icon>
              <span>{{ authState.username }}</span>
            </div>
            <el-tag v-else type="info" effect="plain">未登录</el-tag>
            <el-button @click="persist" type="primary" size="default" round>
              <el-icon><Check /></el-icon>
              保存
            </el-button>
            <el-button v-if="authState.authenticated" @click="doLogout" size="default" round>
              <el-icon><SwitchButton /></el-icon>
              退出
            </el-button>
            <el-button v-else @click="goLogin" type="primary" size="default" round>
              <el-icon><User /></el-icon>
              登录
            </el-button>
          </el-space>
        </div>
      </el-header>

      <el-main class="main">
        <transition name="fade" mode="out-in">
          <router-view :key="activePath" />
        </transition>
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { useRoute, useRouter } from "vue-router";
import { persistSettings, settingsState } from "../lib/settingsStore";
import { authState, logout } from "../lib/authStore";
import {
  Coin,
  DataBoard,
  Document,
  Monitor,
  Tickets,
  Timer,
  User,
  UserFilled,
  Connection,
  Check,
  SwitchButton,
  EditPen
} from "@element-plus/icons-vue";

const route = useRoute();
const router = useRouter();
const activePath = computed(() => route.path);

function persist() {
  persistSettings();
}

async function doLogout() {
  await logout();
  await router.push("/login");
}

async function goLogin() {
  await router.push("/login");
}
</script>

<style scoped>
.aside {
  background: linear-gradient(180deg, #1e293b 0%, #0f172a 100%);
  border-right: 1px solid var(--border-color);
  box-shadow: var(--shadow-lg);
  overflow-y: auto;
}

.brand {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 20px 16px;
  border-bottom: 1px solid var(--border-color);
  background: rgba(255, 255, 255, 0.02);
}

.brand-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 48px;
  height: 48px;
  border-radius: 12px;
  background: var(--primary-gradient);
  box-shadow: var(--shadow-md);
}

.brand-content {
  flex: 1;
}

.brand-title {
  font-weight: 700;
  font-size: 16px;
  line-height: 1.2;
}

.brand-sub {
  margin-top: 4px;
  font-size: 12px;
  color: var(--text-tertiary);
}

.menu-section {
  padding: 16px 0;
}

.menu-label {
  padding: 8px 20px;
  font-size: 12px;
  font-weight: 600;
  color: var(--text-tertiary);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.menu {
  border-right: none;
  background: transparent;
}

.menu-item-custom {
  margin: 4px 12px;
  border-radius: 8px;
  transition: all 0.3s ease;
}

.menu-item-custom:hover {
  background: rgba(99, 102, 241, 0.1) !important;
}

.menu-item-custom.is-active {
  background: var(--primary-gradient) !important;
  color: white !important;
  box-shadow: var(--shadow-md);
}

.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 0 24px;
  background: rgba(30, 41, 59, 0.8);
  backdrop-filter: blur(10px);
  border-bottom: 1px solid var(--border-color);
  box-shadow: var(--shadow-sm);
}

.header-left {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
}

.header-right {
  display: flex;
  align-items: center;
}

.user-badge {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 16px;
  background: var(--primary-gradient);
  border-radius: 20px;
  color: white;
  font-weight: 500;
  box-shadow: var(--shadow-sm);
}

.main {
  background: var(--bg-primary);
  color: var(--text-primary);
  padding: 24px;
  min-height: calc(100vh - 60px);
}

/* 页面切换动画 */
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.3s ease, transform 0.3s ease;
}

.fade-enter-from {
  opacity: 0;
  transform: translateY(10px);
}

.fade-leave-to {
  opacity: 0;
  transform: translateY(-10px);
}
</style>
