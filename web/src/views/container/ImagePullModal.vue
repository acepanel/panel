<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

import { useImagePull } from './composables/useImagePull'
import ImagePullProgress from './ImagePullProgress.vue'

const { $gettext } = useGettext()

const show = defineModel<boolean>('show', { type: Boolean, required: true })

const props = defineProps<{
  image: string
}>()

const emit = defineEmits<{
  success: []
  background: []
  cancel: []
}>()

const background = ref(false)

const { isPulling, pullProgress, pullStatus, pullError, totalProgress, start, cancel, reset } =
  useImagePull(() => {
    show.value = false
    emit('success')
  })

const handleSubmit = () => {
  if (background.value) {
    show.value = false
    emit('background')
    return
  }

  start(props.image)
}

// 取消拉取
const cancelPull = () => {
  reset()
  show.value = false
  emit('cancel')
}

watch(show, (val) => {
  if (val) {
    reset()
    background.value = false
  } else {
    cancel()
  }
})
</script>

<template>
  <app-modal
    v-model:show="show"
    :title="$gettext('Pull Image')"
    :mask-closable="false"
    :closable="false"
    :close-on-esc="false"
  >
    <image-pull-progress
      v-if="isPulling || pullError || pullProgress.size > 0"
      :progress="pullProgress"
      :status="pullStatus"
      :error="pullError"
      :percentage="totalProgress"
      @cancel="cancelPull"
      @retry="start(props.image)"
    />

    <n-form v-else>
      <n-form-item :label="$gettext('Image Name')">
        <n-input :value="props.image" readonly />
      </n-form-item>
      <n-form-item :label="$gettext('Execution Mode')">
        <n-radio-group v-model:value="background">
          <n-radio-button :value="false">{{ $gettext('Foreground') }}</n-radio-button>
          <n-radio-button :value="true">{{ $gettext('Background') }}</n-radio-button>
        </n-radio-group>
      </n-form-item>
      <n-flex justify="end">
        <n-button @click="cancelPull">{{ $gettext('Cancel') }}</n-button>
        <n-button type="primary" @click="handleSubmit">{{ $gettext('Submit') }}</n-button>
      </n-flex>
    </n-form>
  </app-modal>
</template>
