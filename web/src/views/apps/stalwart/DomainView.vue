<script setup lang="ts">
defineOptions({
  name: 'stalwart-domains',
})

import copy2clipboard from '@vavt/copy2clipboard'
import { NButton, NDataTable, NFlex, NTag, NText, NTooltip } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import stalwart from '@/api/apps/stalwart'
import cert from '@/api/panel/cert'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { useFormLayout } from '@/components/system/composables/useFormLayout'
import { useTablePagination } from '@/components/system/composables/useTablePagination'
import { formatDateTime } from '@/utils'

const { $gettext } = useGettext()
const { confirmDelete } = useConfirm()
const formLayout = useFormLayout()

const addModal = ref(false)
const addLoading = ref(false)
const addModel = ref({ name: '' })

const catchAllModal = ref(false)
const catchAllLoading = ref(false)
const catchAllModel = ref({ id: '', name: '', address: '' })

const dnsModal = ref(false)
const dnsLoading = ref(false)
const dnsPublishLoading = ref(false)
const dnsDomain = ref({ id: '', name: '' })
const dnsZone = ref('')
const dnsRecords = ref<any[]>([])
const dnsSelected = ref<string[]>([])
const dnsAccounts = ref<any[]>([])
const dnsAccountID = ref<number | null>(null)

const { loading, data, page, total, pageSize, refresh } = usePagination(
  (page, pageSize) => stalwart.domains(page, pageSize),
  {
    initialData: { total: 0, list: [] },
    initialPageSize: 20,
    total: (res: any) => res.total,
    data: (res: any) => res.items,
  },
)
const pagination = useTablePagination(page, pageSize, total)

const columns: any = [
  {
    title: $gettext('Domain'),
    key: 'name',
    minWidth: 220,
    resizable: true,
    ellipsis: { tooltip: true },
    render(row: any) {
      return h(NFlex, { size: 'small', align: 'center', wrap: false }, () => [
        h(NText, null, { default: () => row.name }),
        row.is_default
          ? h(
              NTag,
              { size: 'small', type: 'info', bordered: false },
              { default: () => $gettext('Default') },
            )
          : null,
      ])
    },
  },
  {
    title: $gettext('Catch-all Address'),
    key: 'catch_all',
    minWidth: 200,
    resizable: true,
    ellipsis: { tooltip: true },
    render: (row: any) => row.catch_all || '-',
  },
  {
    title: $gettext('Alias Domains'),
    key: 'aliases',
    minWidth: 200,
    resizable: true,
    render(row: any) {
      if (!row.aliases?.length) return '-'
      return h(NFlex, { size: 'small' }, () =>
        row.aliases.map((alias: string) =>
          h(NTag, { size: 'small', bordered: false }, { default: () => alias }),
        ),
      )
    },
  },
  {
    title: $gettext('Created At'),
    key: 'created_at',
    width: 180,
    ellipsis: { tooltip: true },
    render: (row: any) => formatDateTime(row.created_at),
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 300,
    hideInExcel: true,
    render(row: any) {
      return h(NFlex, { size: 'small', align: 'center', wrap: false }, () => [
        h(
          NButton,
          {
            size: 'small',
            type: 'primary',
            secondary: true,
            onClick: () => openDNS(row),
          },
          { default: () => $gettext('DNS Records') },
        ),
        h(
          NButton,
          {
            size: 'small',
            type: 'info',
            secondary: true,
            onClick: () => {
              catchAllModel.value = { id: row.id, name: row.name, address: row.catch_all }
              catchAllModal.value = true
            },
          },
          { default: () => $gettext('Catch-all') },
        ),
        h(
          NButton,
          {
            size: 'small',
            type: 'error',
            disabled: row.is_default,
            onClick: async () => {
              const ok = await confirmDelete({
                content: $gettext(
                  'Are you sure you want to delete domain %{ name }? All mailboxes under this domain and their mail will be deleted as well.',
                  { name: row.name },
                ),
              })
              if (ok) handleDelete(row.id)
            },
          },
          { default: () => $gettext('Delete') },
        ),
      ])
    },
  },
]

const dnsStatusType = (status: string) => {
  switch (status) {
    case 'ok':
      return 'success'
    case 'missing':
      return 'error'
    case 'mismatch':
      return 'warning'
    default:
      return 'default'
  }
}

