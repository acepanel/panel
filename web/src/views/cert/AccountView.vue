<script setup lang="ts">
import {
  type MessageReactive,
  NButton,
  NDataTable,
  NInput,
  NPopconfirm,
  NSpace,
  NTag
} from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import cert from '@/api/panel/cert'

const { $gettext } = useGettext()

const props = defineProps({
  caProviders: {
    type: Array<any>,
    required: true
  },
  algorithms: {
    type: Array<any>,
    required: true
  }
})

const { caProviders, algorithms } = toRefs(props)

let messageReactive: MessageReactive | null = null

const updateAccountModel = ref<any>({
  hmac_encoded: '',
  email: '',
  kid: '',
  key_type: 'P256',
  ca: 'letsencrypt'
})
const updateAccountModal = ref(false)
const updateAccountLoading = ref(false)
const updateAccount = ref<any>()

const columns: any = [
  {
    title: $gettext('Email'),
    key: 'email',
    minWidth: 200,
    resizable: true,
    ellipsis: { tooltip: true }
  },
  {
    title: 'CA',
    key: 'ca',
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
            return caProviders.value?.find((item: any) => item.value === row.ca)?.label
          }
        }
      )
    }
  },
  {
    title: $gettext('Key Type'),
    key: 'key_type',
    width: 150,
    resizable: true,
    ellipsis: { tooltip: true }
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
              updateAccount.value = row.id
              updateAccountModel.value.email = row.email
              updateAccountModel.value.hmac_encoded = row.hmac_encoded
              updateAccountModel.value.kid = row.kid
              updateAccountModel.value.key_type = row.key_type
              updateAccountModel.value.ca = row.ca
              updateAccountModal.value = true
            }
          },
          {
            default: () => $gettext('Modify')
          }
        ),
        h(
          NPopconfirm,
          {
            onPositiveClick: () => {
              useRequest(cert.accountDelete(row.id)).onSuccess(() => {
                window.$message.success($gettext('Deletion successful'))
                refresh()
              })
            }
          },
          {
            default: () => {
              return $gettext('Are you sure you want to delete the account?')
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
  (page, pageSize) => cert.accounts(page, pageSize),
  {
    initialData: { total: 0, list: [] },
    initialPageSize: 20,
    total: (res: any) => res.total,
    data: (res: any) => res.items
  }
)

const handleUpdateAccount = () => {
  updateAccountLoading.value = true
  messageReactive = window.$message.loading(
    $gettext('Registering account with CA, please wait patiently'),
    {
      duration: 0
    }
  )
  useRequest(cert.accountUpdate(updateAccount.value, updateAccountModel.value))
    .onSuccess(() => {
      refresh()
      updateAccountModal.value = false
      updateAccountModel.value.email = ''
      updateAccountModel.value.hmac_encoded = ''
      updateAccountModel.value.kid = ''
      window.$message.success($gettext('Update successful'))
    })
    .onComplete(() => {
      messageReactive?.destroy()
      updateAccountLoading.value = false
    })
}

onMounted(() => {
  refresh()
  window.$bus.on('cert:refresh-account', () => {
    refresh()
  })
})

onUnmounted(() => {
  window.$bus.off('cert:refresh-account')
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
    v-model:show="updateAccountModal"
    preset="card"
    :title="$gettext('Modify Account')"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
  >
    <n-space vertical>
      <n-alert type="info">{{
        $gettext(
          'LiteSSL, Google and SSL.com require obtaining EAB (KID and HMAC) from their official websites first'
        )
      }}</n-alert>
      <n-alert type="warning">
        {{
          $gettext(
            "Google is not accessible in mainland China, other CAs depend on network conditions, recommend using Let's Encrypt or LiteSSL"
          )
        }}
      </n-alert>
      <n-form :model="updateAccountModel">
        <n-form-item path="ca" :label="$gettext('CA')">
          <n-select
            v-model:value="updateAccountModel.ca"
            :placeholder="$gettext('Select CA')"
            clearable
            :options="caProviders"
          />
        </n-form-item>
        <n-form-item path="key_type" :label="$gettext('Key Type')">
          <n-select
            v-model:value="updateAccountModel.key_type"
            :placeholder="$gettext('Select key type')"
            clearable
            :options="algorithms"
          />
        </n-form-item>
        <n-form-item path="email" :label="$gettext('Email')">
          <n-input
            v-model:value="updateAccountModel.email"
            type="text"
            @keydown.enter.prevent
            :placeholder="$gettext('Enter email address')"
          />
        </n-form-item>
        <n-form-item path="kid" label="KID">
          <n-input
            v-model:value="updateAccountModel.kid"
            type="text"
            @keydown.enter.prevent
            :placeholder="$gettext('Enter KID')"
          />
        </n-form-item>
        <n-form-item path="hmac_encoded" label="HMAC">
          <n-input
            v-model:value="updateAccountModel.hmac_encoded"
            type="text"
            @keydown.enter.prevent
            :placeholder="$gettext('Enter HMAC')"
          />
        </n-form-item>
      </n-form>
      <n-button type="info" block :loading="updateAccountLoading" :disabled="updateAccountLoading" @click="handleUpdateAccount">{{ $gettext('Submit') }}</n-button>
    </n-space>
  </n-modal>
</template>

<style scoped lang="scss"></style>
