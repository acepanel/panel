<script lang="ts" setup>
import type { FormInst, FormRules } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import { formWidthMap, type FormSize } from './types'

interface Props {
  show: boolean
  title: string
  size?: FormSize
  width?: number | string
  loading?: boolean
  rules?: FormRules
  model?: Record<string, any>
  labelPlacement?: 'top' | 'left'
  labelWidth?: number | 'auto'
  submitText?: string
  cancelText?: string
  showFooter?: boolean
  closable?: boolean
  maskClosable?: boolean
  validateOnSubmit?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  size: 'md',
  width: undefined,
  loading: false,
  rules: undefined,
  model: undefined,
  labelPlacement: 'top',
  labelWidth: 'auto',
  submitText: undefined,
  cancelText: undefined,
  showFooter: true,
  closable: true,
  maskClosable: false,
  validateOnSubmit: true,
})

const emit = defineEmits<{
  (e: 'update:show', v: boolean): void
  (e: 'submit', model: any): void | Promise<void>
  (e: 'cancel'): void
}>()

const { $gettext } = useGettext()
const formRef = ref<FormInst>()

const computedWidth = computed(() => {
  if (props.width != null) {
    return typeof props.width === 'number' ? `${props.width}px` : props.width
  }
  return `${formWidthMap[props.size]}px`
})

const handleClose = () => {
  emit('update:show', false)
  emit('cancel')
}

const handleSubmit = async () => {
  if (props.validateOnSubmit && formRef.value && props.rules) {
    try {
      await formRef.value.validate()
    } catch {
      return
    }
  }
  await emit('submit', props.model)
}

defineExpose({ formRef })
</script>

<template>
  <n-modal
    :show="show"
    preset="card"
    :title="title"
    :style="{ width: computedWidth, maxWidth: '95vw' }"
    :closable="closable"
    :mask-closable="maskClosable"
    :bordered="false"
    @update:show="(v: boolean) => emit('update:show', v)"
    @close="handleClose"
  >
    <n-form
      ref="formRef"
      :model="model"
      :rules="rules"
      :label-placement="labelPlacement"
      :label-width="labelWidth"
      :disabled="loading"
      require-mark-placement="right-hanging"
    >
      <slot />
    </n-form>
    <template v-if="showFooter" #footer>
      <slot name="footer">
        <div class="flex gap-3 items-center justify-end">
          <n-button :disabled="loading" @click="handleClose">
            {{ cancelText ?? $gettext('Cancel') }}
          </n-button>
          <n-button type="primary" :loading="loading" @click="handleSubmit">
            {{ submitText ?? $gettext('Submit') }}
          </n-button>
        </div>
      </slot>
    </template>
  </n-modal>
</template>