const dnsStatusText = (status: string) => {
  switch (status) {
    case 'ok':
      return $gettext('Active')
    case 'missing':
      return $gettext('Missing')
    case 'mismatch':
      return $gettext('Mismatch')
    default:
      return $gettext('Unknown')
  }
}

const dnsColumns: any = [
  {
    type: 'selection',
    disabled: (row: any) => row.status === 'ok',
  },
  {
    title: $gettext('Type'),
    key: 'type',
    width: 90,
    render(row: any) {
      return h(NFlex, { size: 'small', align: 'center', wrap: false }, () => [
        h(NText, null, { default: () => row.type }),
        h(
          NTag,
          { size: 'tiny', bordered: false, type: row.group === 'required' ? 'warning' : 'default' },
          {
            default: () => (row.group === 'required' ? $gettext('Required') : $gettext('Optional')),
          },
        ),
      ])
    },
  },
  {
    title: $gettext('Name'),
    key: 'name',
    minWidth: 220,
    resizable: true,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Value'),
    key: 'value',
    minWidth: 320,
    resizable: true,
    ellipsis: { tooltip: true },
    render(row: any) {
      return h(
        NText,
        {
          class: 'cursor-pointer',
          title: $gettext('Click to copy'),
          onClick: () => {
            copy2clipboard(row.value).then(() => {
              window.$message.success($gettext('Copied successfully'))
            })
          },
        },
        { default: () => row.value },
      )
    },
  },
  {
    title: $gettext('Status'),
    key: 'status',
    width: 110,
    render(row: any) {
      const tag = h(
        NTag,
        { size: 'small', bordered: false, type: dnsStatusType(row.status) },
        { default: () => dnsStatusText(row.status) },
      )
      if (row.status !== 'mismatch' || !row.found) return tag
      return h(NTooltip, null, {
        trigger: () => tag,
        default: () => $gettext('Current value: %{ value }', { value: row.found }),
      })
    },
  },
]

// 必需在前、可选在后，同组内保持服务端顺序
const sortedRecords = computed(() =>
  [...dnsRecords.value].sort(
    (a, b) => Number(b.group === 'required') - Number(a.group === 'required'),
  ),
)

const dnsAccountOptions = computed(() =>
  dnsAccounts.value.map((item: any) => ({ label: `${item.name} (${item.type})`, value: item.id })),
)

const loadDNS = async () => {
  dnsLoading.value = true
  try {
    const res = await stalwart.domainDNS(dnsDomain.value.id)
    dnsZone.value = res.zone
    dnsRecords.value = res.records
    dnsSelected.value = res.records
      .filter((r: any) => r.group === 'required' && r.status !== 'ok')
      .map((r: any) => r.id)
  } finally {
    dnsLoading.value = false
  }
}

const openDNS = async (row: any) => {
  dnsDomain.value = { id: row.id, name: row.name }
  dnsRecords.value = []
  dnsSelected.value = []
  dnsModal.value = true
  if (!dnsAccounts.value.length) {
    useRequest(cert.dns(1, 10000)).onSuccess(({ data }) => {
      dnsAccounts.value = data.items
      dnsAccountID.value = data.items[0]?.id ?? null
    })
  }
  await loadDNS()
}

const handlePublish = () => {
  if (!dnsAccountID.value) return
  dnsPublishLoading.value = true
  useRequest(stalwart.domainDNSPublish(dnsDomain.value.id, dnsAccountID.value, dnsSelected.value))
    .onSuccess(({ data }) => {
      window.$message.success(
        $gettext('%{ count } records added', { count: String(data.published) }),
      )
      loadDNS()
    })
    .onComplete(() => {
      dnsPublishLoading.value = false
    })
}

const handleAdd = () => {
  addLoading.value = true
  useRequest(stalwart.domainCreate(addModel.value.name))
    .onSuccess(() => {
      refresh()
      addModal.value = false
      addModel.value.name = ''
      window.$message.success($gettext('Added successfully'))
    })
    .onComplete(() => {
      addLoading.value = false
    })
}

const handleCatchAll = () => {
  catchAllLoading.value = true
  useRequest(stalwart.domainCatchAll(catchAllModel.value.id, catchAllModel.value.address))
    .onSuccess(() => {
      refresh()
      catchAllModal.value = false
      window.$message.success($gettext('Saved successfully'))
    })
    .onComplete(() => {
      catchAllLoading.value = false
    })
}

const handleDelete = (id: string) => {
  useRequest(stalwart.domainDelete(id)).onSuccess(() => {
    refresh()
    window.$message.success($gettext('Deleted successfully'))
  })
}

