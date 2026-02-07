<script setup lang="ts">
import cert from '@/api/panel/cert'
import type { MessageReactive } from 'naive-ui'
import { NButton } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

const { $gettext } = useGettext()
let messageReactive: MessageReactive | null = null

const show = defineModel<boolean>('show', { type: Boolean, required: true })
const id = defineModel<number>('id', { type: Number, required: true })

const model = ref({
  type: 'auto'
})

const loading = ref(false)

const options = [
  { label: $gettext('Automatic'), value: 'auto' },
  { label: $gettext('Self-signed'), value: 'self-signed' }
]

const handleSubmit = () => {
  loading.value = true
  messageReactive = window.$message.loading($gettext('Please wait...'), {
    duration: 0
  })
  if (model.value.type == 'auto') {
    useRequest(cert.obtainAuto(id.value))
      .onSuccess(() => {
        window.$bus.emit('cert:refresh-cert')
        window.$bus.emit('cert:refresh-async')
        show.value = false
        window.$message.success($gettext('Issuance successful'))
      })
      .onComplete(() => {
        loading.value = false
        messageReactive?.destroy()
      })
  } else {
    useRequest(cert.obtainSelfSigned(id.value))
      .onSuccess(() => {
        window.$bus.emit('cert:refresh-cert')
        window.$bus.emit('cert:refresh-async')
        show.value = false
        window.$message.success($gettext('Issuance successful'))
      })
      .onComplete(() => {
        loading.value = false
        messageReactive?.destroy()
      })
  }
}
</script>

<template>
  <n-modal
    v-model:show="show"
    preset="card"
    :title="$gettext('Issue Certificate')"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
  >
    <n-form :model="model">
      <n-form-item path="type" :label="$gettext('Issuance Mode')">
        <n-select v-model:value="model.type" :options="options" />
      </n-form-item>
      <n-button type="info" block :loading="loading" :disabled="loading" @click="handleSubmit">{{ $gettext('Submit') }}</n-button>
    </n-form>
  </n-modal>
</template>

<style scoped lang="scss"></style>
