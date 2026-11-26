<script setup lang="ts">
import { computed } from "vue";
import { useRoute, useRouter } from "vue-router";
import { menuConfig } from "./menuConfig";
import { useAuthStore } from "@/store/auth";

const route = useRoute();
const router = useRouter();
const auth = useAuthStore();
const current = computed(() => menuConfig.find((item) => route.path.startsWith(item.path)) ?? menuConfig[0]);

async function signOut() {
  await auth.logout();
  await router.replace("/login");
}
</script>

<template>
  <div class="flex h-screen overflow-hidden bg-[#f5f7fa]">
    <aside class="app-sidebar flex w-60 shrink-0 flex-col border-r border-slate-200 bg-white text-slate-600">
      <div class="flex h-16 items-center gap-3 border-b border-slate-200 px-5">
        <div class="grid h-9 w-9 place-items-center rounded-full bg-blue-600 text-lg font-black text-white">S</div>
        <div class="sidebar-brand-text">
          <div class="text-base font-bold text-slate-900">SVN 权限面板</div>
          <div class="text-[11px] text-slate-400">svnserve access control</div>
        </div>
      </div>
      <nav class="flex-1 space-y-1 px-3 py-4">
        <RouterLink
          v-for="item in menuConfig"
          :key="item.path"
          :to="item.path"
          :title="item.name"
          class="flex items-center gap-3 rounded-lg px-4 py-3 text-sm font-medium transition"
          :class="route.path.startsWith(item.path) ? 'bg-blue-50 text-blue-600' : 'hover:bg-slate-50 hover:text-slate-900'"
        >
          <span class="sidebar-dot h-2.5 w-2.5 rounded-full border-2" :class="route.path.startsWith(item.path) ? 'border-blue-500 bg-blue-500' : 'border-slate-400'" />
          <span class="sidebar-short text-sm font-bold">{{ item.name.slice(0, 1) }}</span>
          <span class="sidebar-item-label">{{ item.name }}</span>
        </RouterLink>
      </nav>
      <div class="sidebar-footer border-t border-slate-200 p-4 text-xs">
        <div class="sidebar-user mb-3 flex items-center justify-between">
          <span class="text-slate-400">当前管理员</span>
          <span class="font-semibold text-slate-700">{{ auth.username }}</span>
        </div>
        <button class="btn btn-ghost w-full text-slate-500 hover:bg-slate-100 hover:text-slate-800" title="退出登录" @click="signOut">
          <span class="sidebar-logout-full">退出登录</span>
          <span class="sidebar-logout-short text-lg">↪</span>
        </button>
      </div>
    </aside>

    <main class="min-w-0 flex-1 overflow-auto">
      <header class="app-main-header flex h-16 items-center justify-between border-b border-slate-200 bg-white px-7">
        <div class="flex items-center gap-2 text-sm">
          <span class="text-slate-400">首页</span>
          <span class="text-slate-300">/</span>
          <span class="font-semibold text-slate-800">{{ current.name }}</span>
        </div>
        <div class="flex items-center gap-3">
          <div class="grid h-8 w-8 place-items-center rounded-full bg-blue-50 text-sm font-bold text-blue-600">{{ auth.username.slice(0, 1).toUpperCase() }}</div>
          <span class="text-sm text-slate-700">{{ auth.username }}</span>
        </div>
      </header>
      <div class="app-content p-6">
        <RouterView />
      </div>
    </main>
  </div>
</template>
