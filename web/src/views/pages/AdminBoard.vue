<template>
  <el-space direction="vertical" fill style="width: 100%">
    <el-card>
      <template #header>
        <div class="head">
          <span class="title">运营看板</span>
          <el-button type="primary" :loading="loading" @click="loadAll">刷新</el-button>
        </div>
      </template>

      <el-alert v-if="error" :title="error" type="error" show-icon class="mb" />

      <el-form inline>
        <el-form-item label="统计区间">
          <el-date-picker
            v-model="range"
            type="daterange"
            range-separator="至"
            start-placeholder="开始日期"
            end-placeholder="结束日期"
            value-format="YYYY-MM-DD"
          />
        </el-form-item>
      </el-form>
    </el-card>

    <el-card>
      <template #header><b>每用户使用情况（区间汇总）</b></template>
      <el-table :data="userRows" stripe height="340">
        <el-table-column prop="username" label="用户" width="150" />
        <el-table-column prop="usage_records" label="记录数" width="110" />
        <el-table-column prop="gpu_process_records" label="GPU记录" width="110" />
        <el-table-column prop="cpu_process_records" label="CPU记录" width="110" />
        <el-table-column prop="total_cpu_percent" label="CPU总占用%" width="140" />
        <el-table-column prop="total_memory_mb" label="内存MB累计" width="140" />
        <el-table-column prop="total_cost" label="总费用" width="120" />
      </el-table>
    </el-card>

    <el-card>
      <template #header><b>每月所有用户使用情况</b></template>
      <el-table :data="monthlyRows" stripe height="360">
        <el-table-column prop="month" label="月份" width="100" />
        <el-table-column prop="username" label="用户" width="150" />
        <el-table-column prop="usage_records" label="记录数" width="100" />
        <el-table-column prop="gpu_process_records" label="GPU记录" width="110" />
        <el-table-column prop="cpu_process_records" label="CPU记录" width="110" />
        <el-table-column prop="total_cost" label="总费用" width="120" />
        <el-table-column prop="total_cpu_percent" label="CPU总占用%" width="130" />
      </el-table>
    </el-card>

    <el-card>
      <template #header><b>余额增加（充值）统计</b></template>
      <el-table :data="rechargeRows" stripe height="320">
        <el-table-column prop="username" label="用户" width="180" />
        <el-table-column prop="recharge_count" label="充值次数" width="120" />
        <el-table-column prop="recharge_total" label="充值总额" width="140" />
        <el-table-column prop="last_recharge" label="最后充值时间" min-width="220" />
      </el-table>
    </el-card>
  </el-space>
</template>

<script setup lang="ts">
import { ref } from "vue";
import type { RechargeSummary, UsageMonthlySummary, UsageUserSummary } from "../../lib/api";
import { ApiClient } from "../../lib/api";
import { settingsState } from "../../lib/settingsStore";
import { authState } from "../../lib/authStore";

const loading = ref(false);
const error = ref("");

const today = new Date();
const yearAgo = new Date();
yearAgo.setDate(today.getDate() - 365);
const range = ref<[string, string]>([fmtDate(yearAgo), fmtDate(today)]);

const userRows = ref<UsageUserSummary[]>([]);
const monthlyRows = ref<UsageMonthlySummary[]>([]);
const rechargeRows = ref<RechargeSummary[]>([]);

function fmtDate(d: Date): string {
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${y}-${m}-${day}`;
}

async function loadAll() {
  loading.value = true;
  error.value = "";
  try {
    const [from, to] = range.value;
    const client = new ApiClient(settingsState.baseUrl, { csrfToken: authState.csrfToken });
    const [u, m, r] = await Promise.all([
      client.adminStatsUsers({ from, to, limit: 1000 }),
      client.adminStatsMonthly({ from, to, limit: 50000 }),
      client.adminStatsRecharges({ from, to, limit: 1000 }),
    ]);
    userRows.value = u.rows ?? [];
    monthlyRows.value = m.rows ?? [];
    rechargeRows.value = r.rows ?? [];
  } catch (e: any) {
    error.value = e?.body ? `${e.message}\n${e.body}` : (e?.message ?? String(e));
  } finally {
    loading.value = false;
  }
}

loadAll();
</script>

<style scoped>
.head { display: flex; align-items: center; justify-content: space-between; }
.title { font-weight: 700; font-size: 16px; }
.mb { margin-bottom: 12px; }
</style>
