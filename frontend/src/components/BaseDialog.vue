<script setup lang="ts">
defineProps<{
  open: boolean;
  title: string;
  wide?: boolean;
}>();

const emit = defineEmits<{
  close: [];
}>();
</script>

<template>
  <Teleport to="body">
    <div v-if="open" class="modal-mask" @mousedown.self="emit('close')">
      <section class="modal-panel" :style="wide ? 'width: min(52rem, calc(100vw - 3rem))' : undefined">
        <header class="flex shrink-0 items-center justify-between border-b border-slate-200 px-6 py-4">
          <h2 class="text-base font-bold text-slate-900">{{ title }}</h2>
          <button class="rounded-md p-1 text-xl text-slate-400 hover:bg-slate-100 hover:text-slate-700" @click="emit('close')">×</button>
        </header>
        <div class="modal-body">
          <slot />
        </div>
        <footer v-if="$slots.footer" class="modal-footer">
          <slot name="footer" />
        </footer>
      </section>
    </div>
  </Teleport>
</template>