onMounted(() => {
  refresh()
})
</script>

<template>
  <n-flex vertical>
    <n-flex>
      <n-button type="primary" @click="addModal = true">
        {{ $gettext('Add Domain') }}
      </n-button>
    </n-flex>
    <n-data-table
      v-model:page="page"
      v-model:pageSize="pageSize"
      striped
      remote
      :scroll-x="1100"
      :loading="loading"
      :columns="columns"
      :data="data"
      :row-key="(row: any) => row.id"
      :pagination="pagination"
    />
  </n-flex>
  <app-modal v-model:show="addModal" :title="$gettext('Add Domain')">
    <n-flex vertical>
      <n-alert type="info">
        {{
          $gettext(
            'DKIM keys are generated automatically. After adding, open DNS Records to publish the MX, SPF, DKIM and DMARC records.',
          )
        }}
      </n-alert>
      <n-form :model="addModel" v-bind="formLayout">
        <n-form-item path="name" :label="$gettext('Domain')">
          <n-input v-model:value="addModel.name" placeholder="example.com" @keydown.enter.prevent />
        </n-form-item>
      </n-form>
      <n-button type="info" block :loading="addLoading" :disabled="addLoading" @click="handleAdd">
        {{ $gettext('Submit') }}
      </n-button>
    </n-flex>
  </app-modal>
  <app-modal v-model:show="catchAllModal" :title="$gettext('Catch-all Address')">
    <n-flex vertical>
      <n-alert type="info">
        {{
          $gettext(
            'Mail sent to addresses that do not exist under %{ domain } will be delivered to this mailbox. Leave empty to disable.',
            { domain: catchAllModel.name },
          )
        }}
      </n-alert>
      <n-form :model="catchAllModel" v-bind="formLayout">
        <n-form-item path="address" :label="$gettext('Mailbox')">
          <n-input
            v-model:value="catchAllModel.address"
            :placeholder="`postmaster@${catchAllModel.name}`"
            @keydown.enter.prevent
          />
        </n-form-item>
      </n-form>
      <n-button
        type="info"
        block
        :loading="catchAllLoading"
        :disabled="catchAllLoading"
        @click="handleCatchAll"
      >
        {{ $gettext('Save') }}
      </n-button>
    </n-flex>
  </app-modal>
  <app-modal
    v-model:show="dnsModal"
    size="lg"
    :title="$gettext('DNS Records of %{ domain }', { domain: dnsDomain.name })"
  >
    <n-flex vertical>
      <n-alert type="info">
        {{
          $gettext(
            'Add these records at your DNS provider for zone %{ zone }. Required records are needed for mail to be accepted by other servers; optional ones enable client autoconfig, MTA-STS and TLS reporting.',
            { zone: dnsZone || dnsDomain.name },
          )
        }}
      </n-alert>
      <n-data-table
        v-model:checked-row-keys="dnsSelected"
        striped
        :loading="dnsLoading"
        :columns="dnsColumns"
        :data="sortedRecords"
        :row-key="(row: any) => row.id"
        :pagination="false"
        :scroll-x="900"
      />
      <n-card :title="$gettext('Publish via DNS Provider')" size="small">
        <n-flex v-if="dnsAccountOptions.length" vertical>
          <n-text depth="3">
            {{
              $gettext(
                'Selected records that are not active yet will be added through the DNS account below. Existing records are not modified.',
              )
            }}
          </n-text>
          <n-flex align="center">
            <n-select
              v-model:value="dnsAccountID"
              class="w-80"
              :options="dnsAccountOptions"
              :placeholder="$gettext('Select DNS account')"
            />
            <n-button
              type="primary"
              :loading="dnsPublishLoading"
              :disabled="dnsPublishLoading || !dnsAccountID || !dnsSelected.length"
              @click="handlePublish"
            >
              {{ $gettext('Add Selected Records') }}
            </n-button>
            <n-button :loading="dnsLoading" :disabled="dnsLoading" @click="loadDNS">
              {{ $gettext('Recheck') }}
            </n-button>
          </n-flex>
        </n-flex>
        <n-text v-else depth="3">
          {{
            $gettext(
              'No DNS account is configured. Add one under Certificates → DNS Accounts to publish records with one click, or add them manually.',
            )
          }}
        </n-text>
      </n-card>
    </n-flex>
  </app-modal>
</template>
