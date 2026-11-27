<script setup lang="ts">
import { onMounted, reactive, ref } from "vue";
import TablePager from "@/components/TablePager.vue";
import { useTableMaxHeight } from "@/composables/useTableMaxHeight";
import { errorMessage, listAudits, type AuditEvent } from "@/api/client";
import { failure } from "@/utils/notify";

const events = ref<AuditEvent[]>([]);
const total = ref(0);
const loading = ref(false);
const selected = ref<AuditEvent>();
const detailOpen = ref(false);
const { tableMaxHeight } = useTableMaxHeight(250);
const filters = reactive({
  from: "",
  to: "",
  action: "",
  outcome: "",
  keyword: "",
  page: 1,
  pageSize: 20,
});

const actionLabels: Record<string, string> = {
  "auth.login": "管理员登录",
  "auth.logout": "退出登录",
  "user.create": "新增用户",
  "user.disable": "禁用用户",
  "user.enable": "启用用户",
  "user.delete": "删除用户",
  "user.password.reset": "重置密码",
  "user.password.reveal": "查看密码",
  "user.batch.enable": "批量启用用户",
  "user.batch.disable": "批量停用用户",
  "user.batch.reset_password": "批量重置密码",
  "user.batch.set_department": "批量设置部门",
  "department.create": "添加部门",
  "department.rename": "重命名部门",
  "department.delete": "删除部门",
  "file.passwd.update": "编辑 passwd 文件",
  "file.authz.update": "编辑 authz 文件",
  "file.snapshot.restore": "恢复配置快照",
  "group.create": "新增分组",
  "group.members.update": "更新分组成员",
  "group.delete": "删除分组",
  "acl.update": "更新权限",
  "acl.delete": "移除权限",
};

async function load() {
  loading.value = true;
  try {
    const result = await listAudits({
      from: filters.from || undefined,
      to: filters.to || undefined,
      action: filters.action || undefined,
      outcome: filters.outcome || undefined,
      keyword: filters.keyword || undefined,
      page: filters.page,
      pageSize: filters.pageSize,
    });
    events.value = result.items;
    total.value = result.total;
  } catch (error) {
    failure(errorMessage(error));
  } finally {
    loading.value = false;
  }
}

function search() {
  filters.page = 1;
  void load();
}

function resetFilters() {
  filters.from = "";
  filters.to = "";
  filters.action = "";
  filters.outcome = "";
  filters.keyword = "";
  void search();
}

function openDetail(event: AuditEvent) {
  selected.value = event;
  detailOpen.value = true;
}

function setPage(page: number) {
  filters.page = page;
  void load();
}

function setPageSize(size: number) {
  filters.pageSize = size;
  filters.page = 1;
  void load();
}

function formatTime(value: string) {
  return new Date(value).toLocaleString("zh-CN", { hour12: false });
}

onMounted(load);
</script>

<template>
  <section class="card overflow-hidden">
    <header class="border-b border-slate-100 p-5">
      <div class="flex flex-wrap items-center gap-2">
        <vxe-date-picker
          v-model="filters.from"
          type="date"
          value-format="yyyy-MM-dd"
          placeholder="开始日期"
          :max-date="filters.to || undefined"
          :transfer="true"
          style="width: 10rem"
        />
        <vxe-date-picker
          v-model="filters.to"
          type="date"
          value-format="yyyy-MM-dd"
          placeholder="结束日期"
          :min-date="filters.from || undefined"
          :transfer="true"
          style="width: 10rem"
        />
        <vxe-select v-model="filters.action" placeholder="全部操作" filterable style="width: 250px">
          <vxe-option value="" label="全部操作" />
          <vxe-option v-for="(label, value) in actionLabels" :key="value" :value="value" :label="label" />
        </vxe-select>
        <vxe-select v-model="filters.outcome" placeholder="全部结果" style="width: 8rem">
          <vxe-option value="" label="全部结果" />
          <vxe-option value="success" label="成功" />
          <vxe-option value="failure" label="失败" />
        </vxe-select>
        <vxe-input
          v-model="filters.keyword"
          placeholder="搜索用户、仓库、路径或错误信息"
          clearable
          class="min-w-[16rem] flex-1"
          @keyup.enter="search"
        />
        <div class="ml-auto flex shrink-0 gap-2">
          <vxe-button status="primary" content="查询" @click="search" />
          <vxe-button content="重置" @click="resetFilters" />
        </div>
      </div>
    </header>

    <vxe-table
      :data="events"
      :loading="loading"
      border
      stripe
      show-overflow
      :max-height="tableMaxHeight"
      auto-resize
      :virtual-x-config="{ enabled: true, gt: 8 }"
      :virtual-y-config="{ enabled: true, gt: 20, oSize: 8 }"
      :row-config="{ isHover: true }"
      :cell-config="{ height: 44 }"
      :column-config="{ resizable: true }"
      @cell-click="({ row }) => openDetail(row)"
    >
      <vxe-column field="timestamp" title="时间" width="160">
        <template #default="{ row }">{{ formatTime(row.timestamp) }}</template>
      </vxe-column>
      <vxe-column field="action" title="操作" width="132">
        <template #default="{ row }">{{ actionLabels[row.action] ?? row.action }}</template>
      </vxe-column>
      <vxe-column field="outcome" title="结果" width="76">
        <template #default="{ row }">
          <span class="badge" :class="row.outcome === 'success' ? 'badge-green' : 'badge-amber'">
            {{ row.outcome === "success" ? "成功" : "失败" }}
          </span>
        </template>
      </vxe-column>
      <vxe-column title="对象" min-width="260" show-overflow>
        <template #default="{ row }">
          <div class="font-medium text-slate-700">{{ row.target || "—" }}</div>
          <div v-if="row.repo || row.path" class="text-[11px] text-slate-400">{{ row.repo || "" }}{{ row.path || "" }}</div>
        </template>
      </vxe-column>
      <vxe-column field="clientIp" title="来源 IP" width="112" />
    </vxe-table>

    <TablePager
      :page="filters.page"
      :page-size="filters.pageSize"
      :total="total"
      :sizes="[20, 50, 100, 200]"
      :allow-all="false"
      @update:page="setPage"
      @update:page-size="setPageSize"
    />
  </section>

  <vxe-modal
    v-model="detailOpen"
    title="审计事件详情"
    :position="{ top: 100 }"
    width="min(56rem, calc(100vw - 3rem))"
    height="min(42rem, calc(100vh - 8rem))"
    resize
    show-footer
  >
    <pre v-if="selected" class="h-full overflow-auto whitespace-pre-wrap rounded-lg bg-slate-950 p-5 font-mono text-xs leading-6 text-slate-200">{{ JSON.stringify(selected, null, 2) }}</pre>
    <template #footer>
      <vxe-button @click="detailOpen = false">关闭</vxe-button>
    </template>
  </vxe-modal>
</template>
