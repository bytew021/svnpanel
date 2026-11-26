import { computed, onBeforeUnmount, onMounted, ref } from "vue";

export function useTableMaxHeight(offset = 270) {
  const viewportHeight = ref(900);

  const update = () => {
    viewportHeight.value = window.innerHeight;
  };

  onMounted(() => {
    update();
    window.addEventListener("resize", update);
  });

  onBeforeUnmount(() => {
    window.removeEventListener("resize", update);
  });

  const tableMaxHeight = computed(() => Math.max(280, viewportHeight.value - offset));
  return { tableMaxHeight };
}
