<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import BaseDialog from "@/components/BaseDialog.vue";
import {
  deleteACL,
  effectiveACL,
  errorMessage,
  getACL,
  listGroups,
  listPaths,
  listRepositories,
  listUsers,
  setACL,
} from "@/api/client";
import { useTableMaxHeight } from "@/composables/useTableMaxHeight";
import type {
  ACLEntry,
  EffectiveAccess,
  Group,
  IdentityKind,
  PathNode,
  Permission,
  Repository,
  Revisions,
  User,
} from "@/types";
import { confirmAction, failure, success } from "@/utils/notify";

const repositories = ref<Repository[]>([]);
const users = ref<User[]>([]);
const groups = ref<Group[]>([]);
const selectedRepo = ref("");
const currentPath = ref("/");
const manualPath = ref("/");
const manualPathOpen = ref(false);
const paths = ref<PathNode[]>([]);
const acls = ref<ACLEntry[]>([]);
const revisions = ref<Revisions>({ passwd: "", authz: "" });
const loading = ref(false);
const pathLoading = ref(false);
const effectiveUser = ref("");
const effective = ref<EffectiveAccess>();
const ruleDialog = reactive({
  open: false,
  editing: false,
  kind: "user" as IdentityKind,
  name: "",
  permission: "read" as Permission,
});
const { tableMaxHeight } = useTableMaxHeight(390);

const breadcrumbs = computed(() => {
  if (currentPath.value === "/") return [{ label: "仓库根目录", path: "/" }];
  const parts = currentPath.value.split("/").filter(Boolean);
  return [
    { label: "仓库根目录", path: "/" },
    ...parts.map((part, index) => ({
      label: part,
      path: "/" + parts.slice(0, index + 1).join("/"),
    })),
  ];
});

const identityOptions = computed(() => {
  if (ruleDialog.kind === "user") {
    return users.value.map((user) => ({
      name: user.name,
      label: user.name,
      hint: user.disabled ? "已禁用" : "",
    }));
  }
  if (ruleDialog.kind === "group") {
    return groups.value.map((group) => ({
      name: group.name,
      label: "@" + group.name,
      hint: `${group.members.length} 人`,
    }));
  }
  return [];
});

const needsName = computed(() => ruleDialog.kind === "user" || ruleDialog.kind === "group");

async function bootstrap() {
  loading.value = true;
  try {
    const [repoItems, userResult, groupResult] = await Promise.all([
      listRepositories(),
      listUsers(),
      listGroups(),
    ]);
    repositories.value = repoItems ?? [];
    users.value = userResult.items;
    groups.value = groupResult.items;
    revisions.value = groupResult.revisions;
    if (repositories.value.length) {
      selectedRepo.value = repositories.value[0].name;
      await navigate("/");
    }
  } catch (error) {
    failure(errorMessage(error));
  } finally {
    loading.value = false;
  }
}

async function navigate(path: string) {
  currentPath.value = path || "/";
  manualPath.value = currentPath.value;
  pathLoading.value = true;
  try {
    const [pathResult, aclResult] = await Promise.all([
      listPaths(selectedRepo.value, currentPath.value),
      getACL(selectedRepo.value, currentPath.value),
    ]);
    paths.value = pathResult.items;
    acls.value = aclResult.items;
    revisions.value = aclResult.revisions;
    effective.value = undefined;
  } catch (error) {
    failure(errorMessage(error));
  } finally {
    pathLoading.value = false;
  }
}

async function submitManualPath() {
  await navigate(manualPath.value);
  manualPathOpen.value = false;
}

async function changeRepository() {
  await navigate("/");
}

function toggleManualPath() {
  manualPath.value = currentPath.value;
  manualPathOpen.value = !manualPathOpen.value;
}

function parentPath() {
  if (currentPath.value === "/") return "/";
  const parts = currentPath.value.split("/").filter(Boolean);
  parts.pop();
  return parts.length ? "/" + parts.join("/") : "/";
}

async function refreshACL() {
  if (!selectedRepo.value) return;
  try {
    const result = await getACL(selectedRepo.value, currentPath.value);
    acls.value = result.items;
    revisions.value = result.revisions;
  } catch (error) {
    failure(errorMessage(error));
  }
}

