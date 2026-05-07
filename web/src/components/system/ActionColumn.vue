<script lang="ts" setup generic="T">
import ConfirmDialog from './ConfirmDialog.vue'
import MoreActions from './MoreActions.vue'
import type { ActionItem } from './types'

interface Props {
  row: T
  actions: ActionItem<T>[]
  maxVisible?: number
  size?: 'tiny' | 'small' | 'medium'
  variant?: 'button' | 'text'
}

const props = withDefaults(defineProps<Props>(), {
  maxVisible: 3,
  size: 'small',
  variant: 'text',
})

const visibleActions = computed(() => {
  const filtered = props.actions.filter((a) => !a.show || a.show(props.row))
  const max = props.maxVisible
  if (filtered.length <= max) return filtered
  return filtered.slice(0, max)
})

const overflowActions = computed(() => {
  const filtered = props.actions.filter((a) => !a.show || a.show(props.row))
  if (filtered.length <= props.maxVisible) return []
  return filtered.slice(props.maxVisible)
})

const isDisabled = (action: ActionItem<T>) => {
  if (typeof action.disabled === 'function') return action.disabled(props.row)
  return !!action.disabled
}

const resolveContent = (action: ActionItem<T>) => {
  if (!action.confirm) return ''
  return typeof action.confirm.content === 'function'
    ? action.confirm.content(props.row)
    : action.confirm.content
}

const onSelectMore = (key: string) => {
  const action = props.actions.find((a) => a.key === key)
  if (!action) return
  if (action.confirm) {
    // 通过命令式确认替代下拉里的内联弹窗
    const { useConfirm } = (window as any).__useConfirm || {}
    void invokeWithConfirm(action)
  } else {
    void action.onClick(props.row)
  }
}

const invokeWithConfirm = async (action: ActionItem<T>) => {
  if (!action.confirm) {
    await action.onClick(props.row)
    return
  }
  const content = resolveContent(action)
  const confirmType = action.confirm.type ?? 'normal'
  const confirmed = await new Promise<boolean>((resolve) => {
    if (confirmType === 'delete' || confirmType === 'danger') {
      const total = action.confirm?.countdown ?? (confirmType === 'delete' ? 5 : 0)
      let remain = total
      let timer: ReturnType<typeof setInterval> | null = null
      const dialog = window.$dialog?.warning({
        title: action.confirm?.title ?? '',
        content,
        positiveText: total > 0 ? `(${remain}s)` : '',
        negativeText: '',
        positiveButtonProps: { type: 'error', disabled: total > 0 },
        autoFocus: false,
        maskClosable: false,
        onPositiveClick: () => {
          if (remain > 0) return false
          clean()
          resolve(true)
        },
        onNegativeClick: () => {
          clean()
          resolve(false)
        },
        onClose: () => {
          clean()
          resolve(false)
        },
      })
      const clean = () => {
        if (timer) {
          clearInterval(timer)
          timer = null
        }
      }
      if (total > 0 && dialog) {
        timer = setInterval(() => {
          remain -= 1
          dialog.positiveText = remain > 0 ? `(${remain}s)` : ''
          dialog.positiveButtonProps = { type: 'error', disabled: remain > 0 }
          if (remain <= 0) clean()
        }, 1000)
      }
    } else {
      window.$dialog?.warning({
        title: action.confirm?.title ?? '',
        content,
        positiveText: 'OK',
        negativeText: 'Cancel',
        autoFocus: false,
        onPositiveClick: () => resolve(true),
        onNegativeClick: () => resolve(false),
        onClose: () => resolve(false),
      })
    }
  })
  if (confirmed) await action.onClick(props.row)
}
</script>

<template>
  <div class="flex flex-wrap gap-2 items-center">
    <template v-for="action in visibleActions" :key="action.key">
      <ConfirmDialog
        v-if="action.confirm"
        :type="action.confirm.type"
        :title="action.confirm.title"
        :content="resolveContent(action)"
        :countdown="action.confirm.countdown"
        :placement="action.confirm.placement ?? 'top'"
        @confirm="action.onClick(row)"
      >
        <template #trigger>
          <n-button
            :type="action.type ?? 'default'"
            :size="size"
            :text="variant === 'text'"
            :disabled="isDisabled(action)"
          >
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
        :size="size"
        :text="variant === 'text'"
        :disabled="isDisabled(action)"
        @click="action.onClick(row)"
      >
        <template v-if="action.icon" #icon>
          <Icon :icon="action.icon" />
        </template>
        {{ action.label }}
      </n-button>
    </template>
    <MoreActions
      v-if="overflowActions.length"
      :options="
        overflowActions.map((a) => ({
          key: a.key,
          label: a.label,
          type: a.type,
          disabled: isDisabled(a),
        }))
      "
      :size="size"
      @select="onSelectMore"
    />
  </div>
</template>

<script lang="ts">
import { Icon } from '@iconify/vue'
export default { components: { Icon } }
</script>
