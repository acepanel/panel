<script setup lang="ts">
import { NButton, NDataTable, NFlex, NPopover, NSpace, NSwitch, NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import cert from '@/api/panel/cert'
import AppButton from '@/components/system/AppButton.vue'
import { useBus } from '@/components/system/composables/useBus'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { usePagedTable } from '@/components/system/composables/usePagedTable'
import { useResponsive } from '@/components/system/composables/useResponsive'
import { formatDateTime } from '@/utils'
import CertModal from '@/views/cert/CertModal.vue'
import ObtainModal from '@/views/cert/ObtainModal.vue'

const { $gettext } = useGettext()
const { confirmDelete } = useConfirm()
const { isMobile } = useResponsive()

const props = defineProps({
  algorithms: {
    type: Array<any>,
    required: true,
  },
  websites: {
    type: Array<any>,
    required: true,
  },
  accounts: {
    type: Array<any>,
    required: true,
  },
  dns: {
    type: Array<any>,
    required: true,
  },
})

const { algorithms, websites, accounts, dns } = toRefs(props)

const editModal = ref(false)
const editItem = ref<any>(null)
const showModal = ref(false)
const showModel = ref<any>({
  cert: '',
  key: '',
})
const deployModal = ref(false)
const deployModel = ref<any>({
  id: null,
  websites: [],
  enable_https: true,
})
const deployDomains = ref<string[]>([])
const deployWebsiteOptions = computed(() =>
  websites.value.map((website: any) => ({
    label: website.domains?.length
      ? `${website.label} (${website.domains.join(', ')})`
      : website.label,
    value: website.value,
  })),
)
const obtain = ref(false)
const obtainCert = ref(0)
const obtainMode = ref<'obtain' | 'renew' | 'refresh'>('obtain')

const normalizeDomain = (domain: string) => {
  const value = domain.trim().toLowerCase().replace(/\.$/, '')
  const wildcard = value.startsWith('*.')
  const hostname = wildcard ? value.slice(2) : value

  try {
    const normalized = new URL(`http://${hostname}`).hostname
    return wildcard ? `*.${normalized}` : normalized
  } catch {
    return value
  }
}

const isDomainCovered = (certDomain: string, websiteDomain: string) => {
  const cert = normalizeDomain(certDomain)
  const domain = normalizeDomain(websiteDomain)
  if (cert === domain) return true
  if (!cert.startsWith('*.')) return false

  const suffix = cert.slice(2)
  if (!domain.endsWith(`.${suffix}`)) return false

  const prefix = domain.slice(0, -(suffix.length + 1))
  return prefix !== '' && !prefix.includes('.')
}

const handleOpenDeploy = (row: any) => {
  deployModel.value.id = row.id
  deployModel.value.websites = row.website_ids || []
  deployModel.value.enable_https = true
  deployDomains.value = row.domains || []
  deployModal.value = true
}

const handleMatchWebsites = () => {
  const matched = websites.value
    .filter((website: any) => {
      const domains = website.domains?.filter(Boolean) || []
      return (
        domains.length > 0 &&
        domains.every((domain: string) =>
          deployDomains.value.some((certDomain) => isDomainCovered(certDomain, domain)),
        )
      )
    })
    .map((website: any) => website.value)

  if (matched.length === 0) {
    window.$message.warning($gettext('No matching websites found'))
    return
  }

  deployModel.value.websites = [...new Set([...deployModel.value.websites, ...matched])]
}

const columns: any = [
  {
    title: $gettext('Domain'),
    key: 'domains',
    minWidth: 200,
    resizable: true,
    render(row: any) {
      const domains: string[] = row.domains ?? []
      if (domains.length == 0) {
        return h('span', { class: 'text-text-tertiary' }, $gettext('None'))
      }
      // 只摆第一个，其余收进悬浮列表，多域名证书不会把整行撑高
      const [first, ...rest] = domains
      return h(NFlex, { align: 'center', size: 8, wrap: false }, () => [
        h('span', { class: 'truncate', title: first }, first),
        rest.length > 0 &&
          h(
            NPopover,
            {
              trigger: 'hover',
              placement: 'bottom-start',
              scrollable: true,
              style: 'max-height: 280px',
            },
            {
              trigger: () =>
                h(
                  NTag,
                  {
                    size: 'small',
                    round: true,
                    bordered: false,
                    class: 'flex-none cursor-default',
                  },
                  () => `+${rest.length}`,
                ),
              default: () =>
                h(
                  'div',
                  { class: 'flex flex-col gap-1 py-0.5' },
                  domains.map((domain) => h('span', { class: 'text-13px' }, domain)),
                ),
            },
          ),
      ])
    },
  },
  {
    title: $gettext('Type'),
    key: 'type',
    width: 100,
    render(row: any) {
      return h(
        NTag,
        {
          type: 'info',
          bordered: false,
        },
        {
          default: () => {
            switch (row.type) {
              case 'P256':
                return 'EC 256'
              case 'P384':
                return 'EC 384'
              case '2048':
                return 'RSA 2048'
              case '4096':
                return 'RSA 4096'
              default:
                return $gettext('Upload')
            }
          },
        },
      )
    },
  },
  {
    title: $gettext('Associated Account'),
    key: 'account_id',
    minWidth: 240,
    resizable: true,
    ellipsis: { tooltip: true },
    render(row: any) {
      if (row.account_id == 0) {
        return $gettext('None')
      }
      return accounts.value?.find((item: any) => item.value === row.account_id)?.label
    },
  },
  {
    title: $gettext('Associated Website'),
    key: 'website_ids',
    minWidth: 200,
    resizable: true,
    render(row: any) {
      if (!row.website_ids?.length) {
        return h('span', { class: 'text-text-tertiary' }, $gettext('None'))
      }
      return h(NFlex, null, {
        default: () =>
          row.website_ids.map((id: number) =>
            h(
              NTag,
              { type: 'success' },
              {
                default: () =>
                  websites.value?.find((item: any) => item.value === id)?.label ?? String(id),
              },
            ),
          ),
      })
    },
  },
  {
    title: $gettext('Issuer'),
    key: 'issuer',
    width: 250,
    ellipsis: { tooltip: true },
    render(row: any) {
      return row.issuer == '' ? $gettext('None') : row.issuer
    },
  },
  {
    title: $gettext('Expiration Time'),
    key: 'not_after',
    width: 200,
    ellipsis: { tooltip: true },
    render(row: any) {
      return formatDateTime(row.not_after)
    },
  },
  {
    title: $gettext('Next Renewal Time'),
    key: 'next_renewal',
    minWidth: 200,
    resizable: true,
    render(row: any) {
      return row.next_renewal == 0 ? $gettext('None') : formatDateTime(row.next_renewal)
    },
  },
  {
    title: $gettext('Auto Renewal'),
    key: 'auto_renewal',
    width: 140,
    resizable: true,
    render(row: any) {
      return h(NSwitch, {
        size: 'small',
        rubberBand: false,
        value: row.auto_renewal,
        loading: switching.has(row.id),
        onUpdateValue: () => handleAutoRenewalUpdate(row),
      })
    },
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 400,
    hideInExcel: true,
    render(row: any) {
      const items: any[] = []
      if (row.type != 'upload' && row.cert == '' && row.key == '') {
        items.push(
          h(
            NButton,
            {
              size: 'small',
              type: 'info',
              onClick: () => {
                obtainMode.value = 'obtain'
                obtainCert.value = row.id
                obtain.value = true
              },
            },
            { default: () => $gettext('Issue') },
          ),
        )
      }
      if (row.cert != '' && row.key != '') {
        items.push(
          h(
            NButton,
            {
              size: 'small',
              type: 'info',
              onClick: () => handleOpenDeploy(row),
            },
            { default: () => $gettext('Deploy') },
          ),
        )
      }
      // 从路径导入的证书没有续签一说，重新读取文件叫刷新
      const fromPath = row.type == 'upload'
      if (fromPath ? row.cert_path != '' : row.cert_url != '') {
        items.push(
          h(
            NButton,
            {
              size: 'small',
              type: 'success',
              onClick: () => {
                obtainMode.value = fromPath ? 'refresh' : 'renew'
                obtainCert.value = row.id
                obtain.value = true
              },
            },
            { default: () => (fromPath ? $gettext('Refresh') : $gettext('Renewal')) },
          ),
        )
      }
      if (row.cert != '' && row.key != '') {
        items.push(
          h(
            NButton,
            {
              size: 'small',
              type: 'tertiary',
              onClick: () => {
                showModel.value.cert = row.cert
                showModel.value.key = row.key
                showModal.value = true
              },
            },
            { default: () => $gettext('View') },
          ),
        )
      }
      items.push(
        h(
          NButton,
          {
            size: 'small',
            type: 'primary',
            onClick: () => {
              editItem.value = row
              editModal.value = true
            },
          },
          { default: () => $gettext('Modify') },
        ),
        h(
          AppButton,
          {
            size: 'small',
            type: 'error',
            onClick: async () => {
              const ok = await confirmDelete({
                content: $gettext('Are you sure you want to delete the certificate?'),
              })
              if (!ok) return
              await cert.certDelete(row.id)
              refresh()
              window.$message.success($gettext('Deletion successful'))
            },
          },
          { default: () => $gettext('Delete') },
        ),
      )
      return h(NFlex, { size: 'small', align: 'center' }, () => items)
    },
  },
]

const { loading, data, page, pageSize, refresh, pagination } = usePagedTable((page, pageSize) =>
  cert.certs(page, pageSize),
)

const switching = reactive(new Set<number>())

const handleAutoRenewalUpdate = (row: any) => {
  switching.add(row.id)
  useRequest(
    cert.certUpdate(row.id, {
      domains: row.domains,
      alias: row.alias,
      type: row.type,
      dns_id: row.dns_id == 0 ? null : row.dns_id,
      account_id: row.account_id == 0 ? null : row.account_id,
      website_ids: row.website_ids || [],
      auto_renewal: !row.auto_renewal,
      cert: row.cert,
      key: row.key,
      cert_path: row.cert_path,
      key_path: row.key_path,
      script: row.script,
    }),
  )
    .onSuccess(() => {
      refresh()
      window.$message.success($gettext('Update successful'))
    })
    .onComplete(() => {
      switching.delete(row.id)
    })
}

const handleDeployCert = async () => {
  await cert.deploy(
    deployModel.value.id,
    deployModel.value.websites,
    deployModel.value.enable_https,
  )
  refresh()
  deployModal.value = false
  deployModel.value.id = null
  deployModel.value.websites = []
  deployModel.value.enable_https = true
  deployDomains.value = []
  window.$message.success($gettext('Deployment successful'))
}

const handleShowModalClose = () => {
  showModel.value.cert = ''
  showModel.value.key = ''
}

useBus('cert:refresh-cert', refresh)
</script>

<template>
  <n-space vertical size="large">
    <n-data-table
      v-model:page="page"
      v-model:pageSize="pageSize"
      striped
      remote
      :scroll-x="2000"
      :loading="loading"
      :columns="columns"
      :data="data"
      :row-key="(row: any) => row.id"
      :pagination="pagination"
    />
  </n-space>
  <cert-modal
    v-model:show="editModal"
    :accounts="accounts"
    :algorithms="algorithms"
    :websites="websites"
    :dns="dns"
    :edit-item="editItem"
  />
  <app-modal v-model:show="deployModal" :title="$gettext('Deploy Certificate')">
    <n-flex vertical>
      <n-form :model="deployModel">
        <n-form-item path="websites" :label="$gettext('Website')">
          <n-flex vertical class="w-full">
            <n-flex justify="end">
              <n-button
                type="primary"
                secondary
                :disabled="deployDomains.length === 0 || websites.length === 0"
                @click="handleMatchWebsites"
              >
                {{ $gettext('Match Websites') }}
              </n-button>
            </n-flex>
            <!-- 穿梭框双栏在手机上每栏不足 170px，改用多选下拉 -->
            <n-select
              v-if="isMobile"
              v-model:value="deployModel.websites"
              :options="deployWebsiteOptions"
              multiple
              filterable
              clearable
            />
            <n-transfer
              v-else
              v-model:value="deployModel.websites"
              :options="deployWebsiteOptions"
              :source-title="$gettext('Available')"
              :target-title="$gettext('Selected')"
              source-filterable
              target-filterable
              virtual-scroll
            />
          </n-flex>
        </n-form-item>
        <n-form-item path="enable_https" :label="$gettext('Enable HTTPS')">
          <n-switch v-model:value="deployModel.enable_https" />
        </n-form-item>
      </n-form>
      <app-button
        type="info"
        block
        :disabled="deployModel.websites.length === 0"
        @click="handleDeployCert"
      >
        {{ $gettext('Submit') }}
      </app-button>
    </n-flex>
  </app-modal>
  <app-modal
    size="lg"
    v-model:show="showModal"
    :title="$gettext('View Certificate')"
    @close="handleShowModalClose"
    @mask-click="handleShowModalClose"
  >
    <n-tabs type="line" animated>
      <n-tab-pane name="cert" :tab="$gettext('Certificate')">
        <common-editor v-model:value="showModel.cert" height="60vh" read-only />
      </n-tab-pane>
      <n-tab-pane name="key" :tab="$gettext('Private Key')">
        <common-editor v-model:value="showModel.key" height="60vh" read-only />
      </n-tab-pane>
    </n-tabs>
  </app-modal>
  <obtain-modal v-model:id="obtainCert" v-model:show="obtain" :mode="obtainMode" />
</template>

<style scoped lang="scss"></style>
