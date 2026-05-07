<script lang="ts" setup>
interface Props {
  title: string
  description?: string
  back?: boolean | (() => void)
  size?: 'default' | 'large'
}

const props = withDefaults(defineProps<Props>(), {
  description: undefined,
  back: false,
  size: 'default',
})

const router = useRouter()

const handleBack = () => {
  if (typeof props.back === 'function') props.back()
  else router.back()
}

const titleClass = computed(() =>
  props.size === 'large' ? 'text-2xl font-semibold leading-tight' : 'page-title',
)
</script>

<template>
  <div class="page-header">
    <div class="flex flex-1 gap-3 min-w-0 items-start">
      <n-button v-if="back" quaternary circle @click="handleBack">
        <template #icon>
          <i-mdi-arrow-left />
        </template>
      </n-button>
      <div class="flex-1 min-w-0">
        <h1 :class="titleClass">{{ title }}</h1>
        <p v-if="description" class="page-subtitle">{{ description }}</p>
        <div v-if="$slots.extra" class="mt-2">
          <slot name="extra" />
        </div>
      </div>
    </div>
    <div v-if="$slots.actions" class="flex flex-wrap gap-2 items-center">
      <slot name="actions" />
    </div>
  </div>
</template>
