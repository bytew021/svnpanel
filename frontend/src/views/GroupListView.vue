<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from "vue";
import BaseDialog from "@/components/BaseDialog.vue";
import PathPickerDialog from "@/components/PathPickerDialog.vue";
import TablePager from "@/components/TablePager.vue";
import { useTableMaxHeight } from "@/composables/useTableMaxHeight";
import {
  createGroup,
  deleteGroup,
  errorMessage,
  listGroups,
  listRepositories,
  listUsers,
  setGroupMembers,
  setGroupPermissions,
} from "@/api/client";
import type { Group, GroupPermission, Permission, Repository, Revisions, User } from "@/types";
import { confirmAction, failure, success } from "@/utils/notify";

const groups = ref<Group[]>([]);
const users = ref<User[]>([]);
const repositories = ref<Repository[]>([]);
const revisions = ref<Revisions>({ passwd: "", authz: "" });
const loading = ref(false);
const page = ref(1);
const pageSize = ref(20);
const memberKeyword = ref("");
const dialog = reactive({
  open: false,
  editing: false,
  originalName: "",
  name: "",
  remark: "",
  members: [] as string[],
});
const busy = ref(false);
const permissionDialog = reactive({
  open: false,
  group: "",
  items: [] as GroupPermission[],
});
const pathPicker = reactive({
  open: false,
  repo: "",
  initialPath: "/",
  itemIndex: -1,
});
const { tableMaxHeight } = useTableMaxHeight(250);
const pageItems = computed(() =>
  pageSize.value === 0
    ? groups.value
    : groups.value.slice((page.value - 1) * pageSize.value, page.value * pageSize.value),
);
const filteredUsers = computed(() => {
  const keyword = memberKeyword.value.trim().toLowerCase();
  if (!keyword) return users.value;
  return users.value.filter(
    (user) =>
      user.name.toLowerCase().includes(keyword) ||
      (user.department ?? "").toLowerCase().includes(keyword) ||
      (user.remark ?? "").toLowerCase().includes(keyword),
  );
});

watch(
  () => groups.value.length,
  (total) => {
    const maxPage = pageSize.value === 0 ? 1 : Math.max(1, Math.ceil(total / pageSize.value));
    if (page.value > maxPage) page.value = maxPage;
  },
);

async function load() {
  loading.value = true;
  try {
    const [groupResult, userResult, repoItems] = await Promise.all([listGroups(), listUsers(), listRepositories()]);
    groups.value = groupResult.items;
    revisions.value = groupResult.revisions;
    users.value = userResult.items;
    repositories.value = repoItems ?? [];
  } catch (error) {
    failure(errorMessage(error));
  } finally {
    loading.value = false;
  }
}

function openCreate() {
  memberKeyword.value = "";
  Object.assign(dialog, { open: true, editing: false, originalName: "", name: "", remark: "", members: [] });
}

function openEdit(group: Group) {
  memberKeyword.value = "";
  Object.assign(dialog, {
    open: true,
    editing: true,
    originalName: group.name,
    name: group.name,
    remark: group.remark ?? "",
    members: [...group.members],
  });
}

function closeDialog() {
  dialog.open = false;
}

function openPermissions(group: Group) {
  permissionDialog.group = group.name;
  permissionDialog.items = group.permissions.map((item) => ({ ...item }));
  permissionDialog.open = true;
}

function closePermissions() {
  permissionDialog.open = false;
}

function addPermission() {
  permissionDialog.items.push({
    repo: repositories.value[0]?.name ?? "",
    path: "/",
    permission: "read",
  });
}

function removePermission(index: number) {
  permissionDialog.items.splice(index, 1);
}

function openPathPicker(item: GroupPermission, index: number) {
  pathPicker.repo = item.repo;
  pathPicker.initialPath = item.path;
  pathPicker.itemIndex = index;
  pathPicker.open = true;
}

