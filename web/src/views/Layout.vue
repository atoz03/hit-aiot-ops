<template>
  <el-container class="shell">
    <el-aside width="250px" class="aside">
      <div class="brand">
        <el-icon :size="30"><Cpu /></el-icon>
        <div>
          <div class="brand-title">HIT-AIOT-OPS</div>
          <div class="brand-sub">GPU 运维平台</div>
        </div>
      </div>

      <el-menu :default-active="activePath" router class="menu">
        <template v-if="authState.role === 'admin'">
          <el-menu-item index="/admin/board"><el-icon><DataBoard /></el-icon><span>运营看板</span></el-menu-item>
          <el-menu-item index="/admin/nodes"><el-icon><Monitor /></el-icon><span>节点状态</span></el-menu-item>
          <el-menu-item index="/admin/users"><el-icon><UserFilled /></el-icon><span>用户管理</span></el-menu-item>
          <el-menu-item index="/admin/prices"><el-icon><Money /></el-icon><span>价格配置</span></el-menu-item>
          <el-menu-item index="/admin/usage"><el-icon><Tickets /></el-icon><span>使用记录</span></el-menu-item>
          <el-menu-item index="/admin/requests"><el-icon><Checked /></el-icon><span>注册审核</span></el-menu-item>
          <el-menu-item index="/admin/queue"><el-icon><Clock /></el-icon><span>排队队列</span></el-menu-item>
          <el-menu-item index="/admin/accounts"><el-icon><UserFilled /></el-icon><span>账号映射</span></el-menu-item>
          <el-menu-item index="/admin/whitelist"><el-icon><Lock /></el-icon><span>SSH白名单</span></el-menu-item>
          <el-menu-item index="/admin/mail"><el-icon><Message /></el-icon><span>邮件设置</span></el-menu-item>
          <el-menu-item index="/admin/change-password"><el-icon><Key /></el-icon><span>修改密码</span></el-menu-item>
        </template>
        <template v-else>
          <el-menu-item index="/user/balance"><el-icon><WalletFilled /></el-icon><span>我的余额</span></el-menu-item>
          <el-menu-item index="/user/usage"><el-icon><DataAnalysis /></el-icon><span>我的用量</span></el-menu-item>
          <el-menu-item index="/user/accounts"><el-icon><UserFilled /></el-icon><span>服务器账号</span></el-menu-item>
          <el-menu-item index="/user/change-password"><el-icon><Key /></el-icon><span>修改密码</span></el-menu-item>
        </template>
      </el-menu>
    </el-aside>

    <el-container>
      <el-header class="header">
        <div class="header-left">
          <el-icon><Link /></el-icon>
          <span class="muted">控制器地址</span>
          <el-input
            v-model="settingsState.baseUrl"
            placeholder="留空表示当前站点"
            style="max-width: 320px"
            @change="persist"
            clearable
          />
        </div>
        <div class="header-right">
          <el-tag type="success" effect="light">{{ authState.role === 'admin' ? '管理员' : '用户' }}</el-tag>
          <el-tag effect="plain">{{ authState.username }}</el-tag>
          <el-button @click="persist" type="primary">
            <el-icon><Check /></el-icon>
            保存
          </el-button>
          <el-button @click="doLogout">
            <el-icon><SwitchButton /></el-icon>
            退出
          </el-button>
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
import { useRoute, useRouter } from "vue-router";
import { persistSettings, settingsState } from "../lib/settingsStore";
import { authState, logout } from "../lib/authStore";
import {
  Check,
  Clock,
  Cpu,
  DataAnalysis,
  DataBoard,
  Key,
  Link,
  Message,
  Money,
  Monitor,
  Lock,
  SwitchButton,
  Tickets,
  UserFilled,
  WalletFilled,
  Checked,
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
</script>

<style scoped>
.shell {
  min-height: 100vh;
  background: #f4f7fb;
}
.aside {
  border-right: 1px solid #dbe3ef;
  background: linear-gradient(180deg, #ffffff 0%, #f4f8ff 100%);
}
.brand {
  display: flex;
  gap: 10px;
  align-items: center;
  padding: 18px 16px;
  border-bottom: 1px solid #e2e8f0;
  color: #0f172a;
}
.brand-title {
  font-weight: 800;
  letter-spacing: 0.3px;
}
.brand-sub {
  font-size: 12px;
  color: #475569;
}
.menu {
  border-right: none;
  padding: 8px;
  background: transparent;
}
.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  border-bottom: 1px solid #dbe3ef;
  background: #ffffff;
}
.header-left {
  display: flex;
  align-items: center;
  gap: 8px;
}
.muted {
  color: #475569;
}
.header-right {
  display: flex;
  gap: 8px;
  align-items: center;
}
.main {
  padding: 18px;
}
</style>
