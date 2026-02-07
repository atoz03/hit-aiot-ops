<template>
  <el-card>
    <template #header>
      <div class="row">
        <div>
          <div class="title">用户注册 / 账号登记</div>
          <div class="sub">账号绑定与开号申请都会进入审核队列；审核通过后才会用于扣费与 SSH 登录校验</div>
        </div>
        <el-button :loading="loading" type="primary" @click="refreshAll">刷新</el-button>
      </div>
    </template>

    <el-space direction="vertical" fill style="width: 100%">
      <el-alert v-if="error" :title="error" type="error" show-icon />
      <el-alert
        v-if="success"
        :title="success"
        type="success"
        show-icon
        :closable="true"
        @close="success = ''"
      />

      <el-form label-width="100px">
        <el-form-item label="计费账号">
          <el-input v-model="billingUsername" placeholder="例如 alice" @change="persistBilling" />
        </el-form-item>
      </el-form>

      <el-tabs v-model="activeTab">
        <el-tab-pane label="账号绑定登记" name="bind">
          <el-space direction="vertical" fill style="width: 100%">
            <el-alert
              title="登记说明：填写“端口(机器编号)”与该机器上的 Linux 用户名，提交后等待管理员审核。"
              type="info"
              show-icon
            />

            <el-table :data="bindItems" stripe style="width: 100%">
              <el-table-column label="端口(机器编号)" width="200">
                <template #default="{ row, $index }">
                  <el-select v-model="row.node_id" placeholder="选择端口" filterable style="width: 160px">
                    <el-option v-for="n in nodes" :key="n.node_id" :label="n.node_id" :value="n.node_id" />
                  </el-select>
                  <el-button link type="danger" style="margin-left: 8px" @click="removeBindRow($index)">删除</el-button>
                </template>
              </el-table-column>
              <el-table-column label="该机器用户名">
                <template #default="{ row }">
                  <el-input v-model="row.local_username" placeholder="例如 alice / train01" />
                </template>
              </el-table-column>
            </el-table>

            <div class="row-right">
              <el-button @click="addBindRow">新增一行</el-button>
              <el-button :loading="submitLoading" type="primary" @click="submitBind">提交登记</el-button>
            </div>
          </el-space>
        </el-tab-pane>

        <el-tab-pane label="开号申请" name="open">
          <el-space direction="vertical" fill style="width: 100%">
            <el-alert
              title="开号说明：当你在某台机器还没有账号时，可在此提交申请（仅记录与审核，不自动创建系统账号）。"
              type="info"
              show-icon
            />
            <el-form label-width="140px">
              <el-form-item label="端口(机器编号)">
                <el-select v-model="openNodeId" placeholder="选择端口" filterable style="width: 240px">
                  <el-option v-for="n in nodes" :key="n.node_id" :label="`${n.node_id}（${n.ip}）`" :value="n.node_id" />
                </el-select>
              </el-form-item>
              <el-form-item label="期望用户名">
                <el-input v-model="openLocalUsername" placeholder="建议与计费账号一致" />
              </el-form-item>
              <el-form-item label="备注">
                <el-input v-model="openMessage" placeholder="用途/导师/课题组等（可选）" type="textarea" :rows="3" />
              </el-form-item>
            </el-form>
            <div class="row-right">
              <el-button :loading="submitLoading" type="primary" @click="submitOpen">提交开号申请</el-button>
            </div>
          </el-space>
        </el-tab-pane>

        <el-tab-pane label="我的申请记录" name="list">
          <el-space direction="vertical" fill style="width: 100%">
            <el-alert
              title="提示：这里按“计费账号”查询申请记录；若计费账号为空将无法查询。"
              type="warning"
              show-icon
            />
            <el-table :data="requests" stripe height="420">
              <el-table-column prop="request_id" label="ID" width="80" />
              <el-table-column prop="request_type" label="类型" width="120" />
              <el-table-column prop="node_id" label="端口" width="120" />
              <el-table-column prop="local_username" label="机器用户名" width="160" />
              <el-table-column prop="status" label="状态" width="120" />
              <el-table-column prop="created_at" label="提交时间" min-width="180" />
              <el-table-column prop="reviewed_by" label="审核人" width="140" />
              <el-table-column prop="reviewed_at" label="审核时间" min-width="180" />
              <el-table-column prop="message" label="备注" min-width="220" />
            </el-table>
          </el-space>
        </el-tab-pane>

        <el-tab-pane label="端口列表" name="nodes">
          <el-space direction="vertical" fill style="width: 100%">
            <el-alert title="约定：节点上报 node_id 使用端口号（例如 60000）。" type="info" show-icon />
            <el-table :data="nodes" stripe height="420">
              <el-table-column prop="node_id" label="端口(机器编号)" width="180" />
              <el-table-column prop="ip" label="IP" width="220" />
            </el-table>
          </el-space>
        </el-tab-pane>
      </el-tabs>
    </el-space>
  </el-card>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { ApiClient, type UserRequest } from "../../lib/api";
