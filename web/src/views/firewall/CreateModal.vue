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

const families = [
  {
    label: 'IPv4',
    value: 'ipv4',
  },
  {
    label: 'IPv6',
    value: 'ipv6',
  },
]

const strategies = [
  {
    label: $gettext('Accept'),
    value: 'accept',
  },
  {
    label: $gettext('Drop'),
    value: 'drop',
  },
  {
    label: $gettext('Reject'),
    value: 'reject',
  },
]

const directions = [
  {
    label: $gettext('Inbound'),
    value: 'in',
  },
  {
    label: $gettext('Outbound'),
    value: 'out',
  },
]

const createModel = ref({
  family: 'ipv4',
  protocol: 'tcp',
  port_start: 80,
  port_end: 80,
  address: [] as string[],
  strategy: 'accept',
  direction: 'in',
})

// 当起始端口改变时，同步更新结束端口（如果结束端口小于起始端口）
watch(
  () => createModel.value.port_start,
  (newStart) => {
    if (createModel.value.port_end < newStart) {
      createModel.value.port_end = newStart
    }
  },
)

const handleCreate = async () => {
  loading.value = true
  if (!createModel.value.address.length) {
    createModel.value.address.push('')
  }
  const promises = createModel.value.address.map((address) =>
    useRequest(
      firewall.createRule({
        ...createModel.value,
        address,
      }),
    ).onSuccess(() => {
      window.$message.success($gettext('%{ address } created successfully', { address: address }))
    }),
  )
  await Promise.all(promises)
  show.value = false
  loading.value = false
}
</script>

<template>
  <n-modal
    v-model:show="show"
    preset="card"
    :title="$gettext('Create Rule')"
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
      <n-form-item path="family" :label="$gettext('Network Protocol')">
        <n-select v-model:value="createModel.family" :options="families" />
      </n-form-item>
      <n-row :gutter="[0, 24]">
        <n-col :span="12">
          <n-form-item path="port_start" :label="$gettext('Start Port')">
            <n-input-number
              v-model:value="createModel.port_start"
              :min="1"
              :max="65535"
              placeholder="80"
            />
          </n-form-item>
        </n-col>
        <n-col :span="12">
          <n-form-item path="port_end" :label="$gettext('End Port')">
            <n-input-number
              v-model:value="createModel.port_end"
              :min="createModel.port_start"
              :max="65535"
              placeholder="80"
            />
          </n-form-item>
        </n-col>
      </n-row>
      <n-form-item path="address" :label="$gettext('Target')">
        <n-dynamic-input
          v-model:value="createModel.address"
          show-sort-button
          :placeholder="$gettext('IP or IP range: 172.16.0.1 or 172.16.0.0/16')"
        />
      </n-form-item>
      <n-form-item path="strategy" :label="$gettext('Strategy')">
        <n-select v-model:value="createModel.strategy" :options="strategies" />
      </n-form-item>
      <n-form-item path="strategy" :label="$gettext('Direction')">
        <n-select v-model:value="createModel.direction" :options="directions" />
      </n-form-item>
    </n-form>
    <n-button type="info" block :loading="loading" :disabled="loading" @click="handleCreate">
      {{ $gettext('Submit') }}
    </n-button>
  </n-modal>
</template>

<style scoped lang="scss"></style>
