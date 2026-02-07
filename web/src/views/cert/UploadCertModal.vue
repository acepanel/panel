<script setup lang="ts">
import cert from '@/api/panel/cert'
import { NButton, NSpace } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

const { $gettext } = useGettext()
const show = defineModel<boolean>('show', { type: Boolean, required: true })

const model = ref<any>({
  cert: '',
  key: ''
})

const loading = ref(false)

const handleSubmit = () => {
  loading.value = true
  useRequest(cert.certUpload(model.value))
    .onSuccess(() => {
      window.$bus.emit('cert:refresh-cert')
      window.$bus.emit('cert:refresh-async')
      show.value = false
      model.value.cert = ''
      model.value.key = ''
      window.$message.success($gettext('Created successfully'))
    })
    .onComplete(() => {
      loading.value = false
    })
}
</script>

<template>
  <n-modal
    v-model:show="show"
    preset="card"
    :title="$gettext('Upload Certificate')"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
  >
    <n-space vertical>
      <n-form :model="model">
        <n-form-item :label="$gettext('Certificate')">
          <n-input
            v-model:value="model.cert"
            type="textarea"
            :placeholder="$gettext('Enter the content of the PEM certificate file')"
            :autosize="{ minRows: 10, maxRows: 15 }"
          />
        </n-form-item>
        <n-form-item :label="$gettext('Private Key')">
          <n-input
            v-model:value="model.key"
            type="textarea"
            :placeholder="$gettext('Enter the content of the KEY private key file')"
            :autosize="{ minRows: 10, maxRows: 15 }"
          />
        </n-form-item>
      </n-form>
      <n-button type="info" block :loading="loading" :disabled="loading" @click="handleSubmit">{{ $gettext('Submit') }}</n-button>
    </n-space>
  </n-modal>
</template>

<style scoped lang="scss"></style>
