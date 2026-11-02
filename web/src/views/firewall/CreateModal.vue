<script setup lang="ts">
import { NButton } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import firewall from '@/api/panel/firewall'
import ListInput from '@/components/common/ListInput.vue'

const { $gettext } = useGettext()
const show = defineModel<boolean>('show', { type: Boolean, required: true })
const emit = defineEmits<{ created: [] }>()
const loading = ref(false)

type PortMode = 'single' | 'range'

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
  portMode: 'single' as PortMode,
  family: 'ipv4',
  protocol: 'tcp',
  port_start: 80,
  port_end: 80,
  address: [] as string[],
  strategy: 'accept',
  direction: 'in',
})

const createModel = ref(newCreateModel())

// 入站规则的地址是来源，出站规则的地址是目的地，标签跟随方向避免被当成目标地址
const addressLabel = computed(() =>
  createModel.value.direction === 'in' ? $gettext('Source address') : $gettext('Target address'),
)

watch(show, (value) => {
  if (!value) return
  createModel.value = newCreateModel()
  loading.value = false
})

watch(
  () => createModel.value.portMode,
  () => {
    createModel.value.port_end = createModel.value.port_start
  },
)

watch(
  () => createModel.value.port_start,
  (newStart) => {
    if (createModel.value.portMode === 'range' && createModel.value.port_end < newStart) {
      createModel.value.port_end = newStart
    }
  },
)

const handleCreate = async () => {
  loading.value = true
  try {
    const addresses = createModel.value.address.length ? createModel.value.address : ['']
    const promises = addresses.map((address) =>
      useRequest(
        firewall.createRule({
          family: createModel.value.family,
          protocol: createModel.value.protocol,
          port_start: createModel.value.port_start,
          port_end:
            createModel.value.portMode === 'single'
              ? createModel.value.port_start
              : createModel.value.port_end,
          address,
          strategy: createModel.value.strategy,
          direction: createModel.value.direction,
        }),
      ).onSuccess(() => {
        window.$message.success($gettext('%{ address } created successfully', { address }))
      }),
    )
    await Promise.all(promises)
    emit('created')
    show.value = false
  } finally {
    loading.value = false
  }
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
      <n-form-item path="portMode" :label="$gettext('Port Type')">
        <n-radio-group v-model:value="createModel.portMode" size="small">
          <n-radio-button value="single">{{ $gettext('Single Port') }}</n-radio-button>
          <n-radio-button value="range">{{ $gettext('Port Range') }}</n-radio-button>
        </n-radio-group>
      </n-form-item>
      <n-form-item
        v-if="createModel.portMode === 'single'"
        path="port_start"
        :label="$gettext('Port')"
      >
        <n-input-number
          v-model:value="createModel.port_start"
          :min="1"
          :max="65535"
          placeholder="80"
        />
      </n-form-item>
      <n-grid v-else cols="1 m:24" responsive="screen">
        <n-form-item-gi :span="12" path="port_start" :label="$gettext('Start Port')">
          <n-input-number
            v-model:value="createModel.port_start"
            :min="1"
            :max="65535"
            placeholder="80"
          />
        </n-form-item-gi>
        <n-form-item-gi :span="12" path="port_end" :label="$gettext('End Port')">
          <n-input-number
            v-model:value="createModel.port_end"
            :min="createModel.port_start"
            :max="65535"
            placeholder="80"
          />
        </n-form-item-gi>
      </n-grid>
      <n-form-item path="direction" :label="$gettext('Direction')">
        <n-select v-model:value="createModel.direction" :options="directions" />
      </n-form-item>
      <n-form-item path="address" :label="addressLabel">
        <ListInput
          v-model:value="createModel.address"
          show-sort-button
          :placeholder="$gettext('IP or IP range: 172.16.0.1 or 172.16.0.0/16')"
        />
        <template #feedback>
          {{ $gettext('Leave empty to match all addresses') }}
        </template>
      </n-form-item>
      <n-form-item path="strategy" :label="$gettext('Strategy')">
        <n-select v-model:value="createModel.strategy" :options="strategies" />
      </n-form-item>
    </n-form>
    <n-button type="info" block :loading="loading" :disabled="loading" @click="handleCreate">
      {{ $gettext('Submit') }}
    </n-button>
  </app-modal>
</template>

<style scoped lang="scss"></style>
