<template>
  <el-card class="card">
    <template #header>
      <div class="head">
        <el-icon><Message /></el-icon>
        <span>邮箱发送配置</span>
      </div>
    </template>

    <el-alert v-if="error" :title="error" type="error" show-icon class="mb" />
    <el-alert v-if="success" :title="success" type="success" show-icon class="mb" />

    <el-form label-position="top">
      <el-row :gutter="12">
        <el-col :span="12"><el-form-item label="SMTP 主机"><el-input v-model="form.smtp_host" placeholder="smtp.example.com" /></el-form-item></el-col>
        <el-col :span="12"><el-form-item label="SMTP 端口"><el-input-number v-model="form.smtp_port" :min="1" :max="65535" style="width: 100%" /></el-form-item></el-col>
      </el-row>
      <el-row :gutter="12">
        <el-col :span="12"><el-form-item label="SMTP 用户名"><el-input v-model="form.smtp_user" /></el-form-item></el-col>
        <el-col :span="12"><el-form-item label="SMTP 密码"><el-input v-model="smtpPass" type="password" show-password placeholder="留空表示不修改" /></el-form-item></el-col>
      </el-row>
      <el-row :gutter="12">
        <el-col :span="12"><el-form-item label="发件邮箱"><el-input v-model="form.from_email" /></el-form-item></el-col>
        <el-col :span="12"><el-form-item label="发件人名称"><el-input v-model="form.from_name" /></el-form-item></el-col>
      </el-row>
    </el-form>

    <el-button type="primary" :loading="saving" @click="save">保存设置</el-button>
    <el-divider />
    <el-form inline>
      <el-form-item label="测试用户名">
        <el-input v-model="testUsername" placeholder="已注册用户 username" />
      </el-form-item>
      <el-form-item>
        <el-button :loading="testing" @click="sendTest">发送测试邮件</el-button>
      </el-form-item>
    </el-form>
  </el-card>
</template>

<script setup lang="ts">
import { reactive, ref } from "vue";
import { ApiClient } from "../../lib/api";
import { settingsState } from "../../lib/settingsStore";
import { authState } from "../../lib/authStore";
import { Message } from "@element-plus/icons-vue";

const saving = ref(false);
const testing = ref(false);
const error = ref("");
const success = ref("");
const smtpPass = ref("");
const testUsername = ref("");

const form = reactive({
  smtp_host: "",
  smtp_port: 587,
  smtp_user: "",
  from_email: "",
  from_name: "HIT-AIOT-OPS团队",
});

async function load() {
  const client = new ApiClient(settingsState.baseUrl, { csrfToken: authState.csrfToken });
  const r = await client.adminGetMailSettings();
  form.smtp_host = r.smtp_host ?? "";
  form.smtp_port = r.smtp_port || 587;
  form.smtp_user = r.smtp_user ?? "";
  form.from_email = r.from_email ?? "";
  form.from_name = r.from_name ?? "HIT-AIOT-OPS团队";
}

async function save() {
  error.value = "";
  success.value = "";
  saving.value = true;
  try {
    const client = new ApiClient(settingsState.baseUrl, { csrfToken: authState.csrfToken });
    await client.adminSetMailSettings({
      ...form,
      smtp_pass: smtpPass.value,
      update_pass: !!smtpPass.value,
    });
    success.value = "保存成功";
    smtpPass.value = "";
    await load();
  } catch (e: any) {
    error.value = e?.body ? `${e.message}\n${e.body}` : (e?.message ?? String(e));
  } finally {
    saving.value = false;
  }
}

async function sendTest() {
  error.value = "";
  success.value = "";
  if (!testUsername.value.trim()) {
    error.value = "请填写测试用户名";
    return;
  }
  testing.value = true;
  try {
    const client = new ApiClient(settingsState.baseUrl, { csrfToken: authState.csrfToken });
    const r = await client.adminMailTest(testUsername.value.trim());
    success.value = `测试邮件已发送到 ${r.email}`;
  } catch (e: any) {
    error.value = e?.body ? `${e.message}\n${e.body}` : (e?.message ?? String(e));
  } finally {
    testing.value = false;
  }
}

load().catch((e: any) => {
  error.value = e?.body ? `${e.message}\n${e.body}` : (e?.message ?? String(e));
});
</script>

<style scoped>
.card { max-width: 980px; }
.head { display: flex; align-items: center; gap: 8px; font-weight: 700; }
.mb { margin-bottom: 12px; }
</style>
