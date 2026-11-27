import { createRouter, createWebHistory } from "vue-router";
import { useAuthStore } from "@/store/auth";
import { onUnauthorized } from "@/api/client";

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: "/login",
      name: "login",
      component: () => import("@/views/LoginView.vue"),
      meta: { public: true },
    },
    {
      path: "/",
      component: () => import("@/layout/MainLayout.vue"),
      children: [
        { path: "", redirect: "/users" },
        { path: "svnserve", name: "svnserve", component: () => import("@/views/SvnserveView.vue") },
        { path: "users", name: "users", component: () => import("@/views/UserListView.vue") },
        { path: "groups", name: "groups", component: () => import("@/views/GroupListView.vue") },
        { path: "permissions", name: "permissions", component: () => import("@/views/PermissionView.vue") },
        { path: "files", name: "files", component: () => import("@/views/RawFilesView.vue") },
        { path: "audits", name: "audits", component: () => import("@/views/AuditListView.vue") },
      ],
    },
    { path: "/:pathMatch(.*)*", redirect: "/users" },
  ],
});

onUnauthorized(() => {
  const auth = useAuthStore();
  auth.clearSession();
  if (router.currentRoute.value.name !== "login") {
    void router.replace({ name: "login", query: { expired: "1" } });
  }
});

router.beforeEach(async (to) => {
  const auth = useAuthStore();
  if (!auth.loaded) {
    await auth.restore();
  }
  if (!to.meta.public && !auth.username) {
    return { name: "login", query: { redirect: to.fullPath } };
  }
  if (to.name === "login" && auth.username) {
    return { name: "users" };
  }
  return true;
});

export default router;