import { setDefaultUsername, settingsState } from "../../lib/settingsStore";

type NodeRow = { node_id: string; ip: string };

const nodes: NodeRow[] = [
  { node_id: "60000", ip: "192.168.1.104" },
  { node_id: "60001", ip: "192.168.1.220" },
  { node_id: "60002", ip: "192.168.1.103" },
  { node_id: "60003", ip: "192.168.1.109" },
  { node_id: "60004", ip: "192.168.1.246" },
  { node_id: "60005", ip: "192.168.1.7" },
  { node_id: "60006", ip: "192.168.1.10" },
  { node_id: "60007", ip: "192.168.1.17" },
  { node_id: "60008", ip: "192.168.1.11" },
  { node_id: "60009", ip: "192.168.1.12" },
  { node_id: "60010", ip: "192.168.1.3" },
  { node_id: "60011", ip: "192.168.1.224" },
  { node_id: "60012", ip: "192.168.1.223" },
  { node_id: "60013", ip: "192.168.1.227" },
  { node_id: "60014", ip: "192.168.1.232" },
  { node_id: "60015", ip: "192.168.1.233" },
  { node_id: "60016", ip: "192.168.1.240" },
  { node_id: "60017", ip: "192.168.1.114" },
  { node_id: "60018", ip: "192.168.1.245" },
  { node_id: "60019", ip: "192.168.1.244" },
  { node_id: "60020", ip: "192.168.1.251" },
];

const loading = ref(false);
const submitLoading = ref(false);
const error = ref("");
const success = ref("");

const activeTab = ref<"bind" | "open" | "list" | "nodes">("bind");

const billingUsername = ref(settingsState.defaultUsername ?? "");
const bindItems = ref<Array<{ node_id: string; local_username: string }>>([{ node_id: "60000", local_username: "" }]);
const openNodeId = ref("60000");
const openLocalUsername = ref("");
const openMessage = ref("");

const requests = ref<UserRequest[]>([]);

function persistBilling() {
  if (billingUsername.value.trim()) setDefaultUsername(billingUsername.value.trim());
}

function addBindRow() {
  bindItems.value.push({ node_id: "60000", local_username: "" });
}

function removeBindRow(idx: number) {
  if (bindItems.value.length <= 1) return;
  bindItems.value.splice(idx, 1);
}

function normalizeBindItems(): Array<{ node_id: string; local_username: string }> {
  return bindItems.value
    .map((x) => ({ node_id: (x.node_id ?? "").trim(), local_username: (x.local_username ?? "").trim() }))
    .filter((x) => x.node_id && x.local_username);
}

async function submitBind() {
  error.value = "";
  success.value = "";
  const b = billingUsername.value.trim();
  const items = normalizeBindItems();
  if (!b) {
    error.value = "请先填写计费账号";
    return;
  }
  if (items.length === 0) {
    error.value = "请至少填写一条绑定登记（端口 + 机器用户名）";
    return;
  }
  submitLoading.value = true;
  try {
    const client = new ApiClient(settingsState.baseUrl);
    const r = await client.createBindRequests(b, items, "");
    success.value = `登记已提交，申请编号：${(r.request_ids ?? []).join(", ")}`;
    await refreshRequests();
    activeTab.value = "list";
  } catch (e: any) {
    error.value = e?.body ? `${e.message}\n${e.body}` : (e?.message ?? String(e));
  } finally {
    submitLoading.value = false;
  }
}

async function submitOpen() {
  error.value = "";
  success.value = "";
  const b = billingUsername.value.trim();
  const nodeId = openNodeId.value.trim();
  const local = openLocalUsername.value.trim();
  if (!b) {
    error.value = "请先填写计费账号";
    return;
  }
  if (!nodeId || !local) {
    error.value = "请填写端口与期望用户名";
    return;
  }
  submitLoading.value = true;
  try {
    const client = new ApiClient(settingsState.baseUrl);
    const r = await client.createOpenRequest(b, nodeId, local, openMessage.value ?? "");
    success.value = `开号申请已提交，申请编号：${r.request_id}`;
    await refreshRequests();
    activeTab.value = "list";
  } catch (e: any) {
    error.value = e?.body ? `${e.message}\n${e.body}` : (e?.message ?? String(e));
  } finally {
    submitLoading.value = false;
  }
}

async function refreshRequests() {
  const b = billingUsername.value.trim();
  requests.value = [];
  if (!b) return;
  const client = new ApiClient(settingsState.baseUrl);
  const r = await client.userRequests(b, 200);
  requests.value = r.requests ?? [];
}

async function refreshAll() {
  loading.value = true;
  error.value = "";
  try {
    await refreshRequests();
  } catch (e: any) {
    error.value = e?.body ? `${e.message}\n${e.body}` : (e?.message ?? String(e));
  } finally {
    loading.value = false;
  }
}

refreshAll();
</script>

<style scoped>
.row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.row-right {
  display: flex;
  justify-content: flex-end;
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

