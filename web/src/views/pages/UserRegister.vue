<template>
  <div class="page-wrap">
    <el-card class="form-card">
      <template #header>
        <div class="head">
          <h2>用户注册</h2>
          <p>请填写完整身份信息，注册后可登录并查看个人余额和用量。</p>
        </div>
      </template>

      <el-alert v-if="error" :title="error" type="error" show-icon class="mb" />
      <el-alert v-if="success" :title="success" type="success" show-icon class="mb" />

      <el-form label-position="top">
        <el-row :gutter="12">
          <el-col :span="12"><el-form-item label="邮箱"><el-input v-model="form.email" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="用户名"><el-input v-model="form.username" /></el-form-item></el-col>
        </el-row>
        <el-row :gutter="12">
          <el-col :span="12"><el-form-item label="密码"><el-input v-model="form.password" type="password" show-password /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="确认密码"><el-input v-model="confirmPassword" type="password" show-password /></el-form-item></el-col>
        </el-row>
        <el-row :gutter="12">
          <el-col :span="12"><el-form-item label="实际姓名"><el-input v-model="form.real_name" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="学号"><el-input v-model="form.student_id" /></el-form-item></el-col>
        </el-row>
        <el-row :gutter="12">
          <el-col :span="12"><el-form-item label="导师"><el-input v-model="form.advisor" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="预计毕业年份"><el-input-number v-model="form.expected_graduation_year" :min="2020" :max="2200" style="width: 100%" /></el-form-item></el-col>
        </el-row>
        <el-form-item label="电话"><el-input v-model="form.phone" /></el-form-item>
      </el-form>

      <el-button type="primary" :loading="loading" @click="submit" style="width: 100%">提交注册</el-button>

      <div class="links">
        <router-link to="/login">返回登录</router-link>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref } from "vue";
import { ApiClient } from "../../lib/api";
import { settingsState } from "../../lib/settingsStore";

const loading = ref(false);
const error = ref("");
const success = ref("");
const confirmPassword = ref("");
const form = reactive({
  email: "",
  username: "",
  password: "",
  real_name: "",
  student_id: "",
  advisor: "",
  expected_graduation_year: new Date().getFullYear() + 3,
  phone: "",
});

async function submit() {
  error.value = "";
  success.value = "";
  if (form.password !== confirmPassword.value) {
    error.value = "两次密码输入不一致";
    return;
  }
  loading.value = true;
  try {
    const client = new ApiClient(settingsState.baseUrl);
    await client.authRegister({ ...form });
    success.value = "注册成功，请返回登录";
  } catch (e: any) {
    error.value = e?.body ? `${e.message}\n${e.body}` : (e?.message ?? String(e));
  } finally {
    loading.value = false;
  }
}
</script>

<style scoped>
.page-wrap {
  min-height: 100vh;
  display: grid;
  place-items: center;
  background: #eef4ff;
  padding: 20px;
}
.form-card {
  width: 100%;
  max-width: 860px;
}
.head h2 {
  margin: 0;
}
.head p {
  margin: 6px 0 0;
  color: #475569;
}
.mb {
  margin-bottom: 12px;
}
.links {
  margin-top: 12px;
}
.links a {
  color: #1d4ed8;
  text-decoration: none;
}
</style>
