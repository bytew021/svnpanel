<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue";
import type { VxeTableInstance } from "vxe-table";
import BaseDialog from "@/components/BaseDialog.vue";
import TablePager from "@/components/TablePager.vue";
import { useTableMaxHeight } from "@/composables/useTableMaxHeight";
import {
  addDepartment,
  batchUsers,
  createUser,
  deleteDepartment,
  deleteUser,
  errorMessage,
  listUsers,
  renameDepartment,
  resetPassword,
  revealPassword,
  setUserDisabled,
  setUserMetadata,
} from "@/api/client";
import type { Revisions, User } from "@/types";
import { confirmAction, failure, success } from "@/utils/notify";

const users = ref<User[]>([]);
const revisions = ref<Revisions>({ passwd: "", authz: "" });
const loading = ref(false);
const keyword = ref("");
const status = ref("all");
const departmentFilter = ref("");
const departments = ref<string[]>([]);
const page = ref(1);
const pageSize = ref(20);
const selectedNames = ref<string[]>([]);
const tableRef = ref<VxeTableInstance<User>>();
const batchMenuRef = ref<HTMLElement>();
const statusMenuOpen = ref(false);
const revealedPasswords = reactive<Record<string, string>>({});
const passwordLoading = ref("");
const dialog = reactive({
  open: false,
  mode: "create" as "create" | "metadata" | "reset" | "batch-reset" | "batch-department",
  user: "",
  password: "",
  remark: "",
  department: "",
});
const busy = ref(false);
const departmentManagerOpen = ref(false);
const departmentBusy = ref(false);
const newDepartment = ref("");
const departmentDrafts = reactive<Record<string, string>>({});
const { tableMaxHeight } = useTableMaxHeight(250);

const filtered = computed(() =>
  users.value.filter((user) => {
    const keywordMatch = user.name.toLowerCase().includes(keyword.value.trim().toLowerCase());
    const statusMatch = status.value === "all" || (status.value === "disabled" ? user.disabled : !user.disabled);
    const departmentMatch = !departmentFilter.value || user.department === departmentFilter.value;
    const remarkMatch = (user.remark ?? "").toLowerCase().includes(keyword.value.trim().toLowerCase());
    return (keywordMatch || remarkMatch) && statusMatch && departmentMatch;
  }),
);

const pageItems = computed(() =>
  pageSize.value === 0
    ? filtered.value
    : filtered.value.slice((page.value - 1) * pageSize.value, page.value * pageSize.value),
);
const allSelected = computed(
  () => pageItems.value.length > 0 && selectedNames.value.length === pageItems.value.length,
);

watch([keyword, status, departmentFilter], () => {
  page.value = 1;
  selectedNames.value = [];
});

watch(
  () => filtered.value.length,
  (total) => {
    const maxPage = pageSize.value === 0 ? 1 : Math.max(1, Math.ceil(total / pageSize.value));
    if (page.value > maxPage) page.value = maxPage;
  },
);

watch([page, pageSize], () => {
  clearSelection();
});

async function load() {
  loading.value = true;
  try {
    const result = await listUsers();
    users.value = result.items;
    departments.value = result.departments ?? [];
    revisions.value = result.revisions;
    if (departmentFilter.value && !departments.value.includes(departmentFilter.value)) {
      departmentFilter.value = "";
    }
    syncDepartmentDrafts();
    clearSelection();
    for (const name of Object.keys(revealedPasswords)) delete revealedPasswords[name];
  } catch (error) {
    failure(errorMessage(error));
  } finally {
    loading.value = false;
  }
}

function openCreate() {
  Object.assign(dialog, {
    open: true, mode: "create", user: "", password: "", remark: "", department: "",
  });
}

function resetFilters() {
  keyword.value = "";
  status.value = "all";
  departmentFilter.value = "";
}

function openReset(user: string) {
  Object.assign(dialog, { open: true, mode: "reset", user, password: "", remark: "", department: "" });
}

function openMetadata(user: User) {
  Object.assign(dialog, {
    open: true, mode: "metadata", user: user.name, password: "", remark: user.remark ?? "",
    department: user.department ?? "",
  });
}

function openBatchReset() {
  statusMenuOpen.value = false;
  Object.assign(dialog, {
    open: true, mode: "batch-reset", user: "", password: "", remark: "", department: "",
  });
}

function openBatchDepartment() {
  statusMenuOpen.value = false;
  Object.assign(dialog, {
    open: true, mode: "batch-department", user: "", password: "", remark: "", department: "",
  });
}

function closeDialog() {
  dialog.open = false;
  dialog.password = "";
}