function selectPath(value: string) {
  const item = permissionDialog.items[pathPicker.itemIndex];
  if (item) item.path = value;
  pathPicker.open = false;
}

async function savePermissions() {
  busy.value = true;
  try {
    const result = await setGroupPermissions(revisions.value, permissionDialog.group, permissionDialog.items);
    revisions.value = result.revisions;
    success("分组权限已保存");
    closePermissions();
    await load();
  } catch (error) {
    failure(errorMessage(error));
    await load();
  } finally {
    busy.value = false;
  }
}

function permissionLabel(permission: Permission) {
  return { none: "无权限", read: "只读", write: "读写", unsupported: "未知" }[permission];
}

async function submit() {
  busy.value = true;
  try {
    const result = dialog.editing
      ? await setGroupMembers(revisions.value, dialog.originalName, dialog.members, dialog.remark)
      : await createGroup(revisions.value, dialog.name.trim(), dialog.remark, dialog.members);
    revisions.value = result.revisions;
    success(dialog.editing ? "分组成员已更新" : "分组已创建");
    closeDialog();
    await load();
  } catch (error) {
    failure(errorMessage(error));
    await load();
  } finally {
    busy.value = false;
  }
}

async function remove(group: Group) {
  if (!(await confirmAction(`删除分组 ${group.name}？`, "仍被 ACL 引用的分组不能删除。", true))) return;
  try {
    const result = await deleteGroup(revisions.value, group.name);
    revisions.value = result.revisions;
    success("分组已删除");
    await load();
  } catch (error) {
    failure(errorMessage(error));
    await load();
  }
}

onMounted(load);
</script>

