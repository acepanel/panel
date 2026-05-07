<script lang="ts" setup>
interface Props {
  title?: string
  description?: string
  collapsible?: boolean
  defaultCollapsed?: boolean
  cols?: 1 | 2 | 3
  gap?: number
}

const props = withDefaults(defineProps<Props>(), {
  title: undefined,
  description: undefined,
  collapsible: false,
  defaultCollapsed: false,
  cols: 1,
  gap: 16,
})

const collapsed = ref(props.defaultCollapsed)
const toggle = () => {
  if (props.collapsible) collapsed.value = !collapsed.value
}

const colsClass = computed(() => {
  if (props.cols === 3) return 'grid grid-cols-1 md:grid-cols-3'
  if (props.cols === 2) return 'grid grid-cols-1 md:grid-cols-2'
  return 'flex flex-col'
})
</script>

<template>
  <section class="mb-6 flex flex-col last:mb-0">
    <header
      v-if="title || $slots.title || $slots.extra"
      class="mb-3 flex items-center justify-between"
      :class="{ 'cursor-pointer': collapsible }"
      @click="toggle"
    >
      <div class="flex gap-2 items-center">
        <i-mdi-chevron-down
          v-if="collapsible"
          :class="{ '-rotate-90': collapsed }"
          class="transition-transform"
        />
        <slot name="title">
          <h3 class="text-sm text-text-primary font-semibold">{{ title }}</h3>
        </slot>
      </div>
      <slot name="extra" />
    </header>
    <p v-if="description" class="text-xs text-text-tertiary mb-3 -mt-1">{{ description }}</p>
    <div v-show="!collapsed" :class="colsClass" :style="{ gap: `${gap}px` }">
      <slot />
    </div>
  </section>
</template>
