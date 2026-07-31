<script setup lang="ts">
import { NButton, NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import waf from '@/api/panel/waf'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { formatDateTime } from '@/utils'
import BindingModal from '@/views/waf/BindingModal.vue'

const { $gettext } = useGettext()
const { confirmAction } = useConfirm()

const loading = ref(false)
const bindings = ref<any[]>([])
const modalShow = ref(false)

const loadBindings = () => {
  loading.value = true
  useRequest(waf.bindings())
    .onSuccess(({ data }: any) => {
      bindings.value = Array.isArray(data) ? data : data?.items || []
    })
    .onComplete(() => {
      loading.value = false
    })
}

watch(modalShow, (v) => {
  if (!v) loadBindings()
})

const columns: any = [
  {
    title: $gettext('Website'),
    key: 'website_name',
    minWidth: 160,
    ellipsis: { tooltip: true },
    render: (row: any) => row.website_name || `#${row.website_id}`,
  },
  {
    title: $gettext('Policy ID'),
    key: 'policy_id',
    width: 120,
  },
  {
    title: $gettext('Status'),
    key: 'enabled',
    width: 110,
    render(row: any) {
      return h(
        NTag,
        { type: row.enabled ? 'success' : 'default' },
        { default: () => (row.enabled ? $gettext('Enabled') : $gettext('Disabled')) }
      )
    },
  },
  {
    title: $gettext('Created At'),
    key: 'created_at',
    width: 180,
    render: (row: any) => formatDateTime(row.created_at),
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 130,
    align: 'center',
    render(row: any) {
      return h(
        NButton,
        {
          size: 'small',
          type: 'error',
          onClick: async () => {
            const ok = await confirmAction({
              type: 'warning',
              title: $gettext('Disable WAF'),
              content: $gettext('Are you sure you want to disable WAF for this website?'),
            })
            if (ok) handleDisable(row.website_id)
          },
        },
        { default: () => $gettext('Disable') }
      )
    },
  },
]

const handleDisable = (websiteId: number) => {
  useRequest(waf.disableWebsite(websiteId)).onSuccess(() => {
    loadBindings()
    window.$message.success($gettext('Disabled successfully'))
  })
}

onMounted(() => {
  loadBindings()
})
</script>

<template>
  <n-flex vertical :size="20">
    <n-flex items-center>
      <n-button type="primary" @click="modalShow = true">
        {{ $gettext('Enable WAF for Website') }}
      </n-button>
      <n-button @click="loadBindings">{{ $gettext('Refresh') }}</n-button>
    </n-flex>
    <n-data-table
      striped
      :scroll-x="800"
      :loading="loading"
      :columns="columns"
      :data="bindings"
      :row-key="(row: any) => row.id"
    />
  </n-flex>
  <binding-modal v-model:show="modalShow" />
</template>

<style scoped lang="scss"></style>
