<script setup lang="ts">
defineOptions({
  name: 'apps-s3fs-index',
})

import { useGettext } from 'vue3-gettext'

import s3fs from '@/api/apps/s3fs'
import AppButton from '@/components/system/AppButton.vue'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { usePagedTable } from '@/components/system/composables/usePagedTable'

const { $gettext } = useGettext()
const { confirmDelete } = useConfirm()
const addMountModal = ref(false)

const addMountModel = ref({
  ak: '',
  sk: '',
  bucket: '',
  url: '',
  path: '',
})

const columns: any = [
  {
    title: $gettext('Mount Path'),
    key: 'path',
    minWidth: 150,
    resizable: true,
    ellipsis: { tooltip: true },
  },
  { title: 'Bucket', key: 'bucket', resizable: true, minWidth: 150, ellipsis: { tooltip: true } },
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
              content: $gettext('Are you sure you want to delete mount %{ path }?', {
                path: row.path,
              }),
              positiveText: $gettext('Unmount'),
            })
            if (ok) await handleDeleteMount(row.id)
          },
        },
        { default: () => $gettext('Unmount') },
      )
    },
  },
]

const { loading, data, page, pageSize, refresh, pagination } = usePagedTable((page, pageSize) =>
  s3fs.mounts(page, pageSize),
)

const handleAddMount = async () => {
  await s3fs.add(addMountModel.value)
  refresh()
  addMountModal.value = false
  window.$message.success($gettext('Added successfully'))
}

const handleDeleteMount = async (id: number) => {
  await s3fs.delete(id)
  refresh()
  window.$message.success($gettext('Deleted successfully'))
}
</script>

<template>
  <PageContainer :show-footer="true">
    <n-flex vertical>
      <n-flex>
        <n-button type="primary" @click="addMountModal = true">
          {{ $gettext('Add Mount') }}
        </n-button>
      </n-flex>
      <n-data-table
        v-model:page="page"
        v-model:pageSize="pageSize"
        striped
        remote
        :scroll-x="450"
        :loading="loading"
        :columns="columns"
        :data="data"
        :row-key="(row: any) => row.id"
        :pagination="pagination"
      />
    </n-flex>
  </PageContainer>
  <app-modal v-model:show="addMountModal" :title="$gettext('Add Mount')">
    <n-form :model="addMountModel">
      <n-form-item path="bucket" label="Bucket">
        <n-input
          v-model:value="addMountModel.bucket"
          type="text"
          @keydown.enter.prevent
          :placeholder="$gettext('Enter Bucket name (COS format: xxxx-ID)')"
        />
      </n-form-item>
      <n-form-item path="ak" label="AK">
        <n-input
          v-model:value="addMountModel.ak"
          type="text"
          @keydown.enter.prevent
          :placeholder="$gettext('Enter AK key')"
        />
      </n-form-item>
      <n-form-item path="sk" label="SK">
        <n-input
          v-model:value="addMountModel.sk"
          type="text"
          @keydown.enter.prevent
          :placeholder="$gettext('Enter SK key')"
        />
      </n-form-item>
      <n-form-item path="url" :label="$gettext('Region Endpoint')">
        <n-input
          v-model:value="addMountModel.url"
          type="text"
          @keydown.enter.prevent
          :placeholder="
            $gettext(
              'Enter complete URL of region endpoint (e.g., https://oss-cn-beijing.aliyuncs.com)',
            )
          "
        />
      </n-form-item>
      <n-form-item path="path" :label="$gettext('Mount Directory')">
        <n-input
          v-model:value="addMountModel.path"
          type="text"
          @keydown.enter.prevent
          :placeholder="$gettext('Enter mount directory (e.g., /oss)')"
        />
      </n-form-item>
    </n-form>
    <app-button type="info" block @click="handleAddMount">
      {{ $gettext('Submit') }}
    </app-button>
  </app-modal>
</template>
