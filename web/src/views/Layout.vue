<template>
  <el-container style="min-height: 100vh">
    <el-aside width="240px" class="aside">
      <div class="brand">
        <div class="brand-title">GPU/CPU 集群管理</div>
        <div class="brand-sub">控制器托管前端（Vue3）</div>
      </div>

      <el-menu :default-active="activePath" router class="menu">
        <el-menu-item index="/">
          <el-icon><DataBoard /></el-icon>
          <span>概览</span>
        </el-menu-item>

        <el-menu-item index="/user/balance">
          <el-icon><User /></el-icon>
          <span>用户余额</span>
        </el-menu-item>
        <el-menu-item index="/user/usage">
          <el-icon><Tickets /></el-icon>
          <span>用户用量</span>
        </el-menu-item>

        <el-menu-item index="/admin/nodes">
          <el-icon><Monitor /></el-icon>
          <span>节点状态</span>
        </el-menu-item>
        <el-menu-item index="/admin/users">
          <el-icon><UserFilled /></el-icon>
          <span>用户管理</span>
        </el-menu-item>
        <el-menu-item index="/admin/prices">
          <el-icon><Coin /></el-icon>
          <span>价格配置</span>
        </el-menu-item>
        <el-menu-item index="/admin/usage">
          <el-icon><Document /></el-icon>
          <span>使用记录</span>
        </el-menu-item>
        <el-menu-item index="/admin/queue">
          <el-icon><Timer /></el-icon>
          <span>排队队列</span>
        </el-menu-item>
      </el-menu>
    </el-aside>

    <el-container>
      <el-header class="header">
        <div class="header-left">
          <el-text size="large">控制器地址：</el-text>
          <el-input v-model="settingsState.baseUrl" placeholder="留空表示当前站点" style="max-width: 340px" @change="persist" />
          <el-text size="large" style="margin-left: 12px">管理员 Token：</el-text>
          <el-input v-model="settingsState.adminToken" placeholder="Bearer token（仅管理员接口需要）" style="max-width: 360px" show-password @change="persist" />
        </div>
        <div class="header-right">
          <el-button @click="persist" type="primary">保存</el-button>
        </div>
      </el-header>

      <el-main class="main">
        <router-view :key="activePath" />
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { useRoute } from "vue-router";
import { persistSettings, settingsState } from "../lib/settingsStore";
import { Coin, DataBoard, Document, Monitor, Tickets, Timer, User, UserFilled } from "@element-plus/icons-vue";

const route = useRoute();
const activePath = computed(() => route.path);

function persist() {
  persistSettings();
}
</script>

<style scoped>
.aside {
  background: #0f1830;
  border-right: 1px solid #1f2a4a;
}
.brand {
  padding: 14px 14px 10px;
  border-bottom: 1px solid #1f2a4a;
}
.brand-title {
  font-weight: 700;
  color: #e9ecf1;
}
.brand-sub {
  margin-top: 4px;
  font-size: 12px;
  color: #b9c0cf;
}
.menu {
  border-right: none;
  background: transparent;
}
.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  background: #101a33;
  border-bottom: 1px solid #1f2a4a;
}
.header-left {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}
.main {
  background: #0b1020;
  color: #e9ecf1;
}
</style>
