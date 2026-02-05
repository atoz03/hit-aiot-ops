<template>
  <el-card>
    <template #header>
      <div class="row">
        <div>
          <div class="title">价格配置</div>
          <div class="sub">需要管理员登录：GET/POST /api/admin/prices（CPU 使用模型名 CPU_CORE）</div>
        </div>
        <div class="row">
          <el-button :loading="loading" type="primary" @click="reload">刷新</el-button>
          <el-button @click="openEdit()">新增/修改</el-button>
        </div>
      </div>
    </template>

    <el-space direction="vertical" fill style="width: 100%">
      <el-alert v-if="error" :title="error" type="error" show-icon />

      <el-table :data="rows" stripe height="520">
        <el-table-column prop="model" label="模型关键词" width="260" />
        <el-table-column prop="price" label="元/分钟" width="160" />
        <el-table-column label="操作" width="200">
          <template #default="{ row }">
            <el-button size="small" @click="openEdit(row.model, row.price)">编辑</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-space>

    <el-dialog v-model="editVisible" title="设置价格" width="420px">
      <el-form label-width="110px">
        <el-form-item label="模型关键词">
          <el-input v-model="editModel" placeholder="例如 RTX 3090 / A100 / CPU_CORE" />
        </el-form-item>
        <el-form-item label="元/分钟">
          <el-input-number v-model="editPrice" :min="0" :max="1000" :step="0.01" :precision="4" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">取消</el-button>
        <el-button :loading="editLoading" type="primary" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </el-card>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { ApiClient } from "../../lib/api";
import { settingsState } from "../../lib/settingsStore";
import { authState } from "../../lib/authStore";

type PriceRow = { model: string; price: number };

const loading = ref(false);
const error = ref("");
const rows = ref<PriceRow[]>([]);

const editVisible = ref(false);
const editLoading = ref(false);
const editModel = ref("");
const editPrice = ref(0.1);

function toRow(p: any): PriceRow {
  return {
    model: (p?.model ?? p?.Model ?? "").toString(),
    price: Number(p?.price ?? p?.Price ?? 0),
  };
}

async function reload() {
  loading.value = true;
  error.value = "";
  rows.value = [];
  try {
    const client = new ApiClient(settingsState.baseUrl, { csrfToken: authState.csrfToken });
    const r = await client.adminPrices();
    rows.value = (r.prices ?? []).map(toRow).sort((a, b) => a.model.localeCompare(b.model));
  } catch (e: any) {
    error.value = e?.body ? `${e.message}\n${e.body}` : (e?.message ?? String(e));
  } finally {
    loading.value = false;
  }
}

function openEdit(model = "", price = 0.1) {
  editModel.value = model;
  editPrice.value = price;
  editVisible.value = true;
}

async function save() {
  editLoading.value = true;
  error.value = "";
  try {
    const client = new ApiClient(settingsState.baseUrl, { csrfToken: authState.csrfToken });
    await client.adminSetPrice(editModel.value.trim(), editPrice.value);
    editVisible.value = false;
    await reload();
  } catch (e: any) {
    error.value = e?.body ? `${e.message}\n${e.body}` : (e?.message ?? String(e));
  } finally {
    editLoading.value = false;
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
