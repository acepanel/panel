<script setup lang="ts">
import { NButton, NFlex, NTag, NTooltip } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import waf, {
  policyApplyState,
  type WafPolicy,
  type WafPolicyApplyState,
  type WafPolicyStatus,
} from '@/api/panel/waf'
import { useConfirm } from '@/components/system/composables/useConfirm'
import PolicyEditor from '@/views/waf/PolicyEditor.vue'

const { $gettext } = useGettext()
const { confirmDelete } = useConfirm()

const loading = ref(false)
const policies = ref<WafPolicy[]>([])
let requestPending = false
let statusRequestPending = false
let pollTimer: ReturnType<typeof setTimeout> | undefined
let active = true

const editorShow = ref(false)
const editPolicyId = ref<number>(0)

const stopPolling = () => {
  if (pollTimer) clearTimeout(pollTimer)
  pollTimer = undefined
}

const schedulePolling = () => {
  stopPolling()
  if (policies.value.some((policy) => policyApplyState(policy) === 'pending')) {
    pollTimer = setTimeout(pollPendingPolicies, 3000)
  }
}

const pollPendingPolicies = () => {
  if (statusRequestPending) return
  const pending = policies.value.filter((policy) => policyApplyState(policy) === 'pending')
  if (pending.length === 0) return

  statusRequestPending = true
  let remaining = pending.length
  for (const policy of pending) {
    useRequest(waf.policyStatus(policy.id))
      .onSuccess(({ data }: { data: WafPolicyStatus }) => {
        Object.assign(policy, data)
      })
      .onComplete(() => {
        remaining--
        if (remaining === 0) {
          statusRequestPending = false
          if (active) schedulePolling()
        }
      })
  }
}

const loadPolicies = (showLoading = true) => {
  if (requestPending) return
  stopPolling()
  requestPending = true
  if (showLoading) loading.value = true
  useRequest(waf.policies())
    .onSuccess(({ data }: any) => {
      // agent 直接返回数组
      policies.value = Array.isArray(data) ? data : data?.items || []
    })
    .onComplete(() => {
      requestPending = false
      if (showLoading) loading.value = false
      schedulePolling()
    })
}

const handleSaved = (policy: WafPolicy) => {
  const index = policies.value.findIndex((item) => item.id === policy.id)
  if (index === -1) {
    policies.value.unshift(policy)
  } else {
    policies.value[index] = policy
  }
  schedulePolling()
}

const applyStatusMeta = (
  state: WafPolicyApplyState,
): { type: 'default' | 'error' | 'success' | 'warning'; label: string } => {
  switch (state) {
    case 'pending':
      return { type: 'warning', label: $gettext('Pending') }
    case 'applied':
      return { type: 'success', label: $gettext('Applied') }
    case 'failed':
      return { type: 'error', label: $gettext('Failed') }
    default:
      return { type: 'default', label: $gettext('Saved') }
  }
}

const renderApplyStatus = (row: WafPolicy) => {
  const state = policyApplyState(row)
  const meta = applyStatusMeta(state)
  const tag = () => h(NTag, { type: meta.type }, { default: () => meta.label })
  if (state !== 'failed' || !row.last_error) return tag()
  return h(NTooltip, null, {
    trigger: tag,
    default: () => String(row.last_error),
  })
}

const columns: any = [
  { title: 'ID', key: 'id', width: 80 },
  {
    title: $gettext('Name'),
    key: 'name',
    minWidth: 150,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Enabled'),
    key: 'enabled',
    width: 110,
    render(row: any) {
      return h(
        NTag,
        { type: row.enabled ? 'success' : 'default' },
        { default: () => (row.enabled ? $gettext('Enabled') : $gettext('Disabled')) },
      )
    },
  },
  {
    title: $gettext('Mode'),
    key: 'mode',
    width: 120,
    render(row: any) {
      return h(
        NTag,
        { type: row.mode === 'observe' ? 'warning' : 'error' },
        { default: () => (row.mode === 'observe' ? $gettext('Observe') : $gettext('Block')) },
      )
    },
  },
  {
    title: $gettext('Security Level'),
    key: 'security_level',
    width: 120,
    render(row: any) {
      return h(
        NTag,
        { type: row.security_level === 'strict' ? 'warning' : 'info' },
        {
          default: () =>
            row.security_level === 'strict' ? $gettext('Strict') : $gettext('Standard'),
        },
      )
    },
  },
  {
    title: $gettext('Version'),
    key: 'version',
    width: 90,
  },
  {
    title: $gettext('Apply Status'),
    key: 'apply_status',
    width: 120,
    render: renderApplyStatus,
  },
  {
    title: $gettext('Remark'),
    key: 'remark',
    minWidth: 120,
    ellipsis: { tooltip: true },
    render: (row: any) => row.remark || '-',
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 160,
    align: 'center',
    render(row: any) {
      return h(NFlex, { justify: 'center' }, () => [
        h(
          NButton,
          {
            size: 'small',
            type: 'primary',
            onClick: () => {
              editPolicyId.value = row.id
              editorShow.value = true
            },
          },
          { default: () => $gettext('Edit') },
        ),
        h(
          NButton,
          {
            size: 'small',
            type: 'error',
            onClick: async () => {
              const ok = await confirmDelete({
                content: $gettext('Are you sure you want to delete this policy?'),
                countdown: 5,
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

const handleDelete = (id: number) => {
  useRequest(waf.deletePolicy(id)).onSuccess(() => {
    loadPolicies()
    window.$message.success($gettext('Deleted successfully'))
  })
}

const handleCreate = () => {
  editPolicyId.value = 0
  editorShow.value = true
}

onMounted(() => {
  loadPolicies()
})

onBeforeUnmount(() => {
  active = false
  stopPolling()
})
</script>

<template>
  <n-flex vertical :size="20">
    <n-flex items-center>
      <n-button type="primary" @click="handleCreate">
        {{ $gettext('Create Policy') }}
      </n-button>
      <n-button @click="loadPolicies()">{{ $gettext('Refresh') }}</n-button>
    </n-flex>
    <n-data-table
      striped
      :scroll-x="1120"
      :loading="loading"
      :columns="columns"
      :data="policies"
      :row-key="(row: any) => row.id"
    />
  </n-flex>
  <policy-editor v-model:show="editorShow" :policy-id="editPolicyId" @saved="handleSaved" />
</template>

<style scoped lang="scss"></style>
