<script lang="ts" setup>
import { useGettext } from 'vue3-gettext'

import ConfirmDialog from './ConfirmDialog.vue'
import type { BatchAction } from './types'

interface Props {
  selected: any[]
  actions: BatchAction[]
  sticky?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  sticky: false,
})

const emit = defineEmits<{
  (e: 'action', key: string): void
  (e: 'clear'): void
}>()

const { $gettext } = useGettext()

const visible = computed(() => props.selected.length > 0)
</script>

<template>
  <transition name="fade-slide">
    <div
      v-if="visible"
      class="px-4 py-2 border border-border-subtle rounded-md bg-brand-subtle flex gap-3 items-center justify-between"
      :class="sticky ? 'sticky top-0 z-sticky' : ''"
    >
      <div class="text-sm text-text-secondary flex gap-2 items-center">
        <span class="text-text-primary font-medium">{{ selected.length }}</span>
        {{ $gettext('selected') }}
      </div>
      <div class="flex gap-2 items-center">
        <template v-for="action in actions" :key="action.key">
          <ConfirmDialog
            v-if="action.confirm"
            :type="action.confirm.type"
            :title="action.confirm.title"
            :content="action.confirm.content"
            :countdown="action.confirm.countdown"
            @confirm="emit('action', action.key)"
          >
            <template #trigger>
              <n-button :type="action.type ?? 'default'" size="small">
                <template v-if="action.icon" #icon>
                  <Icon :icon="action.icon" />
                </template>
                {{ action.label }}
              </n-button>
            </template>
          </ConfirmDialog>
          <n-button
            v-else
            :type="action.type ?? 'default'"
            size="small"
            @click="emit('action', action.key)"
          >
            <template v-if="action.icon" #icon>
              <Icon :icon="action.icon" />
            </template>
            {{ action.label }}
          </n-button>
        </template>
        <n-button quaternary size="small" @click="emit('clear')">
          {{ $gettext('Clear') }}
        </n-button>
      </div>
    </div>
  </transition>
</template>

<script lang="ts">
import { Icon } from '@iconify/vue'
export default { components: { Icon } }
</script>
