<template>
  <el-card>
    <template #header>
      <div class="row">
        <div>
          <div class="title">系统概览</div>
          <div class="sub">健康检查、最小监控指标（/metrics）</div>
        </div>
        <div class="row">
          <el-button :loading="loading" type="primary" @click="reload">刷新</el-button>
        </div>
      </div>
    </template>

    <el-space direction="vertical" fill style="width: 100%">
      <el-alert v-if="error" :title="error" type="error" show-icon />

      <el-descriptions :column="2" border>
        <el-descriptions-item label="健康">
          <el-tag :type="healthOk ? 'success' : 'danger'">{{ healthOk ? "OK" : "未知" }}</el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="控制器">
          <el-text>{{ effectiveBaseUrl }}</el-text>
        </el-descriptions-item>
      </el-descriptions>

      <el-card>
        <template #header>
          <div class="row">
            <span>Metrics 原文（截断显示）</span>
            <el-button text @click="copyMetrics">复制</el-button>
          </div>
        </template>
        <el-input v-model="metricsPreview" type="textarea" :rows="14" readonly />
      </el-card>
    </el-space>
  </el-card>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { ElMessage } from "element-plus";
import { ApiClient } from "../../lib/api";
import { loadSettings } from "../../lib/settings";

const loading = ref(false);
const error = ref<string>("");
const healthOk = ref(false);
const metricsPreview = ref("");

const settings = loadSettings();
const client = new ApiClient(settings.baseUrl, settings.adminToken);
const effectiveBaseUrl = computed(() => settings.baseUrl?.trim() || window.location.origin);

async function reload() {
  loading.value = true;
  error.value = "";
  try {
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
    ElMessage.success("已复制");
  } catch {
    ElMessage.warning("复制失败：浏览器权限限制");
  }
}

reload();
</script>

<style scoped>
.row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.title {
  font-weight: 700;
}
.sub {
  margin-top: 4px;
  font-size: 12px;
  color: #6b7280;
}
</style>

