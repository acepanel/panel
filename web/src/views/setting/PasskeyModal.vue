<script setup lang="ts">
import { startRegistration } from '@simplewebauthn/browser'
import { NButton, NDataTable } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import user from '@/api/panel/user'
import AppButton from '@/components/system/AppButton.vue'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { useUserStore } from '@/stores'
import { formatDateTime } from '@/utils'

const { $gettext } = useGettext()
const { confirmDelete } = useConfirm()
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
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Creation Time'),
    key: 'created_at',
    minWidth: 200,
    ellipsis: { tooltip: true },
    render(row: any) {
      return formatDateTime(row.created_at)
    },
  },
  {
    title: $gettext('Last Used'),
    key: 'last_used_at',
    minWidth: 200,
    ellipsis: { tooltip: true },
    render(row: any) {
      return row.last_used_at ? formatDateTime(row.last_used_at) : $gettext('Never')
    },
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 150,
    hideInExcel: true,
    render(row: any) {
      return h(
        AppButton,
        {
          size: 'small',
          type: 'error',
          onClick: async () => {
            const ok = await confirmDelete({
              content: $gettext('Are you sure you want to delete this passkey?'),
            })
            if (ok) await handleDelete(row.id)
          },
        },
        { default: () => $gettext('Delete') },
      )
    },
  },
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

const handleDelete = async (passkeyId: number) => {
  await user.passkeyDelete(passkeyId, id.value)
  window.$message.success($gettext('Deleted successfully'))
  refresh()
}

const handleRegister = async () => {
  // 输入框回车也会触发，按钮禁用挡不住
  if (registerLoading.value) return
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
  { immediate: true },
)
</script>

<template>
  <app-modal v-model:show="show" :title="$gettext('Passkeys')">
    <n-flex vertical>
      <n-alert v-if="!passkeySupported" type="info" :bordered="false">
        {{
          $gettext(
            'Passkeys are only available when using a bound domain with a trusted HTTPS certificate.',
          )
        }}
      </n-alert>
      <template v-else-if="isSelf">
        <n-flex align="center">
          <n-input
            v-model:value="passkeyName"
            :placeholder="$gettext('Passkey name')"
            class="md:max-w-75"
            :disabled="registerLoading"
            :loading="registerLoading || undefined"
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
        :scroll-x="700"
        :loading="loading"
        :columns="columns"
        :data="data"
        :row-key="(row: any) => row.id"
      />
    </n-flex>
  </app-modal>
</template>

<style scoped lang="scss"></style>
