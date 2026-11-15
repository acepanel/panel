<script setup lang="ts">
import copy2clipboard from '@vavt/copy2clipboard'
import type { DataTableColumns } from 'naive-ui'
import { NButton, NFlex, NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import file from '@/api/panel/file'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { useFormLayout } from '@/components/system/composables/useFormLayout'
import { formatDateTime } from '@/utils'

const { $gettext } = useGettext()
const { confirmAction } = useConfirm()
const formLayout = useFormLayout('auto', true)

const show = defineModel<boolean>('show', { type: Boolean, required: true })
const path = defineModel<string>('path', { type: String, required: true })

const emit = defineEmits<{
  refresh: []
}>()

const shares = ref<any[]>([])
const loading = ref(false)

const createModel = ref({
  expireHours: 24,
  maxDownloads: 0,
})

const expireOptions = [
  { label: $gettext('1 hour'), value: 1 },
  { label: $gettext('1 day'), value: 24 },
  { label: $gettext('7 days'), value: 168 },
  { label: $gettext('30 days'), value: 720 },
]

const shareUrl = (token: string) => `${window.location.origin}/download/${token}`

const isExpired = (row: any) => new Date(row.expired_at).getTime() <= Date.now()

const refresh = () => {
  loading.value = true
  useRequest(file.shareList())
    .onSuccess(({ data }: any) => {
      shares.value = (data || []).filter((item: any) => item.path === path.value)
    })
    .onComplete(() => {
      loading.value = false
    })
}

const handleCreate = () => {
  useRequest(
    file.shareCreate(path.value, createModel.value.maxDownloads, createModel.value.expireHours),
  ).onSuccess(({ data }: any) => {
    window.$message.success($gettext('Share link created successfully'))
    copy2clipboard(shareUrl(data.token)).then(() => {
      window.$message.success($gettext('Link copied to clipboard'))
    })
    refresh()
    emit('refresh')
  })
}

const handleCopy = (row: any) => {
  copy2clipboard(shareUrl(row.token)).then(() => {
    window.$message.success($gettext('Link copied to clipboard'))
  })
}

const handleDelete = async (row: any) => {
  const ok = await confirmAction({
    title: $gettext('Cancel Share'),
    content: $gettext('Are you sure you want to cancel this share?'),
  })
  if (!ok) return
  useRequest(file.shareDelete(row.id)).onSuccess(() => {
    window.$message.success($gettext('Share cancelled successfully'))
    refresh()
    emit('refresh')
  })
}

const columns = computed<DataTableColumns<any>>(() => [
  {
    title: $gettext('Link'),
    key: 'token',
    minWidth: 220,
    render: (row) => h('span', { class: 'break-all' }, shareUrl(row.token)),
  },
  {
    title: $gettext('Downloads'),
    key: 'downloads',
    width: 100,
    render: (row) =>
      row.max_downloads > 0 ? `${row.downloads} / ${row.max_downloads}` : `${row.downloads}`,
  },
  {
    title: $gettext('Expire Time'),
    key: 'expired_at',
    width: 180,
    render: (row) =>
      h(
        NTag,
        { type: isExpired(row) ? 'error' : 'success', size: 'small', bordered: false },
        { default: () => formatDateTime(row.expired_at) },
      ),
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 180,
    render: (row) =>
      h(NFlex, { size: 8 }, () => [
        h(
          NButton,
          { size: 'tiny', type: 'info', tertiary: true, onClick: () => handleCopy(row) },
          { default: () => $gettext('Copy Link') },
        ),
        h(
          NButton,
          { size: 'tiny', type: 'error', tertiary: true, onClick: () => handleDelete(row) },
          { default: () => $gettext('Cancel Share') },
        ),
      ]),
  },
])

watch(
  () => show.value,
  (newVal) => {
    if (!newVal) return
    createModel.value = { expireHours: 24, maxDownloads: 0 }
    refresh()
  },
)
</script>

<template>
  <app-modal v-model:show="show" :title="$gettext('Share - %{ path }', { path })">
    <n-flex vertical :size="16">
      <n-form v-bind="formLayout" class="flex-wrap">
        <n-form-item :label="$gettext('Validity Period')">
          <n-select
            v-model:value="createModel.expireHours"
            :options="expireOptions"
            class="md:!w-40"
          />
        </n-form-item>
        <n-form-item :label="$gettext('Download Limit')">
          <n-input-number v-model:value="createModel.maxDownloads" :min="0" class="w-full md:!w-40">
            <template #suffix>
              <span v-if="createModel.maxDownloads === 0" class="text-gray-400">
                {{ $gettext('Unlimited') }}
              </span>
            </template>
          </n-input-number>
        </n-form-item>
        <n-form-item :show-label="false">
          <n-button type="primary" @click="handleCreate">
            {{ $gettext('Create Share Link') }}
          </n-button>
        </n-form-item>
      </n-form>
      <n-data-table
        :columns="columns"
        :data="shares"
        :loading="loading"
        size="small"
        :bordered="false"
      />
    </n-flex>
  </app-modal>
</template>

<style scoped lang="scss"></style>
