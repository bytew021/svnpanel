<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { errorMessage, listPaths } from "@/api/client";
import type { PathNode } from "@/types";
import { failure } from "@/utils/notify";

const props = defineProps<{
  open: boolean;
  repo: string;
  initialPath: string;
}>();

const emit = defineEmits<{
  close: [];
  select: [path: string];
}>();

const currentPath = ref("/");
const selectedPath = ref("/");
const nodes = ref<PathNode[]>([]);
const loading = ref(false);

const breadcrumbs = computed(() => {
  if (currentPath.value === "/") return [{ label: props.repo || "仓库", path: "/" }];
  const parts = currentPath.value.split("/").filter(Boolean);
  return [
    { label: props.repo || "仓库", path: "/" },
    ...parts.map((part, index) => ({
      label: part,
      path: "/" + parts.slice(0, index + 1).join("/"),
    })),
  ];
});

const parentPath = computed(() => {
  if (currentPath.value === "/") return "/";
  const parts = currentPath.value.split("/").filter(Boolean);
  parts.pop();
  return parts.length ? "/" + parts.join("/") : "/";
});

async function load(path: string) {
  if (!props.repo) {
    nodes.value = [];
    return;
  }
  loading.value = true;
  try {
    const result = await listPaths(props.repo, path);
    nodes.value = result.items;
  } catch (error) {
    failure(errorMessage(error));
    nodes.value = [];
  } finally {
    loading.value = false;
  }
}

async function navigate(path: string, selectCurrent = true) {
  currentPath.value = path || "/";
  if (selectCurrent) selectedPath.value = currentPath.value;
  await load(currentPath.value);
}

function choose(path: string) {
  selectedPath.value = path;
}

function confirmSelection() {
  emit("select", selectedPath.value);
  emit("close");
}

watch(
  () => props.open,
  (open) => {
    if (!open) return;
    currentPath.value = "/";
    selectedPath.value = props.initialPath || "/";
    void load("/");
  },
  { immediate: true },
);
</script>

<template>
  <Teleport to="body">
    <div v-if="open" class="path-picker-mask" @mousedown.self="emit('close')">
      <section class="path-picker-panel">
        <header class="flex h-14 items-center justify-between border-b border-slate-200 px-6">
          <h2 class="text-base font-semibold text-slate-800">选择路径</h2>
          <button class="text-2xl text-slate-400 hover:text-slate-700" @click="emit('close')">×</button>
        </header>

        <div class="flex items-center gap-1 border-b border-slate-100 px-6 py-3 text-sm">
          <template v-for="(crumb, index) in breadcrumbs" :key="crumb.path">
            <button class="rounded px-2 py-1 text-blue-600 hover:bg-blue-50" @click="navigate(crumb.path)">
              {{ crumb.label }}
            </button>
            <span v-if="index < breadcrumbs.length - 1" class="text-slate-300">/</span>
          </template>
        </div>

        <div class="path-picker-tree">
          <button
            class="path-picker-row border-b border-slate-100 text-slate-500"
            :class="currentPath === '/' && selectedPath === '/' ? 'bg-blue-50' : ''"
            @click="choose(currentPath)"
          >
            <span class="w-5 text-center" :class="selectedPath === currentPath ? 'text-blue-600' : 'text-slate-300'">●</span>
            <span>选择当前目录：{{ currentPath }}</span>
          </button>
          <button
            v-if="currentPath !== '/'"
            class="path-picker-row border-b border-slate-100 text-slate-600"
            @click="navigate(parentPath)"
          >
            <span class="w-5 text-center text-blue-500">↩</span>
            <span>返回上级</span>
          </button>

          <div v-if="loading" class="py-12 text-center text-sm text-slate-400">正在读取目录…</div>
          <template v-else>
            <div
              v-for="node in nodes"
              :key="node.path"
              class="path-picker-row border-b border-slate-100"
              :class="selectedPath === node.path ? 'bg-blue-50' : ''"
            >
              <input
                type="radio"
                name="selected-path"
                class="h-4 w-4"
                :checked="selectedPath === node.path"
                @change="choose(node.path)"
              />
              <button class="flex min-w-0 flex-1 items-center gap-2 text-left" @click="choose(node.path)">
                <span class="text-lg text-blue-500">▸</span>
                <span class="truncate">{{ node.name }}</span>
              </button>
              <button class="text-xs text-blue-600 hover:underline" @click="navigate(node.path)">进入</button>
            </div>
            <div v-if="!nodes.length" class="py-12 text-center text-sm text-slate-400">当前目录没有子目录</div>
          </template>
        </div>

        <footer class="flex items-center justify-end gap-2 border-t border-slate-200 px-6 py-4">
          <button class="btn btn-secondary" @click="emit('close')">取消</button>
          <button class="btn btn-primary" @click="confirmSelection">确定</button>
        </footer>
      </section>
    </div>
  </Teleport>
</template>

<style scoped>
.path-picker-mask {
  position: fixed;
  z-index: 70;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgb(15 23 42 / 38%);
  backdrop-filter: blur(1px);
}

.path-picker-panel {
  display: flex;
  width: min(46rem, calc(100vw - 3rem));
  height: min(42rem, calc(100vh - 4rem));
  flex-direction: column;
  overflow: hidden;
  border-radius: 8px;
  background: #fff;
  box-shadow: 0 24px 70px rgb(15 23 42 / 24%);
}

.path-picker-tree {
  flex: 1;
  overflow: auto;
  padding: 0 1.5rem;
}

.path-picker-row {
  display: flex;
  width: 100%;
  min-height: 40px;
  align-items: center;
  gap: 0.75rem;
  padding: 0.45rem 0.75rem;
  text-align: left;
}

.path-picker-row:hover {
  background: #f8fafc;
}
</style>
