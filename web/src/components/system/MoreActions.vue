<script lang="ts" setup>
import type { DropdownOption, PopoverPlacement } from 'naive-ui'

import type { MoreActionOption } from './types'

interface Props {
  options: MoreActionOption[]
  trigger?: 'click' | 'hover'
  placement?: PopoverPlacement
  size?: 'tiny' | 'small' | 'medium'
}

const props = withDefaults(defineProps<Props>(), {
  trigger: 'click',
  placement: 'bottom-end',
  size: 'small',
})

const emit = defineEmits<{ (e: 'select', key: string): void }>()

const dropdownOptions = computed<DropdownOption[]>(() =>
  props.options.map((opt) =>
    opt.divider
      ? { type: 'divider', key: opt.key }
      : {
          key: opt.key,
          label: opt.label,
          disabled: opt.disabled,
          props: opt.type === 'error' ? { style: 'color: var(--color-error-fg)' } : undefined,
        },
  ),
)

const onSelect = (key: string) => emit('select', key)
</script>

<template>
  <n-dropdown
    :options="dropdownOptions"
    :trigger="trigger"
    :placement="placement"
    @select="onSelect"
  >
    <n-button quaternary circle :size="size">
      <template #icon>
        <i-mdi-dots-horizontal />
      </template>
    </n-button>
  </n-dropdown>
</template>
