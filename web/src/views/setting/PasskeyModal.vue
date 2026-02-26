<script setup lang="ts">
import user from '@/api/panel/user'
import { useUserStore } from '@/store'
import { formatDateTime } from '@/utils'
import { startRegistration } from '@simplewebauthn/browser'
import { NAlert, NButton, NDataTable, NFlex, NInput, NPopconfirm } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

const { $gettext } = useGettext()
const show = defineModel<boolean>('show', { type: Boolean, required: true })
const id = defineModel<number>('id', { type: Number, required: true })
const userStore = useUserStore()

const registerLoading = ref(false)
const passkeyName = ref('')
const passkeySupported = ref(false)

// 只有当前登录用户自己才能注册通行密钥
const isSelf = computed(() => String(id.value) === String(userStore.id))

const columns: any = [
  {
    title: $gettext('Name'),
    key: 'name',
    minWidth: 150,
    resizable: true,
    ellipsis: { tooltip: true }
  },
  {
    title: $gettext('Creation Time'),
    key: 'created_at',
    minWidth: 200,
    ellipsis: { tooltip: true },
    render(row: any) {
      return formatDateTime(row.created_at)
    }
  },
  {
    title: $gettext('Last Used'),
    key: 'last_used_at',
    minWidth: 200,
    ellipsis: { tooltip: true },
    render(row: any) {
      return row.last_used_at ? formatDateTime(row.last_used_at) : $gettext('Never')
    }
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 150,
    hideInExcel: true,
    render(row: any) {
      return h(
        NPopconfirm,
        {
          onPositiveClick: () => handleDelete(row.id)
        },
        {
          default: () => {
            return $gettext('Are you sure you want to delete this passkey?')
          },
          trigger: () => {
            return h(
              NButton,
              {
                size: 'small',
                type: 'error'
              },
              {
                default: () => $gettext('Delete')
              }
            )
          }
        }
      )
    }
  }
]

const loading = ref(false)
const data = ref<any[]>([])

const refresh = () => {
  loading.value = true
  useRequest(user.passkeyList(id.value))
    .onSuccess(({ data: res }) => {
      data.value = res.items || []
    })
    .onComplete(() => {
      loading.value = false
    })
}

const checkSupported = () => {
  useRequest(user.passkeySupported()).onSuccess(({ data: res }) => {
    passkeySupported.value = Boolean(res)
  })
}

const handleDelete = (passkeyId: number) => {
  useRequest(() => user.passkeyDelete(passkeyId, id.value)).onSuccess(() => {
    window.$message.success($gettext('Deleted successfully'))
    refresh()
  })
}

const handleRegister = async () => {
  const name = passkeyName.value.trim()
  if (!name) {
    window.$message.warning($gettext('Please enter a name for the passkey'))
    return
  }

  registerLoading.value = true
  try {
    // 开始注册
    const options = await user.passkeyBeginRegister(name)

    // 调用浏览器 WebAuthn API
    const credential = await startRegistration({ optionsJSON: options.publicKey })

    // 完成注册
    useRequest(user.passkeyFinishRegister(credential, name))
      .onSuccess(() => {
        window.$message.success($gettext('Passkey registered successfully'))
        passkeyName.value = ''
        refresh()
      })
      .onComplete(() => {
        registerLoading.value = false
      })
  } catch (e: any) {
    registerLoading.value = false
    if (e.name === 'NotAllowedError') {
      window.$message.warning($gettext('Registration was cancelled'))
    } else {
      window.$message.error($gettext('Passkey registration failed: %{msg}', { msg: e.message }))
    }
  }
}

watch(
  () => show.value,
  (val) => {
    if (val) {
      checkSupported()
      refresh()
    }
  },
  { immediate: true }
)
</script>

<template>
  <n-modal
    v-model:show="show"
    preset="card"
    :title="$gettext('Passkeys')"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
    @close="show = false"
  >
    <n-flex vertical>
      <n-alert v-if="!passkeySupported" type="info" :bordered="false">
        {{
          $gettext(
            'Passkeys are only available when using a bound domain with a trusted HTTPS certificate.'
          )
        }}
      </n-alert>
      <template v-else-if="isSelf">
        <n-flex align="center">
          <n-input
            v-model:value="passkeyName"
            :placeholder="$gettext('Passkey name')"
            style="max-width: 300px"
            @keydown.enter="handleRegister"
          />
          <n-button
            type="primary"
            :loading="registerLoading"
            :disabled="registerLoading"
            @click="handleRegister"
          >
            {{ $gettext('Register New Passkey') }}
          </n-button>
        </n-flex>
      </template>
      <n-data-table
        striped
        :scroll-x="600"
        :loading="loading"
        :columns="columns"
        :data="data"
        :row-key="(row: any) => row.id"
      />
    </n-flex>
  </n-modal>
</template>

<style scoped lang="scss"></style>
