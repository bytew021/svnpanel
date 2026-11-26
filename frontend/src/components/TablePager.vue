<script setup lang="ts">
import { computed } from "vue";

const props = withDefaults(
  defineProps<{
    total: number;
    page: number;
    pageSize: number;
    sizes?: number[];
    allowAll?: boolean;
  }>(),
  {
    sizes: () => [20, 50, 100],
    allowAll: true,
  },
);

const emit = defineEmits<{
  "update:page": [value: number];
  "update:pageSize": [value: number];
}>();

const sizes = computed(() => {
  const values = props.sizes.filter((size) => size > 0);
  if (props.allowAll && props.total > 0) values.push(props.total);
  return [...new Set(values)].sort((a, b) => a - b);
});
const pageCount = computed(() => {
  if (props.pageSize === 0) return 1;
  return Math.max(1, Math.ceil(props.total / props.pageSize));
});
const effectivePageSize = computed(() => props.pageSize || Math.max(1, props.total));
const showingAll = computed(() => props.total > 0 && effectivePageSize.value >= props.total);
const currentPageModel = computed({
  get: () => props.page,
  set: (value: number) => emit("update:page", Math.min(pageCount.value, Math.max(1, value))),
});
const pageSizeModel = computed({
  get: () => effectivePageSize.value,
  set: (value: number) => {
    if (!value) return;
    emit("update:pageSize", value);
    emit("update:page", 1);
  },
});

function showAll() {
  pageSizeModel.value = Math.max(1, props.total);
}
</script>

<template>
  <footer class="flex flex-wrap items-center justify-between gap-x-4 gap-y-2 border-t border-slate-100 py-3">
    <div class="flex min-h-8 flex-wrap items-center gap-2">
      <slot name="start" />
    </div>
    <div class="ml-auto flex flex-wrap items-center gap-2">
      <vxe-button
        v-if="allowAll && total > 0 && !showingAll"
        size="mini"
        mode="text"
        @click="showAll"
      >
        显示全部
      </vxe-button>
      <vxe-pager
        class="app-pager"
        size="mini"
        v-model:current-page="currentPageModel"
        v-model:page-size="pageSizeModel"
        :page-sizes="sizes"
        :total="total"
        :layouts="['PrevPage', 'JumpNumber', 'NextPage', 'FullJump', 'Sizes', 'Total']"
      />
    </div>
  </footer>
</template>

<style scoped>
.app-pager {
  max-width: 100%;
  height: auto;
  padding: 0;
  font-size: 12px;
}
.app-pager :deep(.vxe-pager--wrapper) {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: flex-end;
  row-gap: 6px;
}
</style>
