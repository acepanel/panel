<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

import firewall from '@/api/panel/firewall'

const { $gettext } = useGettext()
const show = defineModel<boolean>('show', { type: Boolean, required: true })

const protocols = [
  {
    label: 'TCP',
    value: 'tcp',
  },
  {
    label: 'UDP',
    value: 'udp',
  },
  {
    label: 'TCP/UDP',
    value: 'tcp/udp',
  },
]

const createModel = ref({
  protocol: 'tcp',
  port: 8080,
  target_ip: '127.0.0.1',
  target_port: 80,
})

const handleCreate = async () => {
  await firewall.createForward(createModel.value)
  show.value = false
  createModel.value = {
    protocol: 'tcp',
    port: 8080,
    target_ip: '127.0.0.1',
    target_port: 80,
  }
  window.$message.success($gettext('Created successfully'))
}
</script>

<template>
  <app-modal v-model:show="show" :title="$gettext('Create Forwarding')">
    <n-form :model="createModel">
      <n-form-item path="protocols" :label="$gettext('Transport Protocol')">
        <n-select v-model:value="createModel.protocol" :options="protocols" />
      </n-form-item>
      <n-form-item path="address" :label="$gettext('Target IP')">
        <n-input v-model:value="createModel.target_ip" placeholder="127.0.0.1" />
      </n-form-item>
      <n-grid cols="1 m:24" responsive="screen">
        <n-form-item-gi :span="12" path="address" :label="$gettext('Source Port')">
          <n-input-number
            v-model:value="createModel.port"
            :min="1"
            :max="65535"
            placeholder="8080"
          />
        </n-form-item-gi>
        <n-form-item-gi :span="12" path="address" :label="$gettext('Target Port')">
          <n-input-number
            v-model:value="createModel.target_port"
            :min="1"
            :max="65535"
            placeholder="80"
          />
        </n-form-item-gi>
      </n-grid>
    </n-form>
    <app-button type="info" block @click="handleCreate">
      {{ $gettext('Submit') }}
    </app-button>
  </app-modal>
</template>

<style scoped lang="scss"></style>
