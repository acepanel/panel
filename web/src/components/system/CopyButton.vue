<script lang="ts" setup>
import copy2clipboard from '@vavt/copy2clipboard'
import { useGettext } from 'vue3-gettext'

interface Props {
  text: string | (() => string)
  successMessage?: string
  size?: 'tiny' | 'small' | 'medium'
  variant?: 'icon' | 'button' | 'text'
  tooltip?: string
}

const props = withDefaults(defineProps<Props>(), {
  successMessage: undefined,
  size: 'small',
  variant: 'icon',
  tooltip: undefined,
})

const { $gettext } = useGettext()

const handleCopy = async () => {
  const value = typeof props.text === 'function' ? props.text() : props.text
  if (!value) return
  await copy2clipboard(value)
  window.$message?.success(props.successMessage ?? $gettext('Copied'))
}
</script>

<template>
  <n-tooltip v-if="variant === 'icon'" trigger="hover">
    <template #trigger>
      <n-button quaternary circle :size="size" @click="handleCopy">
        <template #icon>
          <i-mdi-content-copy />
        </template>
      </n-button>
    </template>
    {{ tooltip ?? $gettext('Copy') }}
  </n-tooltip>
  <n-button v-else-if="variant === 'text'" text :size="size" @click="handleCopy">
    <template #icon>
      <i-mdi-content-copy />
    </template>
    <slot>{{ $gettext('Copy') }}</slot>
  </n-button>
  <n-button v-else :size="size" @click="handleCopy">
    <template #icon>
      <i-mdi-content-copy />
    </template>
    <slot>{{ $gettext('Copy') }}</slot>
  </n-button>
</template>