function openAddRule() {
  Object.assign(ruleDialog, {
    open: true,
    editing: false,
    kind: "user",
    name: "",
    permission: "read",
  });
}

function openEditRule(entry: ACLEntry) {
  if (entry.unsupported) return;
  Object.assign(ruleDialog, {
    open: true,
    editing: true,
    kind: entry.identity.kind,
    name: entry.identity.name ?? "",
    permission: entry.permission,
  });
}

function closeRuleDialog() {
  ruleDialog.open = false;
}

function changeRuleKind() {
  ruleDialog.name = "";
}

async function saveRule() {
  if (!selectedRepo.value) return;
  if (needsName.value && !ruleDialog.name) {
    failure("请选择用户或分组");
    return;
  }
  try {
    const result = await setACL(
      revisions.value,
      selectedRepo.value,
      currentPath.value,
      ruleDialog.kind,
      ruleDialog.name,
      ruleDialog.permission,
    );
    revisions.value = result.revisions;
    success("权限已发布");
    closeRuleDialog();
    await refreshACL();
    if (effectiveUser.value) await previewEffective();
  } catch (error) {
    failure(errorMessage(error));
    await refreshACL();
  }
}

async function removeRule(entry: ACLEntry) {
  if (entry.unsupported) return;
  const name = entry.rawIdentity || "该身份";
  if (!(await confirmAction(`移除 ${name} 的权限？`, `路径：${selectedRepo.value}:${currentPath.value}`, true))) return;
  try {
    const result = await deleteACL(
      revisions.value,
      selectedRepo.value,
      currentPath.value,
      entry.identity.kind,
      entry.identity.name ?? "",
    );
    revisions.value = result.revisions;
    success("权限已移除");
    await refreshACL();
  } catch (error) {
    failure(errorMessage(error));
    await refreshACL();
  }
}

async function previewEffective() {
  if (!selectedRepo.value || !effectiveUser.value) {
    effective.value = undefined;
    return;
  }
  try {
    effective.value = await effectiveACL(selectedRepo.value, currentPath.value, effectiveUser.value);
  } catch (error) {
    failure(errorMessage(error));
  }
}

function permissionLabel(permission: Permission) {
  return { none: "无权限", read: "只读", write: "读写", unsupported: "未知" }[permission];
}

function identityLabel(entry: ACLEntry) {
  if (entry.identity.kind === "group") return "@" + entry.identity.name;
  if (entry.identity.kind === "all") return "* · 所有用户";
  if (entry.identity.kind === "authenticated") return "$authenticated · 已认证";
  if (entry.identity.kind === "anonymous") return "$anonymous · 匿名";
  return entry.rawIdentity;
}

onMounted(bootstrap);
</script>

