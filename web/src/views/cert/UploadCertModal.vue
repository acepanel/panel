<script setup lang="ts">
import { NSpace } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import cert from '@/api/panel/cert'
import UploadFields from '@/views/cert/UploadFields.vue'

const { $gettext } = useGettext()
const show = defineModel<boolean>('show', { type: Boolean, required: true })

const defaultModel = () => ({
  cert: '',
  key: '',
  cert_path: '',
  key_path: '',
})

const model = ref<any>(defaultModel())

const handleSubmit = async () => {
  await cert.certUpload(model.value)
  window.$bus.emit('cert:refresh-cert')
  window.$bus.emit('cert:refresh-async')
  show.value = false
  model.value = defaultModel()
  window.$message.success($gettext('Created successfully'))
}
</script>

<template>
  <app-modal v-model:show="show" :title="$gettext('Upload Certificate')">
    <n-space vertical>
      <n-form :model="model">
        <upload-fields v-model:value="model" />
      </n-form>
      <app-button type="info" block @click="handleSubmit">{{ $gettext('Submit') }}</app-button>
    </n-space>
  </app-modal>
</template>

<style scoped lang="scss"></style>
