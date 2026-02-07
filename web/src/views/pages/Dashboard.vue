<template>
  <div class="page-container fade-in">
    <div class="page-header">
      <div class="header-content">
        <div class="header-icon">
          <el-icon :size="28"><DataBoard /></el-icon>
        </div>
        <div>
          <h1 class="page-title">系统概览</h1>
          <p class="page-subtitle">监控系统健康状态和核心指标</p>
        </div>
      </div>
      <div class="header-actions">
        <el-button :loading="loading" type="primary" @click="reload" size="large" round>
          <el-icon><Refresh /></el-icon>
          刷新数据
        </el-button>
      </div>
    </div>

    <el-alert v-if="error" :title="error" type="error" show-icon class="error-alert" />

    <!-- 健康状态卡片 -->
    <div class="health-grid">
      <div class="health-card">
        <div class="health-icon" :class="healthOk ? 'health-ok' : 'health-error'">
          <el-icon :size="32"><CircleCheck v-if="healthOk" /><CircleClose v-else /></el-icon>
        </div>
        <div class="health-content">
          <div class="health-label">系统健康状态</div>
          <div class="health-value" :class="healthOk ? 'text-success' : 'text-error'">
            {{ healthOk ? "运行正常" : "异常" }}
          </div>
        </div>
      </div>

      <div class="health-card">
        <div class="health-icon" style="background: linear-gradient(135deg, #667eea 0%, #764ba2 100%)">
          <el-icon :size="32"><Connection /></el-icon>
        </div>
        <div class="health-content">
          <div class="health-label">控制器地址</div>
          <div class="health-value" style="font-size: 14px; word-break: break-all">
            {{ effectiveBaseUrl }}
          </div>
        </div>
      </div>
    </div>

    <!-- Metrics 数据卡片 -->
    <el-card class="metrics-card">
      <template #header>
        <div class="card-header">
          <div class="card-title">
            <el-icon><Document /></el-icon>
            <span>Metrics 监控数据</span>
          </div>
          <el-button text @click="copyMetrics" :icon="DocumentCopy">
            复制全部
          </el-button>
        </div>
      </template>

      <div class="metrics-content">
        <el-input
          v-model="metricsPreview"
          type="textarea"
          :rows="16"
          readonly
          class="metrics-textarea"
        />
        <div class="metrics-info">
          <el-icon><InfoFilled /></el-icon>
          <span>显示前 6000 字符，完整数据请访问 /metrics 端点</span>
        </div>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { ElMessage } from "element-plus";
import { ApiClient } from "../../lib/api";
import { settingsState } from "../../lib/settingsStore";
import {
  DataBoard,
  Refresh,
  CircleCheck,
  CircleClose,
  Connection,
  Document,
  DocumentCopy,
  InfoFilled
} from "@element-plus/icons-vue";

const loading = ref(false);
const error = ref<string>("");
const healthOk = ref(false);
const metricsPreview = ref("");

const effectiveBaseUrl = computed(() => settingsState.baseUrl?.trim() || window.location.origin);

async function reload() {
  loading.value = true;
  error.value = "";
  try {
    const client = new ApiClient(settingsState.baseUrl);
    const h = await client.healthz();
    healthOk.value = !!h.ok;
    const metrics = await client.metricsText();
    metricsPreview.value = metrics.slice(0, 6000);
  } catch (e: any) {
    error.value = e?.body ? `${e.message}\n${e.body}` : (e?.message ?? String(e));
    healthOk.value = false;
  } finally {
    loading.value = false;
  }
}

async function copyMetrics() {
  try {
    await navigator.clipboard.writeText(metricsPreview.value);
    ElMessage.success("已复制到剪贴板");
  } catch {
    ElMessage.warning("复制失败：浏览器权限限制");
  }
}

reload();
</script>

<style scoped>
.page-container {
  max-width: 1400px;
  margin: 0 auto;
}

.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 24px;
  padding: 24px;
  background: var(--bg-card);
  border-radius: var(--radius-lg);
  border: 1px solid var(--border-color);
  box-shadow: var(--shadow-md);
}

.header-content {
  display: flex;
  align-items: center;
  gap: 16px;
}

.header-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 56px;
  height: 56px;
  border-radius: 12px;
  background: var(--primary-gradient);
  box-shadow: var(--shadow-md);
}

.page-title {
  font-size: 24px;
  font-weight: 700;
  color: var(--text-primary);
  margin: 0;
}

.page-subtitle {
  font-size: 14px;
  color: var(--text-tertiary);
  margin: 4px 0 0 0;
}

.header-actions {
  display: flex;
  gap: 12px;
}

.error-alert {
  margin-bottom: 24px;
}

.health-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
  gap: 20px;
  margin-bottom: 24px;
}

.health-card {
  display: flex;
  align-items: center;
  gap: 20px;
  padding: 24px;
  background: var(--bg-card);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-md);
  transition: all 0.3s ease;
}

.health-card:hover {
  transform: translateY(-4px);
  box-shadow: var(--shadow-lg);
}

.health-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 64px;
  height: 64px;
  border-radius: 12px;
  color: white;
  box-shadow: var(--shadow-md);
}

.health-icon.health-ok {
  background: linear-gradient(135deg, #10b981 0%, #059669 100%);
}

.health-icon.health-error {
  background: linear-gradient(135deg, #ef4444 0%, #dc2626 100%);
}

.health-content {
  flex: 1;
}

.health-label {
  font-size: 13px;
  color: var(--text-tertiary);
  margin-bottom: 6px;
}

.health-value {
  font-size: 20px;
  font-weight: 700;
  color: var(--text-primary);
}

.text-success {
  color: var(--success-color);
}

.text-error {
  color: var(--error-color);
}

.metrics-card {
  animation: fadeIn 0.5s ease;
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.card-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 16px;
  font-weight: 600;
  color: var(--text-primary);
}

.metrics-content {
  position: relative;
}

.metrics-textarea {
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  font-size: 12px;
}

.metrics-info {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 12px;
  padding: 12px;
  background: rgba(59, 130, 246, 0.1);
  border-radius: var(--radius-md);
  color: var(--info-color);
  font-size: 13px;
}

@keyframes fadeIn {
  from {
    opacity: 0;
    transform: translateY(20px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}
</style>