<template>
  <div class="space-y-4">
    <section class="card flex flex-wrap items-center gap-3 px-4 py-3">
      <div>
        <div class="text-sm font-bold text-slate-800">路径权限</div>
        <div class="mt-1 text-xs text-slate-500">选择仓库和目录后，管理用户、分组及内置身份的访问权限</div>
      </div>
      <div class="ml-auto flex items-center gap-3">
        <span class="text-xs text-slate-500">最近版本 r{{ repositories.find((item) => item.name === selectedRepo)?.revision ?? "-" }}</span>
        <vxe-select v-model="selectedRepo" style="width: 220px" @change="changeRepository">
          <vxe-option v-for="repo in repositories" :key="repo.name" :value="repo.name" :label="repo.name" />
        </vxe-select>
        <vxe-button @click="bootstrap">刷新仓库</vxe-button>
      </div>
    </section>

    <section class="rounded-md border border-blue-200 bg-blue-50 px-4 py-3 text-sm text-blue-800">
      SVN 权限会从当前目录向父目录查找最近规则；子目录没有单独配置时，会继承父目录权限。
    </section>

    <div v-if="!repositories.length" class="card empty">
      仓库根目录下未发现可管理的 SVN 仓库
    </div>

    <div v-else class="grid grid-cols-[19rem_minmax(0,1fr)] gap-4">
      <section class="card flex min-h-[520px] flex-col overflow-hidden">
        <header class="border-b border-slate-100 px-4 py-3">
          <div class="mb-2 flex items-center gap-2">
            <vxe-button size="small" :disabled="currentPath === '/'" @click="navigate(parentPath())">返回</vxe-button>
            <span class="truncate font-mono text-xs text-slate-600">{{ currentPath }}</span>
          </div>
          <button class="text-xs text-blue-600 hover:underline" @click="toggleManualPath">
            {{ manualPathOpen ? "收起手动输入" : "无法从目录树找到？手动输入路径" }}
          </button>
          <form v-if="manualPathOpen" class="mt-2 flex gap-2" @submit.prevent="submitManualPath">
            <input v-model="manualPath" class="input py-1.5 font-mono text-xs" placeholder="/trunk" />
            <button class="btn btn-secondary px-2 py-1.5">跳转</button>
          </form>
        </header>
        <div class="flex-1 overflow-auto p-2">
          <div v-if="pathLoading" class="empty text-sm">正在读取目录…</div>
          <button
            v-for="node in paths"
            :key="node.path"
            class="flex w-full items-center gap-2 rounded-md px-3 py-2.5 text-left text-sm hover:bg-blue-50 hover:text-blue-700"
            @click="navigate(node.path)"
          >
            <span class="text-blue-500">▸</span>
            <span class="truncate font-mono">{{ node.name }}</span>
          </button>
          <div v-if="!pathLoading && !paths.length" class="empty text-sm">当前目录没有子目录</div>
        </div>
      </section>

      <div class="space-y-4">
        <section class="card overflow-hidden">
          <header class="flex flex-wrap items-center justify-between gap-3 border-b border-slate-100 px-4 py-3">
            <div>
              <div class="flex flex-wrap items-center gap-1 text-sm font-semibold text-slate-800">
                <template v-for="(crumb, index) in breadcrumbs" :key="crumb.path">
                  <button class="rounded px-1.5 py-1 font-mono hover:bg-slate-100" @click="navigate(crumb.path)">{{ crumb.label }}</button>
                  <span v-if="index < breadcrumbs.length - 1" class="text-slate-300">/</span>
                </template>
              </div>
              <div class="mt-1 text-xs text-slate-500">当前目录的显式权限</div>
            </div>
            <button class="btn btn-primary" @click="openAddRule">+ 添加权限</button>
          </header>
          <vxe-table
            :data="acls"
            :loading="loading"
            border
            stripe
            show-overflow
            empty-text="当前目录没有额外权限规则，将继承父目录权限"
            :max-height="tableMaxHeight"
            auto-resize
            :virtual-x-config="{ enabled: true, gt: 8 }"
            :virtual-y-config="{ enabled: true, gt: 20, oSize: 8 }"
            :row-config="{ isHover: true, height: 44 }"
            :column-config="{ resizable: true }"
          >
            <vxe-column title="身份" min-width="220">
              <template #default="{ row }">
                <button class="font-mono font-semibold text-slate-700 hover:text-blue-700" @click="openEditRule(row)">
                  {{ identityLabel(row) }}
                </button>
                <span v-if="row.unsupported" class="badge badge-amber ml-2">原样保留</span>
              </template>
            </vxe-column>
            <vxe-column title="权限" width="120" align="center">
              <template #default="{ row }">
                <span class="badge" :class="row.permission === 'none' ? 'badge-gray' : row.permission === 'write' ? 'badge-green' : 'badge-amber'">
                  {{ permissionLabel(row.permission) }}
                </span>
              </template>
            </vxe-column>
            <vxe-column field="rawValue" title="权限值" width="100" align="center">
              <template #default="{ row }"><code class="text-xs">{{ row.rawValue || "（空）" }}</code></template>
            </vxe-column>
            <vxe-column title="操作" width="140" align="center">
              <template #default="{ row }">
                <vxe-button size="small" mode="text" status="primary" :disabled="row.unsupported" @click="openEditRule(row)">编辑</vxe-button>
                <vxe-button size="small" mode="text" status="danger" :disabled="row.unsupported" @click="removeRule(row)">移除</vxe-button>
              </template>
            </vxe-column>
          </vxe-table>
        </section>

        <section class="card p-4">
          <div class="flex flex-wrap items-center gap-3">
            <div>
              <div class="text-sm font-bold text-slate-800">有效权限预览</div>
              <div class="mt-1 text-xs text-slate-500">查看某个用户最终继承到的权限</div>
            </div>
            <vxe-select v-model="effectiveUser" class="ml-auto" style="width: 220px" @change="previewEffective">
              <vxe-option value="" label="选择 SVN 用户" />
              <vxe-option v-for="user in users" :key="user.name" :value="user.name" :label="user.name" />
            </vxe-select>
          </div>
          <div v-if="effective" class="mt-3 rounded-lg border border-slate-200 bg-slate-50 p-4">
            <div class="flex flex-wrap items-center gap-3">
              <span class="text-sm text-slate-600">{{ effective.user }} 在 {{ effective.path }}</span>
              <span class="badge" :class="effective.permission === 'none' ? 'badge-gray' : effective.permission === 'write' ? 'badge-green' : 'badge-amber'">
                {{ permissionLabel(effective.permission) }}
              </span>
              <span v-if="effective.sourcePath" class="text-xs text-slate-500">来源：{{ effective.sourcePath }}</span>
            </div>
            <div v-if="effective.sources.length" class="mt-2 font-mono text-xs text-slate-500">{{ effective.sources.join(" · ") }}</div>
            <div v-if="!effective.supported" class="mt-3 rounded-md bg-amber-50 px-3 py-2 text-xs text-amber-800">
              {{ (effective.warnings ?? []).join("；") || "存在未知 authz 语法，预览可能不完整" }}
            </div>
          </div>
        </section>
      </div>
    </div>

    <BaseDialog
      :open="ruleDialog.open"
      :title="ruleDialog.editing ? `编辑权限 · ${selectedRepo}:${currentPath}` : `添加权限 · ${selectedRepo}:${currentPath}`"
      @close="closeRuleDialog"
    >
      <div class="space-y-5">
        <template v-if="!ruleDialog.editing">
          <label class="block">
            <span class="mb-2 block text-sm font-semibold text-slate-700">身份类型</span>
            <vxe-select v-model="ruleDialog.kind" style="width: 100%" @change="changeRuleKind">
              <vxe-option value="user" label="SVN 用户" />
              <vxe-option value="group" label="SVN 分组" />
              <vxe-option value="all" label="* 所有用户" />
              <vxe-option value="authenticated" label="$authenticated 已认证用户" />
              <vxe-option value="anonymous" label="$anonymous 匿名用户" />
            </vxe-select>
          </label>
          <label v-if="needsName" class="block">
            <span class="mb-2 block text-sm font-semibold text-slate-700">{{ ruleDialog.kind === "user" ? "用户" : "分组" }}</span>
            <vxe-select v-model="ruleDialog.name" style="width: 100%">
              <vxe-option value="" :label="ruleDialog.kind === 'user' ? '请选择用户' : '请选择分组'" />
              <vxe-option v-for="option in identityOptions" :key="option.name" :value="option.name" :label="option.label + (option.hint ? ` · ${option.hint}` : '')" />
            </vxe-select>
          </label>
          <div v-else class="rounded-md border border-blue-200 bg-blue-50 px-3 py-2 text-sm text-blue-700">
            已选择内置身份
          </div>
        </template>
        <div v-else class="rounded-md bg-slate-50 px-3 py-2 font-mono text-sm text-slate-700">
          {{ ruleDialog.kind === "group" ? "@" : "" }}{{ ruleDialog.name || ruleDialog.kind }}
        </div>

        <div>
          <div class="mb-2 text-sm font-semibold text-slate-700">权限</div>
          <div class="inline-flex overflow-hidden rounded-md border border-blue-500">
            <button
              v-for="option in [{ value: 'write', label: '读写' }, { value: 'read', label: '只读' }, { value: 'none', label: '无权限' }]"
              :key="option.value"
              class="px-5 py-2 text-sm"
              :class="ruleDialog.permission === option.value ? 'bg-blue-500 text-white' : 'bg-white text-slate-600 hover:bg-blue-50'"
              @click="ruleDialog.permission = option.value as Permission"
            >
              {{ option.label }}
            </button>
          </div>
        </div>

        <div class="flex justify-end gap-2">
          <button class="btn btn-secondary" @click="closeRuleDialog">取消</button>
          <button class="btn btn-primary" @click="saveRule">保存</button>
        </div>
      </div>
    </BaseDialog>
  </div>
</template>