function syncDepartmentDrafts() {
  for (const department of Object.keys(departmentDrafts)) delete departmentDrafts[department];
  for (const department of departments.value) departmentDrafts[department] = department;
}

function openDepartmentManager() {
  syncDepartmentDrafts();
  newDepartment.value = "";
  departmentManagerOpen.value = true;
}

function closeDepartmentManager() {
  departmentManagerOpen.value = false;
  newDepartment.value = "";
}

async function submitDialog() {
  busy.value = true;
  try {
    if (dialog.mode === "create") {
      const result = await createUser(
        revisions.value, dialog.user.trim(), dialog.password, dialog.remark, dialog.department,
      );
      revisions.value = result.revisions;
      success("用户已创建");
      closeDialog();
      await load();
      return;
    }
    if (dialog.mode === "metadata") {
      const result = await setUserMetadata(revisions.value, dialog.user, dialog.remark, dialog.department);
      revisions.value = result.revisions;
      success("用户备注已更新");
      closeDialog();
      await load();
      return;
    }
    if (dialog.mode === "reset") {
      const result = await resetPassword(revisions.value, dialog.user, dialog.password);
      revisions.value = result.revisions;
      success("密码已重置");
      closeDialog();
      await load();
      return;
    }
    if (dialog.mode === "batch-reset") {
      const result = await batchUsers(revisions.value, selectedNames.value, "reset_password", {
        password: dialog.password,
      });
      revisions.value = result.revisions;
      success("批量密码已重置");
      closeDialog();
      await load();
      return;
    }
    if (dialog.mode === "batch-department") {
      const result = await batchUsers(revisions.value, selectedNames.value, "set_department", {
        department: dialog.department,
      });
      revisions.value = result.revisions;
      success("批量部门已更新");
      closeDialog();
      await load();
    }
  } catch (error) {
    failure(errorMessage(error));
  } finally {
    busy.value = false;
  }
}

async function togglePassword(user: User) {
  if (revealedPasswords[user.name]) {
    delete revealedPasswords[user.name];
    return;
  }
  passwordLoading.value = user.name;
  try {
    const result = await revealPassword(user.name);
    revealedPasswords[user.name] = result.value;
  } catch (error) {
    failure(errorMessage(error));
  } finally {
    passwordLoading.value = "";
  }
}

async function addDepartmentItem() {
  const name = newDepartment.value.trim();
  if (!name) {
    failure("请输入部门名称");
    return;
  }
  departmentBusy.value = true;
  try {
    const result = await addDepartment(revisions.value, name);
    revisions.value = result.revisions;
    newDepartment.value = "";
    success("部门已添加");
    await load();
  } catch (error) {
    failure(errorMessage(error));
  } finally {
    departmentBusy.value = false;
  }
}

async function renameDepartmentItem(oldName: string) {
  const name = (departmentDrafts[oldName] ?? "").trim();
  if (!name || name === oldName) return;
  if (!(await confirmAction(
    `重命名部门“${oldName}”？`,
    `已设置为“${oldName}”的用户会同步改为“${name}”。`,
  ))) return;
  departmentBusy.value = true;
  try {
    const result = await renameDepartment(revisions.value, oldName, name);
    revisions.value = result.revisions;
    success("部门已重命名");
    await load();
  } catch (error) {
    failure(errorMessage(error));
    await load();
  } finally {
    departmentBusy.value = false;
  }
}

async function removeDepartmentItem(name: string) {
  if (!(await confirmAction(
    `删除部门“${name}”？`,
    "只有未被任何用户使用的部门才能删除。",
    true,
  ))) return;
  departmentBusy.value = true;
  try {
    const result = await deleteDepartment(revisions.value, name);
    revisions.value = result.revisions;
    success("部门已删除");
    await load();
  } catch (error) {
    failure(errorMessage(error));
    await load();
  } finally {
    departmentBusy.value = false;
  }
}

function updateSelection({ records }: { records: User[] }) {
  selectedNames.value = records.map((row) => row.name);
}

function clearSelection() {
  selectedNames.value = [];
  statusMenuOpen.value = false;
  void tableRef.value?.clearCheckboxRow();
}

function toggleSelectAll({ checked }: { checked: boolean }) {
  selectedNames.value = checked ? pageItems.value.map((row) => row.name) : [];
  void (checked ? tableRef.value?.setAllCheckboxRow(true) : tableRef.value?.clearCheckboxRow());
}

