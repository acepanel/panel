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

const newCreateModel = () => ({
  family: 'ipv4',
  protocol: 'tcp',
  address: [] as string[],
  strategy: 'accept',
  direction: 'in',
})

const createModel = ref(newCreateModel())

// 弹窗复用同一实例，每次打开时重置表单，避免残留上次创建的地址
watch(show, (value) => {
  if (!value) return
  createModel.value = newCreateModel()
})

const handleCreate = async () => {
  if (!createModel.value.address.length) {
    createModel.value.address.push('')
  }
  const promises = createModel.value.address.map(async (address) => {
    await firewall.createIpRule({
      ...createModel.value,
      address,
    })
    window.$message.success($gettext('%{ address } created successfully', { address: address }))
  })
  await Promise.all(promises)
  show.value = false
}
</script>

<template>
  <app-modal v-model:show="show" :title="$gettext('Create Rule')">
    <n-form :model="createModel">
      <n-form-item path="protocols" :label="$gettext('Transport Protocol')">
        <n-select v-model:value="createModel.protocol" :options="protocols" />
      </n-form-item>
      <n-form-item path="family" :label="$gettext('Network Protocol')">
        <n-select v-model:value="createModel.family" :options="families" />
      </n-form-item>
      <n-form-item path="address" :label="$gettext('IP Address')">
        <ListInput
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
    <app-button type="info" block @click="handleCreate">
      {{ $gettext('Submit') }}
    </app-button>
  </app-modal>
</template>

<style scoped lang="scss"></style>
