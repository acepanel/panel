<script setup lang="ts">
import { NButton, NDataTable, NInput, NPopconfirm, NSpace, NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import cert from '@/api/panel/cert'

const { $gettext } = useGettext()

const props = defineProps({
  dnsProviders: {
    type: Array<any>,
    required: true
  }
})

const { dnsProviders } = toRefs(props)

const updateDNSModel = ref<any>({
  data: {
    ak: '',
    sk: ''
  },
  type: 'aliyun',
  name: ''
})
const updateDNSModal = ref(false)
const updateDNSLoading = ref(false)
const updateDNS = ref<any>()

const columns: any = [
  {
    title: $gettext('Note Name'),
    key: 'name',
    minWidth: 200,
    resizable: true,
    ellipsis: { tooltip: true }
  },
  {
    title: $gettext('Type'),
    key: 'type',
    width: 150,
    resizable: true,
    ellipsis: { tooltip: true },
    render(row: any) {
      return h(
        NTag,
        {
          type: 'info',
          bordered: false
        },
        {
          default: () => {
            const provider = dnsProviders.value.find((provider: any) => provider.value === row.type)
            if (provider) {
              return provider.label
            } else {
              return $gettext('Unknown')
            }
          }
        }
      )
    }
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 200,
    hideInExcel: true,
    render(row: any) {
      return [
        h(
          NButton,
          {
            size: 'small',
            type: 'primary',
            onClick: () => {
              updateDNS.value = row.id
              updateDNSModel.value.data.ak = row.dns_param.ak
              updateDNSModel.value.data.sk = row.dns_param.sk
              updateDNSModel.value.type = row.type
              updateDNSModel.value.name = row.name
              updateDNSModal.value = true
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
              useRequest(cert.dnsDelete(row.id)).onSuccess(() => {
                refresh()
                window.$message.success($gettext('Deletion successful'))
              })
            }
          },
          {
            default: () => {
              return $gettext('Are you sure you want to delete the DNS?')
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
  (page, pageSize) => cert.dns(page, pageSize),
  {
    initialData: { total: 0, list: [] },
    initialPageSize: 20,
    total: (res: any) => res.total,
    data: (res: any) => res.items
  }
)

const handleUpdateDNS = () => {
  updateDNSLoading.value = true
  useRequest(cert.dnsUpdate(updateDNS.value, updateDNSModel.value))
    .onSuccess(() => {
      refresh()
      updateDNSModal.value = false
      updateDNSModel.value.data.ak = ''
      updateDNSModel.value.data.sk = ''
      updateDNSModel.value.name = ''
      window.$message.success($gettext('Update successful'))
    })
    .onComplete(() => {
      updateDNSLoading.value = false
    })
}

onMounted(() => {
  refresh()
  window.$bus.on('cert:refresh-dns', () => {
    refresh()
  })
})

onUnmounted(() => {
  window.$bus.off('cert:refresh-dns')
})
</script>

<template>
  <n-space vertical size="large">
    <n-data-table
      striped
      remote
      :scroll-x="1000"
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
    v-model:show="updateDNSModal"
    preset="card"
    :title="$gettext('Modify DNS')"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
  >
    <n-space vertical>
      <n-form :model="updateDNSModel">
        <n-form-item path="name" :label="$gettext('Note Name')">
          <n-input
            v-model:value="updateDNSModel.name"
            type="text"
            :placeholder="$gettext('Enter note name')"
          />
        </n-form-item>
        <n-form-item path="type" :label="$gettext('DNS')">
          <n-select
            v-model:value="updateDNSModel.type"
            :placeholder="$gettext('Select DNS')"
            clearable
            :options="dnsProviders"
          />
        </n-form-item>
        <n-form-item v-if="updateDNSModel.type == 'aliyun'" path="ak" label="Access Key">
          <n-input
            v-model:value="updateDNSModel.data.ak"
            type="text"
            :placeholder="$gettext('Enter Aliyun Access Key')"
          />
        </n-form-item>
        <n-form-item v-if="updateDNSModel.type == 'aliyun'" path="sk" label="Secret Key">
          <n-input
            v-model:value="updateDNSModel.data.sk"
            type="text"
            :placeholder="$gettext('Enter Aliyun Secret Key')"
          />
        </n-form-item>
        <n-form-item v-if="updateDNSModel.type == 'tencent'" path="ak" label="SecretId">
          <n-input
            v-model:value="updateDNSModel.data.ak"
            type="text"
            :placeholder="$gettext('Enter Tencent Cloud SecretId')"
          />
        </n-form-item>
        <n-form-item v-if="updateDNSModel.type == 'tencent'" path="sk" label="SecretKey">
          <n-input
            v-model:value="updateDNSModel.data.sk"
            type="text"
            :placeholder="$gettext('Enter Tencent Cloud SecretKey')"
          />
        </n-form-item>
        <n-form-item v-if="updateDNSModel.type == 'huawei'" path="ak" label="AccessKeyId">
          <n-input
            v-model:value="updateDNSModel.data.ak"
            type="text"
            :placeholder="$gettext('Enter Huawei Cloud AccessKeyId')"
          />
        </n-form-item>
        <n-form-item v-if="updateDNSModel.type == 'huawei'" path="sk" label="SecretAccessKey">
          <n-input
            v-model:value="updateDNSModel.data.sk"
            type="text"
            :placeholder="$gettext('Enter Huawei Cloud SecretAccessKey')"
          />
        </n-form-item>
        <n-form-item v-if="updateDNSModel.type == 'westcn'" path="sk" label="Username">
          <n-input
            v-model:value="updateDNSModel.data.sk"
            type="text"
            :placeholder="$gettext('Enter West.cn Username')"
          />
        </n-form-item>
        <n-form-item v-if="updateDNSModel.type == 'westcn'" path="ak" label="API Password">
          <n-input
            v-model:value="updateDNSModel.data.ak"
            type="text"
            :placeholder="$gettext('Enter West.cn API Password')"
          />
        </n-form-item>
        <n-form-item v-if="updateDNSModel.type == 'cloudflare'" path="ak" label="API Key">
          <n-input
            v-model:value="updateDNSModel.data.ak"
            type="text"
            :placeholder="$gettext('Enter Cloudflare API Key')"
          />
        </n-form-item>
        <n-form-item v-if="updateDNSModel.type == 'gcore'" path="ak" label="API Key">
          <n-input
            v-model:value="updateDNSModel.data.ak"
            type="text"
            :placeholder="$gettext('Enter G-Core API Key')"
          />
        </n-form-item>
        <n-form-item v-if="updateDNSModel.type == 'porkbun'" path="ak" label="API Key">
          <n-input
            v-model:value="updateDNSModel.data.ak"
            type="text"
            :placeholder="$gettext('Enter Porkbun API Key')"
          />
        </n-form-item>
        <n-form-item v-if="updateDNSModel.type == 'porkbun'" path="sk" label="Secret Key">
          <n-input
            v-model:value="updateDNSModel.data.sk"
            type="text"
            :placeholder="$gettext('Enter Porkbun Secret Key')"
          />
        </n-form-item>
        <n-form-item v-if="updateDNSModel.type == 'namesilo'" path="ak" label="API Token">
          <n-input
            v-model:value="updateDNSModel.data.ak"
            type="text"
            :placeholder="$gettext('Enter NameSilo API Token')"
          />
        </n-form-item>
        <n-form-item v-if="updateDNSModel.type == 'cloudns'" path="ak" label="Auth ID">
          <n-input
            v-model:value="updateDNSModel.data.ak"
            type="text"
            :placeholder="$gettext('Enter ClouDNS Auth ID (use Sub Auth ID by adding sub-prefix)')"
          />
        </n-form-item>
        <n-form-item v-if="updateDNSModel.type == 'cloudns'" path="sk" label="Auth Password">
          <n-input
            v-model:value="updateDNSModel.data.sk"
            type="text"
            :placeholder="$gettext('Enter ClouDNS Auth Password')"
          />
        </n-form-item>
      </n-form>
      <n-button type="info" block :loading="updateDNSLoading" :disabled="updateDNSLoading" @click="handleUpdateDNS">{{ $gettext('Submit') }}</n-button>
    </n-space>
  </n-modal>
</template>

<style scoped lang="scss"></style>