<template>
  <section class="card overflow-hidden">
    <header class="flex items-center justify-between border-b border-slate-100 px-5 py-4">
      <div>
        <span class="text-sm font-semibold text-slate-800">全局用户分组</span>
        <span class="ml-2 text-xs text-slate-500">分组只能包含当前 SVN 用户</span>
      </div>
      <button class="btn btn-primary" @click="openCreate">新增分组</button>
    </header>
    <vxe-table
      :data="pageItems"
      :loading="loading"
      border
      stripe
      show-overflow
      :max-height="tableMaxHeight"
      auto-resize
      :virtual-x-config="{ enabled: true, gt: 8 }"
      :virtual-y-config="{ enabled: true, gt: 20, oSize: 8 }"
      :row-config="{ isHover: true, height: 44 }"
      :column-config="{ resizable: true }"
    >
      <vxe-column type="seq" title="#" width="44" />
      <vxe-column field="name" title="分组名称" width="200">
        <template #default="{ row }">
          <span class="font-mono font-semibold">@{{ row.name }}</span>
          <span v-if="row.unsupported" class="badge badge-amber ml-2">含未知语法</span>
        </template>
      </vxe-column>
      <vxe-column field="remark" title="备注" width="230" show-overflow>
        <template #default="{ row }"><span :class="row.remark ? 'text-slate-700' : 'text-slate-400'">{{ row.remark || "—" }}</span></template>
      </vxe-column>
      <vxe-column title="权限" min-width="300">
        <template #default="{ row }">
          <div v-if="row.permissions.length" class="flex flex-wrap gap-1">
            <span
              v-for="permission in row.permissions"
              :key="`${permission.repo}:${permission.path}`"
              class="rounded bg-blue-50 px-1.5 py-0.5 font-mono text-[11px] text-blue-700"
              :title="`${permission.repo}:${permission.path} = ${permissionLabel(permission.permission)}`"
            >
              {{ repositories.length > 1 ? `${permission.repo}:` : "" }}{{ permission.path }} =
              {{ permission.permission === "write" ? "rw" : permission.permission === "read" ? "r" : "" }}
            </span>
          </div>
          <span v-else class="text-slate-400">—</span>
        </template>
      </vxe-column>
      <vxe-column title="成员" min-width="220">
        <template #default="{ row }">
          <div v-if="row.members.length" class="flex flex-wrap gap-1.5">
            <span v-for="member in row.members" :key="member" class="badge badge-gray font-mono">{{ member }}</span>
          </div>
          <span v-else class="text-sm text-slate-400">空分组</span>
        </template>
      </vxe-column>
      <vxe-column title="操作" width="220" align="center">
        <template #default="{ row }">
          <div class="flex justify-center gap-1">
            <vxe-button size="small" mode="text" status="primary" :disabled="row.unsupported" @click="openPermissions(row)">权限</vxe-button>
            <vxe-button size="small" mode="text" status="primary" :disabled="row.unsupported" @click="openEdit(row)">编辑</vxe-button>
            <vxe-button size="small" mode="text" status="danger" :disabled="row.unsupported" @click="remove(row)">删除</vxe-button>
          </div>
        </template>
      </vxe-column>
    </vxe-table>
    <TablePager
      v-model:page="page"
      v-model:page-size="pageSize"
      :total="groups.length"
      :sizes="[20, 50, 100, 0]"
    />
  </section>

  <BaseDialog :open="dialog.open" :title="dialog.editing ? `编辑分组 · ${dialog.originalName}` : '新增 SVN 分组'" @close="closeDialog">
    <form id="group-editor-form" class="space-y-5" @submit.prevent="submit">
      <label v-if="!dialog.editing" class="block">
        <span class="mb-2 block text-sm font-semibold text-slate-700">分组名称</span>
        <input v-model="dialog.name" class="input" maxlength="64" placeholder="例如 developers" />
      </label>
      <label class="block">
        <span class="mb-2 block text-sm font-semibold text-slate-700">分组备注</span>
        <input v-model="dialog.remark" class="input" maxlength="500" placeholder="例如：示例分组" />
        <span class="mt-1 block text-xs text-slate-500">保存为分组条目上方的 # svnpanel:remark 注释</span>
      </label>
      <div>
        <div class="mb-2 flex items-center justify-between gap-3">
          <div class="text-sm font-semibold text-slate-700">选择成员</div>
          <div class="text-xs text-slate-500">已选 {{ dialog.members.length }} 人</div>
        </div>
        <vxe-input
          v-model="memberKeyword"
          class="mb-2"
          style="width: 100%"
          placeholder="筛选用户名、部门或备注..."
          clearable
        />
        <div class="max-h-80 overflow-auto rounded-lg border border-slate-200 p-2">
          <label
            v-for="user in filteredUsers"
            :key="user.name"
            class="flex cursor-pointer items-start justify-between gap-3 rounded-md px-3 py-2 hover:bg-slate-50"
          >
            <span class="flex min-w-0 items-start gap-2">
              <input v-model="dialog.members" type="checkbox" :value="user.name" class="mt-0.5 h-4 w-4 shrink-0" />
              <span class="min-w-0">
                <span class="block truncate font-mono text-sm text-slate-800">{{ user.name }}</span>
                <span class="mt-0.5 block truncate text-xs text-slate-500">
                  {{ user.department || "未设置部门" }}-{{ user.remark || "无备注" }}
                </span>
              </span>
            </span>
            <span v-if="user.disabled" class="badge badge-gray shrink-0">已禁用</span>
          </label>
          <div v-if="!users.length" class="empty text-sm">还没有 SVN 用户</div>
          <div v-else-if="!filteredUsers.length" class="empty text-sm">没有匹配的成员</div>
        </div>
      </div>
    </form>
    <template #footer>
      <div class="flex justify-end gap-2">
        <button type="button" class="btn btn-secondary" @click="closeDialog">取消</button>
        <button
          type="submit"
          form="group-editor-form"
          class="btn btn-primary"
          :disabled="busy || (!dialog.editing && !dialog.name.trim())"
        >
          {{ busy ? "保存中…" : "保存" }}
        </button>
      </div>
    </template>
  </BaseDialog>

  <BaseDialog :open="permissionDialog.open" :title="`目录权限设定 · ${permissionDialog.group}`" wide @close="closePermissions">
    <div class="space-y-3">
      <div
        v-for="(item, index) in permissionDialog.items"
        :key="index"
        class="space-y-2 rounded-md bg-slate-50 p-2"
      >
        <div class="flex flex-wrap items-center gap-2">
          <button
            class="grid h-9 w-9 place-items-center rounded border border-rose-200 bg-rose-50 text-rose-500 hover:bg-rose-100"
            title="删除此路径权限"
            @click="removePermission(index)"
          >
            <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M3 6h18M8 6V4h8v2M19 6l-1 14H6L5 6M10 11v5M14 11v5" />
            </svg>
          </button>
          <vxe-select
            v-if="repositories.length > 1"
            v-model="item.repo"
            class="permission-repository-select min-w-0 flex-1"
            placeholder="选择仓库"
          >
            <vxe-option
              v-if="item.repo && !repositories.some((repo) => repo.name === item.repo)"
              :value="item.repo"
              :label="`${item.repo}（不存在）`"
            />
            <vxe-option v-for="repo in repositories" :key="repo.name" :value="repo.name" :label="repo.name" />
          </vxe-select>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <div class="flex min-w-52 flex-1 overflow-hidden rounded-md border border-slate-300 bg-white">
            <input v-model="item.path" class="min-w-0 flex-1 px-3 py-2 font-mono text-sm outline-none" placeholder="/path/to/directory" />
            <button class="border-l border-slate-300 bg-slate-50 px-4 text-sm text-slate-600 hover:bg-slate-100" @click="openPathPicker(item, index)">
              修改
            </button>
          </div>
          <div class="inline-flex overflow-hidden rounded-md border border-blue-500">
            <button
              v-for="option in [{ value: 'write', label: '读写' }, { value: 'read', label: '只读' }, { value: 'none', label: '无权限' }]"
              :key="option.value"
              class="px-3 py-1.5 text-sm"
              :class="item.permission === option.value ? 'bg-blue-500 text-white' : 'bg-white text-slate-600 hover:bg-blue-50'"
              @click="item.permission = option.value as Permission"
            >
              {{ option.label }}
            </button>
          </div>
        </div>
      </div>

      <button
        class="w-full rounded-md border border-dashed border-blue-300 bg-blue-50/60 px-4 py-2 text-sm text-blue-600 hover:bg-blue-50"
        :disabled="!repositories.length"
        @click="addPermission"
      >
        + 添加路径
      </button>
    </div>
    <template #footer>
      <div class="flex justify-end gap-2">
        <button class="btn btn-secondary" @click="closePermissions">取消</button>
        <button class="btn btn-primary" :disabled="busy" @click="savePermissions">{{ busy ? "保存中…" : "保存" }}</button>
      </div>
    </template>
  </BaseDialog>

  <PathPickerDialog
    :open="pathPicker.open"
    :repo="pathPicker.repo"
    :initial-path="pathPicker.initialPath"
    @close="pathPicker.open = false"
    @select="selectPath"
  />
</template>

<style scoped>
.permission-repository-select :deep(.vxe-input) {
  height: 42px;
  border: 1px solid #cbd5e1;
  border-radius: 0.375rem;
  background: #fff;
  box-shadow: none;
  transition: border-color 120ms, box-shadow 120ms;
}

.permission-repository-select :deep(.vxe-input--inner) {
  height: 40px;
  padding: 0 0.75rem;
  color: #172033;
  font-size: 0.875rem;
}

.permission-repository-select :deep(.vxe-input--suffix) {
  height: 40px;
  background: transparent;
}

.permission-repository-select :deep(.vxe-input:focus-within) {
  border-color: #2f6fed;
  box-shadow: 0 0 0 3px rgb(47 111 237 / 12%);
}
</style>
