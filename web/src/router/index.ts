import { createRouter, createWebHistory } from "vue-router";
import Layout from "../views/Layout.vue";
import Dashboard from "../views/pages/Dashboard.vue";
import UserBalance from "../views/pages/UserBalance.vue";
import UserUsage from "../views/pages/UserUsage.vue";
import AdminUsers from "../views/pages/AdminUsers.vue";
import AdminPrices from "../views/pages/AdminPrices.vue";
import AdminNodes from "../views/pages/AdminNodes.vue";
import AdminUsage from "../views/pages/AdminUsage.vue";
import AdminQueue from "../views/pages/AdminQueue.vue";

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: "/",
      component: Layout,
      children: [
        { path: "", component: Dashboard },
        { path: "user/balance", component: UserBalance },
        { path: "user/usage", component: UserUsage },
        { path: "admin/users", component: AdminUsers },
        { path: "admin/prices", component: AdminPrices },
        { path: "admin/nodes", component: AdminNodes },
        { path: "admin/usage", component: AdminUsage },
        { path: "admin/queue", component: AdminQueue },
      ],
    },
  ],
});

