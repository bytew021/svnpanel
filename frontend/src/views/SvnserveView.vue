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

const configStatusLabel = computed(() => {
  const config = info.value?.svnserveConfig;
  if (!config) return "读取中";
  if (!config.exists) return "未找到";
  if (!config.readable) return "不可读";
  if (config.passwordMatches && config.authzMatches && config.warnings.length === 0) {
    return "与面板一致";
  }
  return "需要检查";
});

const configStatusClass = computed(() => {
  const config = info.value?.svnserveConfig;
  if (!config) return "badge-gray";
  if (!config.exists || !config.readable) return "badge-amber";
  return config.passwordMatches && config.authzMatches && config.warnings.length === 0
    ? "badge-green"
    : "badge-amber";
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
  <div class="flex h-full min-h-0 flex-col gap-4">
    <section class="card shrink-0 overflow-hidden">
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

    <div class="svnserve-page-scroll min-h-0 flex-1 space-y-4 overflow-y-auto pr-1">
      <section v-if="info?.error" class="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3">
        <div class="text-sm font-semibold text-amber-900">检查提示</div>
        <pre class="mt-2 whitespace-pre-wrap font-mono text-xs leading-6 text-amber-800">{{ info.error }}</pre>
      </section>

      <section v-if="info" class="card overflow-hidden">
      <header class="flex flex-wrap items-center justify-between gap-3 border-b border-slate-200 px-5 py-4">
        <div>
          <div class="flex items-center gap-2">
            <h2 class="text-sm font-bold text-slate-900">svnserve 共享配置</h2>
            <span class="badge" :class="configStatusClass">{{ configStatusLabel }}</span>
          </div>
          <p class="mt-1 text-xs text-slate-500">
            检查 svnserve 实际读取的认证文件是否与面板管理的是同一份
          </p>
        </div>
        <span class="text-xs text-slate-500">只读查看</span>
      </header>

      <div class="grid gap-px bg-slate-200 sm:grid-cols-2 xl:grid-cols-4">
        <div class="bg-white p-4">
          <div class="text-xs font-semibold text-slate-500">配置文件</div>
          <div class="mt-2 break-all font-mono text-sm text-slate-800">{{ info.svnserveConfig.path }}</div>
          <div class="mt-1 text-xs" :class="info.svnserveConfig.exists ? 'text-slate-500' : 'text-amber-700'">
            {{ info.svnserveConfig.exists
              ? `${formatBytes(info.svnserveConfig.size)} · ${formatTime(info.svnserveConfig.modifiedAt)}`
              : "文件不存在" }}
          </div>
        </div>

        <div class="bg-white p-4">
          <div class="flex items-center justify-between gap-2">
            <div class="text-xs font-semibold text-slate-500">password-db</div>
            <span class="badge" :class="info.svnserveConfig.passwordMatches ? 'badge-green' : 'badge-amber'">
              {{ info.svnserveConfig.passwordMatches ? "一致" : "不一致" }}
            </span>
          </div>
          <div class="mt-2 break-all font-mono text-sm text-slate-800">{{ info.svnserveConfig.passwordDb || "未配置" }}</div>
          <div v-if="info.svnserveConfig.passwordDbResolved && info.svnserveConfig.passwordDbResolved !== info.svnserveConfig.passwordDb" class="mt-1 break-all text-xs text-slate-500">
            解析为 {{ info.svnserveConfig.passwordDbResolved }}
          </div>
          <div class="mt-1 break-all text-xs text-slate-500">面板：{{ info.passwd.path }}</div>
        </div>

        <div class="bg-white p-4">
          <div class="flex items-center justify-between gap-2">
            <div class="text-xs font-semibold text-slate-500">authz-db</div>
            <span class="badge" :class="info.svnserveConfig.authzMatches ? 'badge-green' : 'badge-amber'">
              {{ info.svnserveConfig.authzMatches ? "一致" : "不一致" }}
            </span>
          </div>
          <div class="mt-2 break-all font-mono text-sm text-slate-800">{{ info.svnserveConfig.authzDb || "未配置" }}</div>
          <div v-if="info.svnserveConfig.authzDbResolved && info.svnserveConfig.authzDbResolved !== info.svnserveConfig.authzDb" class="mt-1 break-all text-xs text-slate-500">
            解析为 {{ info.svnserveConfig.authzDbResolved }}
          </div>
          <div class="mt-1 break-all text-xs text-slate-500">面板：{{ info.authz.path }}</div>
        </div>

        <div class="bg-white p-4">
          <div class="text-xs font-semibold text-slate-500">realm 与访问策略</div>
          <div class="mt-2 font-mono text-sm font-semibold text-slate-800">{{ info.svnserveConfig.realm || "未配置 realm" }}</div>
          <div class="mt-1 text-xs text-slate-500">
            匿名：{{ info.svnserveConfig.anonAccess || "未配置" }} · 认证：{{ info.svnserveConfig.authAccess || "未配置" }}
          </div>
          <div class="mt-1 text-xs text-slate-500">修改此文件后需要重启 svnserve</div>
        </div>
      </div>

      <div v-if="info.svnserveConfig.warnings.length" class="border-t border-amber-200 bg-amber-50 px-5 py-4">
        <div class="text-sm font-semibold text-amber-900">检查提示</div>
        <ul class="mt-2 space-y-1 text-xs leading-5 text-amber-800">
          <li v-for="warning in info.svnserveConfig.warnings" :key="warning">• {{ warning }}</li>
        </ul>
      </div>

        <details v-if="info.svnserveConfig.readable" class="border-t border-slate-200">
          <summary class="cursor-pointer px-5 py-3 text-sm font-semibold text-slate-700 hover:bg-slate-50">
            查看 svnserve.conf 原始内容
          </summary>
          <pre class="whitespace-pre-wrap border-t border-slate-200 bg-slate-950 p-5 font-mono text-xs leading-6 text-slate-200">{{ info.svnserveConfig.content }}</pre>
        </details>
      </section>

      <div v-if="info" class="grid gap-4 xl:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
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
          <div v-else class="divide-y divide-slate-100">
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
        <pre class="whitespace-pre-wrap border-t border-slate-200 bg-slate-950 p-5 font-mono text-xs leading-6 text-slate-200">{{ info.versionOutput }}</pre>
      </details>
    </div>
  </div>
</template>

<style scoped>
.svnserve-page-scroll {
  overscroll-behavior: contain;
  scrollbar-color: #cbd5e1 transparent;
  scrollbar-gutter: stable;
  scrollbar-width: thin;
}

.svnserve-page-scroll::-webkit-scrollbar {
  width: 8px;
}

.svnserve-page-scroll::-webkit-scrollbar-thumb {
  border: 2px solid transparent;
  border-radius: 999px;
  background: #cbd5e1;
  background-clip: padding-box;
}

.svnserve-page-scroll::-webkit-scrollbar-track {
  background: transparent;
}
</style>
