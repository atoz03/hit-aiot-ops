<template>
  <div class="page-container fade-in">
    <div class="page-header">
      <div class="header-content">
        <div class="header-icon">
          <el-icon :size="28"><Monitor /></el-icon>
        </div>
        <div>
          <h1 class="page-title">节点状态监控</h1>
          <p class="page-subtitle">实时监控集群节点运行状态和资源使用情况</p>
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

    <!-- 统计卡片 -->
    <div class="stats-grid" v-if="rows.length > 0">
      <div class="stat-card">
        <div class="stat-icon" style="background: linear-gradient(135deg, #667eea 0%, #764ba2 100%)">
          <el-icon :size="24"><Monitor /></el-icon>
        </div>
        <div class="stat-content">
          <div class="stat-value">{{ rows.length }}</div>
          <div class="stat-label">在线节点</div>
        </div>
      </div>

      <div class="stat-card">
        <div class="stat-icon" style="background: linear-gradient(135deg, #f093fb 0%, #f5576c 100%)">
          <el-icon :size="24"><Cpu /></el-icon>
        </div>
        <div class="stat-content">
          <div class="stat-value">{{ totalGpuProcesses }}</div>
          <div class="stat-label">GPU 进程总数</div>
        </div>
      </div>

      <div class="stat-card">
        <div class="stat-icon" style="background: linear-gradient(135deg, #4facfe 0%, #00f2fe 100%)">
          <el-icon :size="24"><Cpu /></el-icon>
        </div>
        <div class="stat-content">
          <div class="stat-value">{{ totalCpuProcesses }}</div>
          <div class="stat-label">CPU 进程总数</div>
        </div>
      </div>

      <div class="stat-card">
        <div class="stat-icon" style="background: linear-gradient(135deg, #fa709a 0%, #fee140 100%)">
          <el-icon :size="24"><Coin /></el-icon>
        </div>
        <div class="stat-content">
          <div class="stat-value">{{ totalCost.toFixed(2) }}</div>
          <div class="stat-label">总成本</div>
        </div>
      </div>
    </div>

    <!-- 节点列表 -->
    <el-card class="table-card">
      <template #header>
        <div class="card-header">
          <div class="card-title">
            <el-icon><List /></el-icon>
            <span>节点详细信息</span>
          </div>
          <el-text type="info" size="small">共 {{ rows.length }} 个节点</el-text>
        </div>
      </template>

      <el-table
        :data="rows"
        stripe
        style="width: 100%"
        :height="520"
        :header-cell-style="{ background: 'var(--bg-tertiary)', color: 'var(--text-primary)' }"
      >
        <el-table-column prop="node_id" label="节点 ID" width="180" fixed>
          <template #default="{ row }">
            <div class="node-id-cell">
              <el-icon color="var(--primary-color)"><Monitor /></el-icon>
              <span>{{ row.node_id }}</span>
            </div>
          </template>
        </el-table-column>

        <el-table-column prop="last_seen_at" label="最后在线时间" width="200">
          <template #default="{ row }">
            <div class="time-cell">
              <el-icon><Clock /></el-icon>
              <span>{{ formatTime(row.last_seen_at) }}</span>
            </div>
          </template>
        </el-table-column>

        <el-table-column prop="gpu_process_count" label="GPU 进程" width="120" align="center">
          <template #default="{ row }">
            <el-tag :type="row.gpu_process_count > 0 ? 'success' : 'info'" effect="dark" round>
              {{ row.gpu_process_count }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column prop="cpu_process_count" label="CPU 进程" width="120" align="center">
          <template #default="{ row }">
            <el-tag :type="row.cpu_process_count > 0 ? 'success' : 'info'" effect="dark" round>
              {{ row.cpu_process_count }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column prop="usage_records_count" label="记录数" width="100" align="center">
          <template #default="{ row }">
            <el-text type="primary" style="font-weight: 600">{{ row.usage_records_count }}</el-text>
          </template>
        </el-table-column>

        <el-table-column prop="cost_total" label="当次成本" width="120" align="right">
          <template #default="{ row }">
            <div class="cost-cell">
              <el-icon color="var(--warning-color)"><Coin /></el-icon>
              <span>{{ row.cost_total.toFixed(4) }}</span>
            </div>
          </template>
        </el-table-column>

        <el-table-column prop="interval_seconds" label="周期 (秒)" width="110" align="center">
          <template #default="{ row }">
            <el-tag type="info" effect="plain" size="small">{{ row.interval_seconds }}s</el-tag>
          </template>
        </el-table-column>

        <el-table-column prop="last_report_id" label="Report ID" min-width="200">
          <template #default="{ row }">
            <el-text type="info" size="small" style="font-family: monospace">
              {{ row.last_report_id }}
            </el-text>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from "vue";
import { ApiClient, type NodeStatus } from "../../lib/api";
import { settingsState } from "../../lib/settingsStore";
import { authState } from "../../lib/authStore";
import { Monitor, Refresh, Cpu, Coin, Clock, List } from "@element-plus/icons-vue";
import dayjs from "dayjs";

const loading = ref(false);
const error = ref("");
const rows = ref<NodeStatus[]>([]);

const totalGpuProcesses = computed(() => rows.value.reduce((sum, node) => sum + node.gpu_process_count, 0));
const totalCpuProcesses = computed(() => rows.value.reduce((sum, node) => sum + node.cpu_process_count, 0));
const totalCost = computed(() => rows.value.reduce((sum, node) => sum + node.cost_total, 0));

function formatTime(time: string): string {
  if (!time) return "-";
  return dayjs(time).format("YYYY-MM-DD HH:mm:ss");
}

async function reload() {
  loading.value = true;
  error.value = "";
  rows.value = [];
  try {
    const client = new ApiClient(settingsState.baseUrl, { csrfToken: authState.csrfToken });
    const r = await client.adminNodes(200);
    rows.value = r.nodes ?? [];
  } catch (e: any) {
    error.value = e?.body ? `${e.message}\n${e.body}` : (e?.message ?? String(e));
  } finally {
    loading.value = false;
  }
}

reload();
</script>

<style scoped>
.page-container {
  max-width: 1600px;
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

.stats-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 20px;
  margin-bottom: 24px;
}

.stat-card {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 20px;
  background: var(--bg-card);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-md);
  transition: all 0.3s ease;
}

.stat-card:hover {
  transform: translateY(-4px);
  box-shadow: var(--shadow-lg);
  border-color: var(--primary-color);
}

.stat-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 56px;
  height: 56px;
  border-radius: 12px;
  color: white;
  box-shadow: var(--shadow-md);
}

.stat-content {
  flex: 1;
}

.stat-value {
  font-size: 28px;
  font-weight: 700;
  color: var(--text-primary);
  line-height: 1;
}

.stat-label {
  font-size: 13px;
  color: var(--text-tertiary);
  margin-top: 6px;
}

.table-card {
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

.node-id-cell {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 500;
}

.time-cell {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: var(--text-secondary);
}

.cost-cell {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 6px;
  font-weight: 600;
  color: var(--warning-color);
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
