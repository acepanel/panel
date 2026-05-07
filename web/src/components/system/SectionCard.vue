<script lang="ts" setup>
interface Props {
  title?: string
  description?: string
  collapsible?: boolean
  defaultCollapsed?: boolean
  bordered?: boolean
  padding?: boolean
  size?: 'small' | 'medium' | 'large'
}

const props = withDefaults(defineProps<Props>(), {
  title: undefined,
  description: undefined,
  collapsible: false,
  defaultCollapsed: false,
  bordered: false,
  padding: true,
  size: 'medium',
})

const collapsed = ref(props.defaultCollapsed)
const toggle = () => {
  if (props.collapsible) collapsed.value = !collapsed.value
}
</script>

<template>
  <n-card :bordered="bordered" :size="size" :content-class="padding ? undefined : '!p-0'">
    <template v-if="title || $slots.title || $slots.extra" #header>
      <div
        class="flex gap-2 cursor-default items-center"
        :class="{ 'cursor-pointer': collapsible }"
        @click="toggle"
      >
        <i-mdi-chevron-down
          v-if="collapsible"
          :class="{ '-rotate-90': collapsed }"
          class="transition-transform"
        />
        <slot name="title">
          <span class="section-title">{{ title }}</span>
        </slot>
      </div>
      <p v-if="description" class="text-sm text-text-tertiary mt-1">{{ description }}</p>
    </template>
    <template v-if="$slots.extra" #header-extra>
      <slot name="extra" />
    </template>
    <div v-show="!collapsed">
      <slot />
    </div>
    <template v-if="$slots.footer" #footer>
      <slot name="footer" />
    </template>
  </n-card>
</template>
