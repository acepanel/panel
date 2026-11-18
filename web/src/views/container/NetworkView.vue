<script setup lang="ts">
import { NDataTable, NFlex, NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import container from '@/api/panel/container'
import AppButton from '@/components/system/AppButton.vue'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { usePagedTable } from '@/components/system/composables/usePagedTable'
import { formatDateTime } from '@/utils'

const { $gettext } = useGettext()
const { confirmDelete } = useConfirm()

const createModel = ref({
  name: '',
  driver: 'bridge',
  ipv4: {
    enabled: false,
    subnet: '',
    gateway: '',
    ip_range: '',
  },
  ipv6: {
    enabled: false,
    subnet: '',
    gateway: '',
    ip_range: '',
  },
  options: [],
  labels: [],
})

const options = [
  { label: 'bridge', value: 'bridge' },
  { label: 'host', value: 'host' },
  { label: 'overlay', value: 'overlay' },
  { label: 'macvlan', value: 'macvlan' },
  { label: 'ipvlan', value: 'ipvlan' },
  { label: 'none', value: 'none' },
]

const createModal = ref(false)

const selectedRowKeys = ref<any>([])

const columns: any = [
  { type: 'selection', fixed: 'left' },
  {
    title: $gettext('Name'),
    key: 'name',
    minWidth: 150,
    resizable: true,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Driver'),
    key: 'driver',
    width: 100,
    resizable: true,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Scope'),
    key: 'scope',
    width: 100,
    resizable: true,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Subnet'),
    key: 'subnet',
    minWidth: 150,
    resizable: true,
    ellipsis: { tooltip: true },
    render(row: any): any {
      return h(NFlex, null, {
        default: () =>
          row.ipam.config.map((tag: any) =>
            h(NTag, null, {
              default: () => tag.subnet,
            }),
          ),
      })
    },
  },
  {
    title: $gettext('Gateway'),
    key: 'gateway',
    width: 150,
    resizable: true,
    ellipsis: { tooltip: true },
    render(row: any): any {
      return h(NFlex, null, {
        default: () =>
          row.ipam.config.map((tag: any) =>
            h(NTag, null, {
              default: () => tag.gateway,
            }),
          ),
      })
    },
  },
  {
    title: $gettext('Creation Time'),
    key: 'created_at',
    width: 200,
    resizable: true,
    render(row: any) {
      return formatDateTime(row.created_at)
    },
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 120,
    hideInExcel: true,
    render(row: any) {
      return h(
        AppButton,
        {
          size: 'small',
          type: 'error',
          disabled: row.name === 'acepanel-network', // 不允许删除 acepanel-network
          onClick: async () => {
            const ok = await confirmDelete({
              content: $gettext('Are you sure you want to delete?'),
            })
            if (ok) await handleDelete(row)
          },
        },
        { default: () => $gettext('Delete') },
      )
    },
  },
]

const { loading, data, page, pageSize, refresh, pagination } = usePagedTable((page, pageSize) =>
  container.networkList(page, pageSize),
)

const handleDelete = async (row: any) => {
  await container.networkRemove(row.id)
  refresh()
  window.$message.success($gettext('Delete successful'))
}

const handlePrune = async () => {
  await container.networkPrune()
  refresh()
  window.$message.success($gettext('Cleanup successful'))
}

const handleBulkDelete = async () => {
  // 排除受保护的 acepanel-network，避免触发后端拦截
  const protectedIds = new Set(
    data.value
      ?.filter((item: any) => item.name === 'acepanel-network')
      .map((item: any) => item.id) ?? [],
  )
  const ids = selectedRowKeys.value.filter((id: any) => !protectedIds.has(id))
  const promises = ids.map((id: any) => container.networkRemove(id))
  await Promise.all(promises)

  selectedRowKeys.value = []
  refresh()
  window.$message.success($gettext('Deleted successfully'))
}

const handleCreate = () => {
  loading.value = true
  useRequest(container.networkCreate(createModel.value))
    .onSuccess(() => {
      refresh()
      window.$message.success($gettext('Created successfully'))
    })
    .onComplete(() => {
      loading.value = false
      createModal.value = false
    })
}
</script>

<template>
  <n-flex vertical :size="20">
    <n-flex>
      <n-button type="primary" @click="createModal = true">{{
        $gettext('Create Network')
      }}</n-button>
      <app-button type="primary" ghost @click="handlePrune">
        {{ $gettext('Cleanup Networks') }}
      </app-button>
      <ConfirmDialog
        type="danger"
        :content="$gettext('Are you sure you want to delete the selected networks?')"
        @confirm="handleBulkDelete"
      >
        <template #trigger>
          <app-button type="error" :disabled="selectedRowKeys.length === 0" ghost>
            {{ $gettext('Delete') }}
          </app-button>
        </template>
      </ConfirmDialog>
    </n-flex>
    <n-data-table
      v-model:checked-row-keys="selectedRowKeys"
      v-model:page="page"
      v-model:pageSize="pageSize"
      striped
      remote
      :loading="loading"
      :scroll-x="1100"
      :data="data"
      :columns="columns"
      :row-key="(row: any) => row.id"
      :pagination="pagination"
    />
  </n-flex>
  <app-modal v-model:show="createModal" :title="$gettext('Create Network')">
    <n-form :model="createModel">
      <n-form-item path="name" :label="$gettext('Network Name')">
        <n-input v-model:value="createModel.name" type="text" @keydown.enter.prevent />
      </n-form-item>
      <n-form-item path="driver" :label="$gettext('Driver')">
        <n-select
          :options="options"
          v-model:value="createModel.driver"
          type="text"
          @keydown.enter.prevent
        >
        </n-select>
      </n-form-item>
      <n-form-item path="ipv4" label="IPV4">
        <n-switch v-model:value="createModel.ipv4.enabled" />
      </n-form-item>
      <n-form-item v-if="createModel.ipv4.enabled" path="subnet" :label="$gettext('Subnet')">
        <n-input
          v-model:value="createModel.ipv4.subnet"
          type="text"
          @keydown.enter.prevent
          :placeholder="$gettext('172.16.10.0/24')"
        />
      </n-form-item>
      <n-form-item v-if="createModel.ipv4.enabled" path="gateway" :label="$gettext('Gateway')">
        <n-input
          v-model:value="createModel.ipv4.gateway"
          type="text"
          @keydown.enter.prevent
          :placeholder="$gettext('172.16.10.254')"
        />
      </n-form-item>
      <n-form-item v-if="createModel.ipv4.enabled" path="ip_range" :label="$gettext('IP Range')">
        <n-input
          v-model:value="createModel.ipv4.ip_range"
          type="text"
          @keydown.enter.prevent
          :placeholder="$gettext('172.16.10.0/24')"
        />
      </n-form-item>
      <n-form-item path="ipv6" label="IPV6">
        <n-switch v-model:value="createModel.ipv6.enabled" />
      </n-form-item>
      <n-form-item v-if="createModel.ipv6.enabled" path="subnet" :label="$gettext('Subnet')">
        <n-input
          v-model:value="createModel.ipv6.subnet"
          type="text"
          @keydown.enter.prevent
          :placeholder="$gettext('2408:400e::/48')"
        />
      </n-form-item>
      <n-form-item v-if="createModel.ipv6.enabled" path="gateway" :label="$gettext('Gateway')">
        <n-input
          v-model:value="createModel.ipv6.gateway"
          type="text"
          @keydown.enter.prevent
          :placeholder="$gettext('2408:400e::1')"
        />
      </n-form-item>
      <n-form-item v-if="createModel.ipv6.enabled" path="ip_range" :label="$gettext('IP Range')">
        <n-input
          v-model:value="createModel.ipv6.ip_range"
          type="text"
          @keydown.enter.prevent
          :placeholder="$gettext('2408:400e::/64')"
        />
      </n-form-item>
      <n-form-item path="env" :label="$gettext('Labels')">
        <n-dynamic-input
          v-model:value="createModel.labels"
          preset="pair"
          :key-placeholder="$gettext('Label Name')"
          :value-placeholder="$gettext('Label Value')"
        />
      </n-form-item>
      <n-form-item path="env" :label="$gettext('Options')">
        <n-dynamic-input
          v-model:value="createModel.options"
          preset="pair"
          :key-placeholder="$gettext('Option Name')"
          :value-placeholder="$gettext('Option Value')"
        />
      </n-form-item>
    </n-form>
    <n-button type="info" block :loading="loading" :disabled="loading" @click="handleCreate">
      {{ $gettext('Submit') }}
    </n-button>
  </app-modal>
</template>