async function batchStatus(disabled: boolean) {
  statusMenuOpen.value = false;
  const action = disabled ? "停用" : "启用";
  if (!(await confirmAction(`批量${action} ${selectedNames.value.length} 个用户？`, "将批量更新选中用户的登录状态。", disabled))) {
    return;
  }
  try {
    const result = await batchUsers(revisions.value, selectedNames.value, disabled ? "disable" : "enable");
    revisions.value = result.revisions;
    success(`已批量${action}`);
    await load();
  } catch (error) {
    failure(errorMessage(error));
    await load();
  }
}

async function toggleUser(user: User) {
  const action = user.disabled ? "启用" : "禁用";
  if (!(await confirmAction(`${action}用户 ${user.name}？`, user.disabled ? "启用后可恢复使用当前密码登录。" : "禁用后该用户无法通过 SVN 登录，权限引用仍会保留。"))) {
    return;
  }
  try {
    const result = await setUserDisabled(revisions.value, user.name, !user.disabled);
    revisions.value = result.revisions;
    success(`用户已${action}`);
    await load();
  } catch (error) {
    failure(errorMessage(error));
    await load();
  }
}

async function removeUser(user: User) {
  if (!(await confirmAction(`删除用户 ${user.name}？`, "只有未被任何分组或 ACL 引用的用户才能删除。", true))) return;
  try {
    const result = await deleteUser(revisions.value, user.name);
    revisions.value = result.revisions;
    success("用户已删除");
    await load();
  } catch (error) {
    failure(errorMessage(error));
    await load();
  }
}

function closeBatchMenu(event: MouseEvent) {
  if (statusMenuOpen.value && batchMenuRef.value && !batchMenuRef.value.contains(event.target as Node)) {
    statusMenuOpen.value = false;
  }
}

onMounted(() => {
  void load();
  document.addEventListener("click", closeBatchMenu);
});
onBeforeUnmount(() => {
  document.removeEventListener("click", closeBatchMenu);
});
</script>

