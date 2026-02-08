<script setup lang="ts">
import type { MessageReactive } from 'naive-ui'
import { NButton, NDataTable, NFlex, NPopconfirm, NSpace, NSwitch, NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import cert from '@/api/panel/cert'
import { formatDateTime } from '@/utils'
import ObtainModal from '@/views/cert/ObtainModal.vue'

const { $gettext } = useGettext()

const props = defineProps({
  algorithms: {
    type: Array<any>,
    required: true
  },
  websites: {
    type: Array<any>,
    required: true
  },
  accounts: {
    type: Array<any>,
    required: true
  },
  dns: {
    type: Array<any>,
    required: true
  }
})

const { algorithms, websites, accounts, dns } = toRefs(props)

let messageReactive: MessageReactive | null = null

const updateModel = ref<any>({
  domains: [],
  type: 'P256',
  dns_id: null,
  account_id: null,
  website_id: null,
  auto_renewal: true,
  cert: '',
  key: '',
  script: ''
})
const updateModal = ref(false)
const updateCertLoading = ref(false)
const updateCert = ref<any>()
const showModal = ref(false)
const showModel = ref<any>({
  cert: '',
  key: ''
})
const deployModal = ref(false)
const deployModel = ref<any>({
  id: null,
  websites: []
})
const obtain = ref(false)
const obtainCert = ref(0)

const columns: any = [
  {
    title: $gettext('Domain'),
    key: 'domains',
    minWidth: 200,
    resizable: true,
    render(row: any) {
      if (row.domains == null || row.domains.length == 0) {
        return h(NTag, null, { default: () => $gettext('None') })
      }
      return h(NFlex, null, {
        default: () =>
          row.domains.map((domain: any) =>
            h(
              NTag,
              { type: 'primary' },
              {
                default: () => domain
              }
            )
          )
      })
    }
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
          bordered: false
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
          }
        }
      )
    }
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
    }
  },
  {
    title: $gettext('Issuer'),
    key: 'issuer',
    width: 250,
    ellipsis: { tooltip: true },
    render(row: any) {
      return row.issuer == '' ? $gettext('None') : row.issuer
    }
  },
  {
    title: $gettext('Expiration Time'),
    key: 'not_after',
    width: 200,
    ellipsis: { tooltip: true },
    render(row: any) {
      return formatDateTime(row.not_after)
    }
  },
  {
    title: $gettext('Next Renewal Time'),
    key: 'next_renewal',
    minWidth: 200,
    resizable: true,
    render(row: any) {
      return row.next_renewal == 0 ? $gettext('None') : formatDateTime(row.next_renewal)
    }
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
        onUpdateValue: () => handleAutoRenewalUpdate(row)
      })
    }
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 400,
    hideInExcel: true,
    render(row: any) {
      return [
        row.type != 'upload' && row.cert == '' && row.key == ''
          ? h(
              NButton,
              {
                size: 'small',
                type: 'info',
                style: 'margin-left: 15px;',
                onClick: async () => {
                  obtain.value = true
                  obtainCert.value = row.id
                }
              },
              {
                default: () => $gettext('Issue')
              }
            )
          : null,
        row.cert != '' && row.key != ''
          ? h(
              NButton,
              {
                size: 'small',
                type: 'info',
                onClick: () => {
                  deployModel.value.id = row.id
                  if (row.website_id != 0) {
                    deployModel.value.websites = [row.website_id]
                  }
                  deployModal.value = true
                }
              },
              {
                default: () => $gettext('Deploy')
              }
            )
          : null,
        row.cert_url != '' && row.type != 'upload'
          ? h(
              NButton,
              {
                size: 'small',
                type: 'success',
                style: 'margin-left: 15px;',
                onClick: async () => {
                  messageReactive = window.$message.loading($gettext('Please wait...'), {
                    duration: 0
                  })
                  useRequest(cert.renew(row.id))
                    .onSuccess(() => {
                      refresh()
                      window.$message.success($gettext('Renewal successful'))
                    })
                    .onComplete(() => {
                      messageReactive?.destroy()
                    })
                }
              },
              {
                default: () => $gettext('Renewal')
              }
            )
          : null,
        row.cert != '' && row.key != ''
          ? h(
              NButton,
              {
                size: 'small',
                type: 'tertiary',
                style: 'margin-left: 15px;',
                onClick: () => {
                  showModel.value.cert = row.cert
                  showModel.value.key = row.key
                  showModal.value = true
                }
              },
              {
                default: () => $gettext('View')
              }
            )
          : null,
        h(
          NButton,
          {
            size: 'small',
            type: 'primary',
            style: 'margin-left: 15px;',
            onClick: () => {
              updateCert.value = row.id
              updateModel.value.domains = row.domains
              updateModel.value.type = row.type
              updateModel.value.dns_id = row.dns_id == 0 ? null : row.dns_id
              updateModel.value.account_id = row.account_id == 0 ? null : row.account_id
              updateModel.value.website_id = row.website_id == 0 ? null : row.website_id
              updateModel.value.auto_renewal = row.auto_renewal
              updateModel.value.cert = row.cert
              updateModel.value.key = row.key
              updateModel.value.script = row.script
              updateModal.value = true
            }
          },
          {
            default: () => $gettext('Modify')
          }
        ),
        h(
          NPopconfirm,
          {
            onPositiveClick: async () => {
              useRequest(cert.certDelete(row.id)).onSuccess(() => {
                refresh()
                window.$message.success($gettext('Deletion successful'))
              })
            }
          },
          {
            default: () => {
              return $gettext('Are you sure you want to delete the certificate?')
            },
            trigger: () => {
              return h(
                NButton,
                {
                  size: 'small',
                  type: 'error',
                  style: 'margin-left: 15px;'
                },
                {
                  default: () => $gettext('Delete')
                }
              )
            }
          }
        )
      ]
    }
  }
]

