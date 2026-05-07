<script lang="ts" setup>
import { useGettext } from 'vue3-gettext'

interface Props {
  icon?: string
  title?: string
  description?: string
  size?: 'small' | 'medium' | 'large'
}

const { $gettext } = useGettext()

withDefaults(defineProps<Props>(), {
  icon: 'mdi:inbox-outline',
  title: undefined,
  description: undefined,
  size: 'medium',
})
</script>

<template>
  <div class="empty-state">
    <div
      class="rounded-full bg-bg-subtle flex items-center justify-center"
      :class="size === 'large' ? 'w-5 h-5' : size === 'small' ? 'w-10 h-10' : 'w-14 h-14'"
    >
      <Icon
        :icon="icon"
        :class="size === 'large' ? 'text-4xl' : size === 'small' ? 'text-xl' : 'text-2xl'"
      />
    </div>
    <div class="text-center">
      <p class="text-base text-text-secondary font-medium">
        {{ title ?? $gettext('No data') }}
      </p>
      <p v-if="description" class="text-sm text-text-tertiary mt-1">
        {{ description }}
      </p>
    </div>
    <slot name="action" />
  </div>
</template>

<script lang="ts">
import { Icon } from '@iconify/vue'
export default { components: { Icon } }
</script>
