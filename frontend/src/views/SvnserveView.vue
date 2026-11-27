<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { errorMessage, getSvnserveInfo } from "@/api/client";
import type { SvnserveFileInfo, SvnserveInfo } from "@/types";
import { failure } from "@/utils/notify";

const info = ref<SvnserveInfo>();
const loading = ref(false);

const statusLabel = computed(() => {
  if (!info.value) return "读取中";
  if (info.value.portOpen && info.value.available) return "端口可连接";
  if (info.value.portOpen) return "端口已监听";
  return "端口未监听";
});

const statusClass = computed(() => {
  if (!info.value) return "badge-gray";
  return info.value.portOpen ? "badge-green" : "badge-amber";
});

async function load() {
  loading.value = true;
  try {
    info.value = await getSvnserveInfo();
  } catch (error) {
    failure(errorMessage(error));
  } finally {
    loading.value = false;
  }
}

function formatTime(value?: string) {
  if (!value) return "—";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString("zh-CN", { hour12: false });
}

function formatBytes(value?: number) {
  if (value === undefined) return "—";
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  return `${(value / 1024 / 1024).toFixed(1)} MB`;
}

function fileState(file: SvnserveFileInfo) {
  return file.exists ? `${file.mode ?? "—"} · ${formatBytes(file.size)}` : "文件不存在";
}

onMounted(load);
</script>

<template>
  <div class="space-y-5">
    <section class="card overflow-hidden">
      <header class="flex flex-wrap items-center justify-between gap-4 border-b border-slate-200 px-5 py-5">
        <div class="flex items-center gap-4">
          <div class="grid h-12 w-12 place-items-center rounded-xl bg-slate-900 font-mono text-lg font-bold text-emerald-300">
            SVN
          </div>
          <div>
            <div class="flex items-center gap-3">
              <h1 class="text-lg font-bold text-slate-900">SVNserve 服务信息</h1>
              <span class="badge" :class="statusClass">{{ statusLabel }}</span>
            </div>
            <p class="mt-1 text-xs text-slate-500">
              当前二进制、监听端口、共享认证文件及仓库状态
            </p>
          </div>
        </div>
        <button class="btn btn-secondary" :disabled="loading" @click="load">
          {{ loading ? "刷新中…" : "刷新状态" }}
        </button>
      </header>

      <div v-if="info" class="grid gap-px bg-slate-200 sm:grid-cols-2 xl:grid-cols-4">
        <div class="bg-white p-5">
          <div class="text-xs font-semibold uppercase tracking-wide text-slate-400">可执行文件</div>
          <div class="mt-2 break-all font-mono text-sm font-semibold text-slate-800">{{ info.executable }}</div>
          <div class="mt-1 break-all text-xs text-slate-500">{{ info.resolvedPath || "未找到" }}</div>
        </div>
        <div class="bg-white p-5">
          <div class="text-xs font-semibold uppercase tracking-wide text-slate-400">版本</div>
          <div class="mt-2 text-sm font-semibold text-slate-800">{{ info.version || "未知" }}</div>
          <div class="mt-1 text-xs text-slate-500">检查于 {{ formatTime(info.checkedAt) }}</div>
        </div>
        <div class="bg-white p-5">
          <div class="text-xs font-semibold uppercase tracking-wide text-slate-400">监听端口</div>
          <div class="mt-2 flex items-center gap-2">
            <span class="font-mono text-xl font-bold text-slate-900">{{ info.port }}</span>
            <span class="badge" :class="info.portOpen ? 'badge-green' : 'badge-amber'">
              {{ info.portOpen ? "可连接" : "不可连接" }}
            </span>
          </div>
          <div class="mt-1 font-mono text-xs text-slate-500">svn://&lt;服务器地址&gt;:{{ info.port }}</div>
        </div>
        <div class="bg-white p-5">
          <div class="text-xs font-semibold uppercase tracking-wide text-slate-400">仓库</div>
          <div class="mt-2 text-2xl font-bold text-slate-900">{{ info.repositories.length }}</div>
          <div class="mt-1 text-xs text-slate-500">最新 revision 合计 r{{ info.totalLatestRevision }}</div>
        </div>
      </div>
    </section>

    <section v-if="info?.error" class="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3">
      <div class="text-sm font-semibold text-amber-900">检查提示</div>
      <pre class="mt-2 whitespace-pre-wrap font-mono text-xs leading-6 text-amber-800">{{ info.error }}</pre>
    </section>

    <div v-if="info" class="grid gap-5 xl:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
      <section class="card overflow-hidden">
        <header class="border-b border-slate-200 px-5 py-4">
          <h2 class="text-sm font-bold text-slate-900">配置与认证文件</h2>
        </header>
        <dl class="divide-y divide-slate-100">
          <div class="px-5 py-4">
            <dt class="text-xs font-semibold text-slate-500">仓库根目录</dt>
            <dd class="mt-1 break-all font-mono text-sm text-slate-800">{{ info.repoRoot }}</dd>
          </div>
          <div class="px-5 py-4">
            <dt class="text-xs font-semibold text-slate-500">passwd</dt>
            <dd class="mt-1 break-all font-mono text-sm text-slate-800">{{ info.passwd.path }}</dd>
            <dd class="mt-1 text-xs" :class="info.passwd.exists ? 'text-slate-500' : 'text-red-600'">
              {{ fileState(info.passwd) }}
            </dd>
          </div>
          <div class="px-5 py-4">
            <dt class="text-xs font-semibold text-slate-500">authz</dt>
            <dd class="mt-1 break-all font-mono text-sm text-slate-800">{{ info.authz.path }}</dd>
            <dd class="mt-1 text-xs" :class="info.authz.exists ? 'text-slate-500' : 'text-red-600'">
              {{ fileState(info.authz) }}
            </dd>
          </div>
        </dl>
      </section>

      <section class="card overflow-hidden">
        <header class="flex items-center justify-between border-b border-slate-200 px-5 py-4">
          <div>
            <h2 class="text-sm font-bold text-slate-900">仓库 revision</h2>
            <p class="mt-1 text-xs text-slate-500">由 svnlook youngest 实时读取</p>
          </div>
          <span class="badge badge-gray">{{ info.repositories.length }} 个</span>
        </header>
        <div v-if="!info.repositories.length" class="empty text-sm">当前仓库根目录下没有可用仓库</div>
        <div v-else class="max-h-80 divide-y divide-slate-100 overflow-auto">
          <div
            v-for="repository in info.repositories"
            :key="repository.name"
            class="flex items-center justify-between px-5 py-3"
          >
            <span class="font-mono text-sm font-semibold text-slate-800">{{ repository.name }}</span>
            <span class="rounded bg-slate-100 px-2 py-1 font-mono text-xs text-slate-600">r{{ repository.revision }}</span>
          </div>
        </div>
      </section>
    </div>

    <details v-if="info?.versionOutput" class="card overflow-hidden">
      <summary class="cursor-pointer px-5 py-4 text-sm font-semibold text-slate-700 hover:bg-slate-50">
        svnserve --version 原始输出
      </summary>
      <pre class="max-h-80 overflow-auto border-t border-slate-200 bg-slate-950 p-5 font-mono text-xs leading-6 text-slate-200">{{ info.versionOutput }}</pre>
    </details>
  </div>
</template>