const { loading, data, page, total, pageSize, pageCount, refresh } = usePagination(
  (page, pageSize) => cert.certs(page, pageSize),
  {
    initialData: { total: 0, list: [] },
    initialPageSize: 20,
    total: (res: any) => res.total,
    data: (res: any) => res.items
  }
)

const handleUpdateCert = () => {
  updateCertLoading.value = true
  useRequest(cert.certUpdate(updateCert.value, updateModel.value))
    .onSuccess(() => {
      refresh()
      updateModal.value = false
      updateModel.value.domains = []
      updateModel.value.type = 'P256'
      updateModel.value.dns_id = null
      updateModel.value.account_id = null
      updateModel.value.website_id = null
      updateModel.value.auto_renewal = true
      updateModel.value.cert = ''
      updateModel.value.key = ''
      updateModel.value.script = ''
      window.$message.success($gettext('Update successful'))
    })
    .onComplete(() => {
      updateCertLoading.value = false
    })
}

const handleAutoRenewalUpdate = (row: any) => {
  updateModel.value.domains = row.domains
  updateModel.value.type = row.type
  updateModel.value.dns_id = row.dns_id == 0 ? null : row.dns_id
  updateModel.value.account_id = row.account_id == 0 ? null : row.account_id
  updateModel.value.website_id = row.website_id == 0 ? null : row.website_id
  updateModel.value.auto_renewal = !row.auto_renewal
  updateModel.value.cert = row.cert
  updateModel.value.key = row.key
  updateModel.value.script = row.script
  useRequest(cert.certUpdate(row.id, updateModel.value))
    .onSuccess(() => {
      refresh()
      window.$message.success($gettext('Update successful'))
    })
    .onComplete(() => {
      updateModel.value.domains = []
      updateModel.value.type = 'P256'
      updateModel.value.dns_id = null
      updateModel.value.account_id = null
      updateModel.value.website_id = null
      updateModel.value.auto_renewal = true
      updateModel.value.cert = ''
      updateModel.value.key = ''
      updateModel.value.script = ''
    })
}

const handleDeployCert = async () => {
  const promises = deployModel.value.websites.map((website: any) =>
    cert.deploy(deployModel.value.id, website)
  )
  await Promise.all(promises)

  deployModal.value = false
  deployModel.value.id = null
  deployModel.value.websites = []
  window.$message.success($gettext('Deployment successful'))
}

const handleShowModalClose = () => {
  showModel.value.cert = ''
  showModel.value.key = ''
}

onMounted(() => {
  refresh()
  window.$bus.on('cert:refresh-cert', () => {
    refresh()
  })
})

onUnmounted(() => {
  window.$bus.off('cert:refresh-cert')
})
</script>

