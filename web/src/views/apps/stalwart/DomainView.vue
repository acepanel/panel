<script setup lang="ts">
defineOptions({
  name: 'stalwart-domains',
})

import copy2clipboard from '@vavt/copy2clipboard'
import { NButton, NFlex, NTag, NText, NTooltip } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import stalwart from '@/api/apps/stalwart'
import cert from '@/api/panel/cert'
import AppButton from '@/components/system/AppButton.vue'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { useFormLayout } from '@/components/system/composables/useFormLayout'
import { usePagedTable } from '@/components/system/composables/usePagedTable'
import { formatDateTime } from '@/utils'

const { $gettext } = useGettext()
const { confirmDelete } = useConfirm()
const formLayout = useFormLayout()

const addModal = ref(false)
const addModel = ref({ name: '' })

const catchAllModal = ref(false)
const catchAllModel = ref({ id: '', name: '', address: '' })

const dnsModal = ref(false)
const dnsLoading = ref(false)
const dnsDomain = ref({ id: '', name: '' })
const dnsZone = ref('')
const dnsRecords = ref<any[]>([])
const dnsSelected = ref<string[]>([])
const dnsAccounts = ref<any[]>([])
const dnsAccountsLoading = ref(false)
const dnsAccountID = ref<number | null>(null)

const { loading, data, page, pageSize, refresh, pagination } = usePagedTable((page, pageSize) =>
  stalwart.domains(page, pageSize),
)

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
          AppButton,
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
              if (ok) await handleDelete(row.id)
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
  if (!dnsAccounts.value.length && !dnsAccountsLoading.value) {
    dnsAccountsLoading.value = true
    useRequest(cert.dns(1, 10000))
      .onSuccess(({ data }) => {
        dnsAccounts.value = data.items
        dnsAccountID.value = data.items[0]?.id ?? null
      })
      .onComplete(() => {
        dnsAccountsLoading.value = false
      })
  }
  await loadDNS()
}

const handlePublish = async () => {
  if (!dnsAccountID.value) return
  const data = await stalwart.domainDNSPublish(
    dnsDomain.value.id,
    dnsAccountID.value,
    dnsSelected.value,
  )
  window.$message.success($gettext('%{ count } records added', { count: String(data.published) }))
  loadDNS()
}

const handleAdd = async () => {
  await stalwart.domainCreate(addModel.value.name)
  refresh()
  addModal.value = false
  addModel.value.name = ''
  window.$message.success($gettext('Added successfully'))
}

const handleCatchAll = async () => {
  await stalwart.domainCatchAll(catchAllModel.value.id, catchAllModel.value.address)
  refresh()
  catchAllModal.value = false
  window.$message.success($gettext('Saved successfully'))
}

const handleDelete = async (id: string) => {
  await stalwart.domainDelete(id)
  refresh()
  window.$message.success($gettext('Deleted successfully'))
}
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
      <app-button type="info" block @click="handleAdd">
        {{ $gettext('Submit') }}
      </app-button>
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
      <app-button type="info" block @click="handleCatchAll">
        {{ $gettext('Save') }}
      </app-button>
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
        <n-spin :show="dnsAccountsLoading">
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
                class="md:!w-80"
                :options="dnsAccountOptions"
                :placeholder="$gettext('Select DNS account')"
              />
              <app-button
                type="primary"
                :disabled="!dnsAccountID || !dnsSelected.length"
                @click="handlePublish"
              >
                {{ $gettext('Add Selected Records') }}
              </app-button>
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
        </n-spin>
      </n-card>
    </n-flex>
  </app-modal>
</template>
