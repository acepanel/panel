<script setup lang="ts">
import { NButton } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import firewall from '@/api/panel/firewall'

const { $gettext } = useGettext()
const show = defineModel<boolean>('show', { type: Boolean, required: true })
const loading = ref(false)

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

const handleCreate = () => {
  loading.value = true
  useRequest(firewall.createForward(createModel.value))
    .onSuccess(() => {
      show.value = false
      createModel.value = {
        protocol: 'tcp',
        port: 8080,
        target_ip: '127.0.0.1',
        target_port: 80,
      }
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
    :title="$gettext('Create Forwarding')"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
    @close="show = false"
  >
    <n-form :model="createModel">
      <n-form-item path="protocols" :label="$gettext('Transport Protocol')">
        <n-select v-model:value="createModel.protocol" :options="protocols" />
      </n-form-item>
      <n-form-item path="address" :label="$gettext('Target IP')">
        <n-input v-model:value="createModel.target_ip" placeholder="127.0.0.1" />
      </n-form-item>
      <n-row :gutter="[0, 24]">
        <n-col :span="12">
          <n-form-item path="address" :label="$gettext('Source Port')">
            <n-input-number
              v-model:value="createModel.port"
              :min="1"
              :max="65535"
              placeholder="8080"
            />
          </n-form-item>
        </n-col>
        <n-col :span="12">
          <n-form-item path="address" :label="$gettext('Target Port')">
            <n-input-number
              v-model:value="createModel.target_port"
              :min="1"
              :max="65535"
              placeholder="80"
            />
          </n-form-item>
        </n-col>
      </n-row>
    </n-form>
    <n-button type="info" block :loading="loading" :disabled="loading" @click="handleCreate">
      {{ $gettext('Submit') }}
    </n-button>
  </n-modal>
</template>

<style scoped lang="scss"></style>