<template>
  <n-space vertical size="large">
    <n-data-table
      striped
      remote
      :scroll-x="1600"
      :loading="loading"
      :columns="columns"
      :data="data"
      :row-key="(row: any) => row.id"
      v-model:page="page"
      v-model:pageSize="pageSize"
      :pagination="{
        page: page,
        pageCount: pageCount,
        pageSize: pageSize,
        itemCount: total,
        showQuickJumper: true,
        showSizePicker: true,
        pageSizes: [20, 50, 100, 200]
      }"
    />
  </n-space>
  <n-modal
    v-model:show="updateModal"
    preset="card"
    :title="$gettext('Modify Certificate')"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
  >
    <n-space vertical>
      <n-alert v-if="updateModel.type != 'upload'" type="info">
        {{
          $gettext(
            'You can automatically issue and deploy certificates by selecting any website/DNS, or manually enter domain names and set DNS resolution to issue certificates, or fill in deployment scripts to automatically deploy certificates.'
          )
        }}
      </n-alert>
      <n-form :model="updateModel">
        <n-form-item v-if="updateModel.type != 'upload'" path="domains" :label="$gettext('Domain')">
          <n-dynamic-input
            v-model:value="updateModel.domains"
            placeholder="example.com"
            :min="1"
            show-sort-button
          />
        </n-form-item>
        <n-form-item v-if="updateModel.type != 'upload'" path="type" :label="$gettext('Key Type')">
          <n-select
            v-model:value="updateModel.type"
            :placeholder="$gettext('Select key type')"
            clearable
            :options="algorithms"
          />
        </n-form-item>
        <n-form-item path="website_id" :label="$gettext('Website')">
          <n-select
            v-model:value="updateModel.website_id"
            :placeholder="$gettext('Select website for certificate deployment')"
            clearable
            :options="websites"
          />
        </n-form-item>
        <n-form-item
          v-if="updateModel.type != 'upload'"
          path="account_id"
          :label="$gettext('Account')"
        >
          <n-select
            v-model:value="updateModel.account_id"
            :placeholder="$gettext('Select account for certificate issuance')"
            clearable
            :options="accounts"
          />
        </n-form-item>
        <n-form-item v-if="updateModel.type != 'upload'" path="account_id" :label="$gettext('DNS')">
          <n-select
            v-model:value="updateModel.dns_id"
            :placeholder="$gettext('Select DNS for certificate issuance')"
            clearable
            :options="dns"
          />
        </n-form-item>
        <n-form-item
          v-if="updateModel.type == 'upload'"
          path="cert"
          :label="$gettext('Certificate')"
        >
          <n-input
            v-model:value="updateModel.cert"
            type="textarea"
            :placeholder="$gettext('Enter the content of the PEM certificate file')"
            :autosize="{ minRows: 10, maxRows: 15 }"
          />
        </n-form-item>
        <n-form-item
          v-if="updateModel.type == 'upload'"
          path="key"
          :label="$gettext('Private Key')"
        >
          <n-input
            v-model:value="updateModel.key"
            type="textarea"
            :placeholder="$gettext('Enter the content of the KEY private key file')"
            :autosize="{ minRows: 10, maxRows: 15 }"
          />
        </n-form-item>
        <n-form-item
          v-if="updateModel.type != 'upload'"
          path="key"
          :label="$gettext('Deployment Script')"
        >
          <n-input
            v-model:value="updateModel.script"
            type="textarea"
            :placeholder="
              $gettext(
                'The {cert} and {key} in the script will be replaced with the certificate and private key content'
              )
            "
            :autosize="{ minRows: 5, maxRows: 10 }"
          />
        </n-form-item>
      </n-form>
      <n-button type="info" block :loading="updateCertLoading" :disabled="updateCertLoading" @click="handleUpdateCert">{{ $gettext('Submit') }}</n-button>
    </n-space>
  </n-modal>
  <n-modal
    v-model:show="deployModal"
    preset="card"
    :title="$gettext('Deploy Certificate')"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
  >
    <n-flex vertical>
      <n-alert type="info">
        {{ $gettext('If website not enabled HTTPS, please enable it after deployment.') }}
      </n-alert>
      <n-form :model="deployModel">
        <n-form-item path="website_id" :label="$gettext('Website')">
          <n-select
            v-model:value="deployModel.websites"
            :placeholder="$gettext('Select websites to deploy the certificate')"
            clearable
            multiple
            :options="websites"
          />
        </n-form-item>
      </n-form>
      <n-button type="info" block @click="handleDeployCert">{{ $gettext('Submit') }}</n-button>
    </n-flex>
  </n-modal>
  <n-modal
    v-model:show="showModal"
    preset="card"
    :title="$gettext('View Certificate')"
    style="width: 80vw"
    size="huge"
    :bordered="false"
    :segmented="false"
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
  </n-modal>
  <obtain-modal v-model:id="obtainCert" v-model:show="obtain" />
</template>

<style scoped lang="scss"></style>