<template>
  <div class="space-y-5">
    <section class="card overflow-hidden">
      <header class="flex flex-wrap items-center justify-between gap-3 px-4 pb-4 pt-4">
        <div class="flex flex-wrap items-center gap-3">
          <vxe-input v-model="keyword" placeholder="搜索用户名或备注..." clearable style="width: 220px" />
          <vxe-select v-model="status" style="width: 130px">
            <vxe-option value="all" label="全部状态" />
            <vxe-option value="active" label="正常" />
            <vxe-option value="disabled" label="已禁用" />
          </vxe-select>
          <vxe-select v-model="departmentFilter" style="width: 130px">
            <vxe-option value="" label="全部部门" />
            <vxe-option v-for="department in departments" :key="department" :value="department" :label="department" />
          </vxe-select>
          <vxe-button @click="resetFilters">重置</vxe-button>
        </div>
        <div class="flex items-center gap-2">
          <vxe-button @click="openDepartmentManager">管理部门</vxe-button>
          <vxe-button type="success" icon="vxe-icon-plus" @click="openCreate">新增 SVN 用户</vxe-button>
        </div>
      </header>

      <vxe-table
        ref="tableRef"
        :data="pageItems"
        :loading="loading"
        border
        stripe
        show-overflow
        :max-height="tableMaxHeight"
        auto-resize
        :virtual-x-config="{ enabled: true, gt: 8 }"
        :virtual-y-config="{ enabled: true, gt: 20, oSize: 8 }"
        :row-config="{ isHover: true, height: 44, keyField: 'name' }"
        :column-config="{ resizable: true }"
        :checkbox-config="{ highlight: false }"
        @checkbox-change="updateSelection"
        @checkbox-all="updateSelection"
      >
        <vxe-column type="checkbox" width="44" align="center" fixed="left" :resizable="false" />
        <vxe-column type="seq" title="#" width="48" align="center" />
        <vxe-column field="name" title="用户名" width="200">
          <template #default="{ row }">
            <div class="truncate font-mono font-semibold text-slate-800">{{ row.name }}</div>
          </template>
        </vxe-column>
        <vxe-column field="remark" title="备注" show-overflow>
          <template #default="{ row }"><span :class="row.remark ? 'text-slate-700' : 'text-slate-400'">{{ row.remark || "—" }}</span></template>
        </vxe-column>
        <vxe-column field="department" title="部门" width="100">
          <template #default="{ row }">
            <span v-if="row.department" class="badge badge-amber">{{ row.department }}</span>
            <span v-else class="text-slate-400">—</span>
          </template>
        </vxe-column>
        <vxe-column title="状态" width="90" align="center">
          <template #default="{ row }">
            <span class="badge" :class="row.disabled ? 'badge-gray' : 'badge-green'">{{ row.disabled ? "已禁用" : "正常" }}</span>
          </template>
        </vxe-column>
        <vxe-column title="密码" width="230">
          <template #default="{ row }">
            <div class="flex items-center gap-2">
              <code v-if="revealedPasswords[row.name]" class="break-all text-xs text-slate-800">{{ revealedPasswords[row.name] }}</code>
              <span v-else class="font-mono tracking-widest text-slate-500">••••••••</span>
              <span class="text-[10px] uppercase text-slate-400">{{ row.encoding }}</span>
              <button
                class="rounded p-1 text-slate-500 hover:bg-slate-100 hover:text-blue-700"
                :title="revealedPasswords[row.name] ? '隐藏密码' : '查看密码'"
                :disabled="passwordLoading === row.name"
                @click.stop="togglePassword(row)"
              >
                <svg v-if="revealedPasswords[row.name]" class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <path d="M3 3l18 18M10.6 10.6a2 2 0 002.8 2.8M9.9 4.2A10.7 10.7 0 0112 4c5.5 0 9 6 9 6a16 16 0 01-3.2 4.1M6.6 6.6A16 16 0 003 10s3.5 6 9 6c1.4 0 2.7-.4 3.8-1" />
                </svg>
                <svg v-else class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <path d="M2.5 12s3.5-6 9.5-6 9.5 6 9.5 6-3.5 6-9.5 6-9.5-6-9.5-6z" />
                  <circle cx="12" cy="12" r="2.5" />
                </svg>
              </button>
            </div>
          </template>
        </vxe-column>
        <vxe-column title="操作" width="230" align="center">
          <template #default="{ row }">
            <div class="flex justify-end gap-1">
              <vxe-button size="small" mode="text" status="primary" @click="openMetadata(row)">备注</vxe-button>
              <vxe-button size="small" mode="text" status="warning" @click="openReset(row.name)">重置</vxe-button>
              <vxe-button size="small" mode="text" :status="row.disabled ? 'success' : 'warning'" @click="toggleUser(row)">{{ row.disabled ? "启用" : "禁用" }}</vxe-button>
              <vxe-button size="small" mode="text" status="danger" @click="removeUser(row)">删除</vxe-button>
            </div>
          </template>
        </vxe-column>
      </vxe-table>
      <TablePager v-model:page="page" v-model:page-size="pageSize" :total="filtered.length" :sizes="[20, 50, 100]">
        <template #start>
          <div class="batch-select-shell flex h-8 w-11 items-center justify-center">
            <vxe-checkbox
              :model-value="allSelected"
              :indeterminate="selectedNames.length > 0 && !allSelected"
              title="全选当前页"
              @change="toggleSelectAll"
            />
          </div>
          <span v-if="!selectedNames.length" class="text-xs text-slate-500">共 {{ filtered.length }} 项</span>
          <div v-if="selectedNames.length" class="flex flex-wrap items-center gap-2">
            <div ref="batchMenuRef" class="relative inline-flex">
              <div class="inline-flex overflow-hidden rounded-md">
                <button class="batch-main-button" @click="statusMenuOpen = !statusMenuOpen">批量操作</button>
                <button class="batch-menu-button" aria-label="展开批量操作" @click="statusMenuOpen = !statusMenuOpen">▼</button>
              </div>
              <div v-if="statusMenuOpen" class="absolute bottom-full left-0 z-20 mb-1 min-w-36 rounded-md border border-slate-200 bg-white py-1 shadow-lg">
                <button class="batch-dropdown-item" @click="batchStatus(false)">批量启用</button>
                <button class="batch-dropdown-item" @click="batchStatus(true)">批量停用</button>
                <button class="batch-dropdown-item" @click="openBatchReset">批量重置密码</button>
                <button class="batch-dropdown-item" @click="openBatchDepartment">批量设置部门</button>
              </div>
            </div>
            <span class="text-xs text-slate-600">已选择 {{ selectedNames.length }} 项</span>
          </div>
        </template>
      </TablePager>
    </section>

    <BaseDialog
      :open="dialog.open"
      :title="
        dialog.mode === 'create'
          ? '新增 SVN 用户'
          : dialog.mode === 'metadata'
            ? `编辑备注 · ${dialog.user}`
            : dialog.mode === 'batch-reset'
              ? `批量重置密码 · ${selectedNames.length} 个用户`
              : dialog.mode === 'batch-department'
                ? `批量设置部门 · ${selectedNames.length} 个用户`
                : `重置密码 · ${dialog.user}`
      "
      @close="closeDialog"
    >
      <form class="space-y-4" @submit.prevent="submitDialog">
        <label v-if="dialog.mode === 'create'" class="block">
          <span class="mb-2 block text-sm font-semibold text-slate-700">用户名</span>
          <input v-model="dialog.user" class="input" maxlength="64" placeholder="字母、数字、. _ -" autocomplete="off" />
          <span class="mt-1 block text-xs text-slate-500">名称创建后不可修改</span>
        </label>
        <label v-if="dialog.mode === 'create' || dialog.mode === 'reset' || dialog.mode === 'batch-reset'" class="block">
          <span class="mb-2 block text-sm font-semibold text-slate-700">新密码</span>
          <input v-model="dialog.password" class="input" type="password" autocomplete="new-password" />
          <span class="mt-1 block text-xs text-slate-500">8–128 个字符，保存后会写入 svnserve passwd 文件</span>
        </label>
        <label v-if="dialog.mode === 'create' || dialog.mode === 'metadata' || dialog.mode === 'batch-department'" class="block">
          <span class="mb-2 block text-sm font-semibold text-slate-700">部门</span>
          <select v-model="dialog.department" class="input">
            <option value="">不设置</option>
            <option v-for="department in departments" :key="department" :value="department">{{ department }}</option>
          </select>
        </label>
        <label v-if="dialog.mode === 'create' || dialog.mode === 'metadata'" class="block">
          <span class="mb-2 block text-sm font-semibold text-slate-700">备注</span>
          <input v-model="dialog.remark" class="input" maxlength="500" placeholder="例如：客户端负责人" />
          <span class="mt-1 block text-xs text-slate-500">单行文本，保存为条目上方的 # svnpanel:remark 注释</span>
        </label>
        <div class="flex justify-end gap-2 pt-2">
          <button type="button" class="btn btn-secondary" @click="closeDialog">取消</button>
          <button class="btn btn-primary" :disabled="busy">{{ busy ? "保存中…" : "确认保存" }}</button>
        </div>
      </form>
    </BaseDialog>

    <BaseDialog :open="departmentManagerOpen" title="部门管理" @close="closeDepartmentManager">
      <div class="space-y-4">
        <form class="flex gap-2" @submit.prevent="addDepartmentItem">
          <input
            v-model="newDepartment"
            class="input"
            maxlength="64"
            placeholder="输入新部门名称"
            autocomplete="off"
          />
          <button class="btn btn-primary whitespace-nowrap" :disabled="departmentBusy">添加</button>
        </form>
        <div v-if="!departments.length" class="rounded-md bg-slate-50 px-4 py-6 text-center text-sm text-slate-500">
          暂无部门，请先添加
        </div>
        <div v-else class="max-h-80 space-y-2 overflow-y-auto pr-1">
          <div
            v-for="department in departments"
            :key="department"
            class="flex items-center gap-2 rounded-md border border-slate-200 p-2"
          >
            <input
              v-model="departmentDrafts[department]"
              class="input min-w-0 flex-1"
              maxlength="64"
              :aria-label="`部门 ${department}`"
            />
            <button
              class="btn btn-secondary whitespace-nowrap"
              :disabled="departmentBusy || !departmentDrafts[department]?.trim() || departmentDrafts[department]?.trim() === department"
              @click="renameDepartmentItem(department)"
            >
              保存
            </button>
            <button
              class="btn btn-danger whitespace-nowrap"
              :disabled="departmentBusy"
              @click="removeDepartmentItem(department)"
            >
              删除
            </button>
          </div>
        </div>
        <p class="text-xs leading-5 text-slate-500">
          部门定义保存在共享 <code>passwd</code> 文件的
          <code># svnpanel:departments:</code> 注释中。重命名会同步更新用户引用，被用户使用的部门不能删除。
        </p>
        <div class="flex justify-end">
          <button class="btn btn-secondary" @click="closeDepartmentManager">关闭</button>
        </div>
      </div>
    </BaseDialog>
  </div>
</template>

<style scoped>
.batch-main-button,
.batch-menu-button {
  height: 28px;
  border: 1px solid #2b9ff3;
  background: #2b9ff3;
  color: #fff;
  font-size: 12px;
  font-weight: 500;
  transition: background-color 0.15s, border-color 0.15s;
}

.batch-main-button {
  padding: 0 12px;
}

.batch-main-button:hover,
.batch-menu-button:hover {
  border-color: #168de0;
  background: #168de0;
}

.batch-menu-button {
  width: 26px;
  border-left-color: rgb(255 255 255 / 35%);
  font-size: 9px;
}

.batch-dropdown-item {
  display: block;
  width: 100%;
  padding: 7px 14px;
  color: #334155;
  font-size: 13px;
  text-align: left;
}

.batch-dropdown-item:hover {
  background: #f1f5f9;
}

.batch-select-shell {
  transform: translateX(2px);
}
</style>
