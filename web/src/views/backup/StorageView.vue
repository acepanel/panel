<script setup lang="ts">
import { NButton, NDataTable, NFlex } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import storage from '@/api/panel/backup-storage'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { formatDateTime } from '@/utils'

const { $gettext } = useGettext()
const { confirmDelete } = useConfirm()

const createModal = ref(false)
const createLoading = ref(false)
const editModal = ref(false)
const editLoading = ref(false)
const editId = ref(0)

const typeOptions = [
  { label: 'S3', value: 's3' },
  { label: 'SFTP', value: 'sftp' },
  { label: 'WebDAV', value: 'webdav' },
]

const styleOptions = [
  { label: 'Virtual Hosted', value: 'virtual-hosted' },
  { label: 'Path', value: 'path' },
]

const schemeOptions = [
  { label: 'HTTPS', value: 'https' },
  { label: 'HTTP', value: 'http' },
]

const sftpAuthOptions = [
  { label: $gettext('Password'), value: 'password' },
  { label: $gettext('Private Key'), value: 'private_key' },
]

const defaultModel = {
  type: 's3',
  name: '',
  info: {
    access_key: '',
    secret_key: '',
    style: 'virtual-hosted',
    region: 'us-east-1',
    endpoint: '',
    scheme: 'https',
    bucket: '',
    url: '',
    host: '',
    port: 22,
    username: '',
    password: '',
    private_key: '',
    auth_type: 'password',
    path: '',
  },
}

const createModel = ref({ ...defaultModel, info: { ...defaultModel.info } })
const editModel = ref({ ...defaultModel, info: { ...defaultModel.info } })

const columns: any = [
  {
    title: $gettext('Name'),
    key: 'name',
    minWidth: 150,
    resizable: true,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Type'),
    key: 'type',
    width: 120,
    render(row: any) {
      const typeMap: Record<string, string> = {
        local: $gettext('Local'),
        s3: 'S3',
        sftp: 'SFTP',
        webdav: 'WebDAV',
      }
      return typeMap[row.type] || row.type
    },
  },
  {
    title: $gettext('Created At'),
    key: 'created_at',
    width: 180,
    render(row: any) {
      return formatDateTime(row.created_at)
    },
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 200,
    hideInExcel: true,
    render(row: any) {
      const isLocal = row.type === 'local'
      return h(NFlex, { size: 'small', align: 'center' }, () => [
        h(
          NButton,
          {
            size: 'small',
            type: 'primary',
            secondary: true,
            disabled: isLocal,
            onClick: () => handleEdit(row),
          },
          { default: () => $gettext('Edit') },
        ),
        h(
          NButton,
          {
            size: 'small',
            type: 'error',
            disabled: isLocal,
            onClick: async () => {
              const ok = await confirmDelete({
                content: $gettext('Are you sure you want to delete this storage?'),
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

const { loading, data, page, total, pageSize, pageCount, refresh } = usePagination(
  (page, pageSize) => storage.list(page, pageSize),
  {
    initialData: { total: 0, list: [] },
    initialPageSize: 20,
    total: (res: any) => res.total,
    data: (res: any) => res.items,
  },
)

const handleCreate = () => {
  createLoading.value = true
  useRequest(storage.create(createModel.value))
    .onSuccess(() => {
      createModal.value = false
      createModel.value = { ...defaultModel, info: { ...defaultModel.info } }
      refresh()
      window.$message.success($gettext('Created successfully'))
    })
    .onComplete(() => {
      createLoading.value = false
    })
}

const handleEdit = (row: any) => {
  editId.value = row.id
  editModel.value = {
    type: row.type,
    name: row.name,
    info: { ...defaultModel.info, ...row.info },
  }
  editModal.value = true
}

const handleUpdate = () => {
  editLoading.value = true
  useRequest(storage.update(editId.value, editModel.value))
    .onSuccess(() => {
      editModal.value = false
      refresh()
      window.$message.success($gettext('Updated successfully'))
    })
    .onComplete(() => {
      editLoading.value = false
    })
}

const handleDelete = (id: number) => {
  useRequest(storage.delete(id)).onSuccess(() => {
    refresh()
    window.$message.success($gettext('Deleted successfully'))
  })
}

onMounted(() => {
  refresh()
})
</script>

<template>
  <n-flex vertical :size="20">
    <n-flex>
      <n-button type="primary" @click="createModal = true">{{ $gettext('Add Storage') }}</n-button>
    </n-flex>
    <n-data-table
      v-model:page="page"
      v-model:pageSize="pageSize"
      striped
      remote
      :scroll-x="800"
      :loading="loading"
      :columns="columns"
      :data="data"
      :row-key="(row: any) => row.id"
      :pagination="{
        page: page,
        pageSize: pageSize,
        itemCount: total,
        showQuickJumper: true,
        showSizePicker: true,
        pageSizes: [20, 50, 100, 200],
      }"
    />
  </n-flex>

  <!-- Create Modal -->
  <n-modal
    v-model:show="createModal"
    preset="card"
    :title="$gettext('Add Storage')"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
    @close="createModal = false"
  >
    <n-form :model="createModel">
      <n-form-item :label="$gettext('Name')" required>
        <n-input v-model:value="createModel.name" :placeholder="$gettext('Enter storage name')" />
      </n-form-item>
      <n-form-item :label="$gettext('Type')" required>
        <n-select v-model:value="createModel.type" :options="typeOptions" />
      </n-form-item>

      <!-- S3 Fields -->
      <template v-if="createModel.type === 's3'">
        <n-form-item :label="$gettext('Access Key')" required>
          <n-input
            v-model:value="createModel.info.access_key"
            :placeholder="$gettext('Enter access key')"
          />
        </n-form-item>
        <n-form-item :label="$gettext('Secret Key')" required>
          <n-input
            v-model:value="createModel.info.secret_key"
            type="password"
            show-password-on="click"
            :placeholder="$gettext('Enter secret key')"
          />
        </n-form-item>
        <n-form-item :label="$gettext('Style')">
          <n-select v-model:value="createModel.info.style" :options="styleOptions" />
        </n-form-item>
        <n-form-item :label="$gettext('Region')">
          <n-input
            v-model:value="createModel.info.region"
            :placeholder="$gettext('Enter region (e.g., us-east-1)')"
          />
        </n-form-item>
        <n-form-item :label="$gettext('Endpoint')" required>
          <n-input
            v-model:value="createModel.info.endpoint"
            :placeholder="$gettext('Enter endpoint URL')"
          />
        </n-form-item>
        <n-form-item :label="$gettext('Scheme')">
          <n-select v-model:value="createModel.info.scheme" :options="schemeOptions" />
        </n-form-item>
        <n-form-item :label="$gettext('Bucket')" required>
          <n-input
            v-model:value="createModel.info.bucket"
            :placeholder="$gettext('Enter bucket name')"
          />
        </n-form-item>
        <n-form-item :label="$gettext('Path')">
          <n-input
            v-model:value="createModel.info.path"
            :placeholder="$gettext('Enter path (optional)')"
          />
        </n-form-item>
      </template>

      <!-- SFTP Fields -->
      <template v-if="createModel.type === 'sftp'">
        <n-form-item :label="$gettext('Host')" required>
          <n-input v-model:value="createModel.info.host" :placeholder="$gettext('Enter host')" />
        </n-form-item>
        <n-form-item :label="$gettext('Port')" required>
          <n-input-number
            v-model:value="createModel.info.port"
            :min="1"
            :max="65535"
            :placeholder="$gettext('Enter port')"
          />
        </n-form-item>
        <n-form-item :label="$gettext('Username')" required>
          <n-input
            v-model:value="createModel.info.username"
            :placeholder="$gettext('Enter username')"
          />
        </n-form-item>
        <n-form-item :label="$gettext('Auth Type')" required>
          <n-select v-model:value="createModel.info.auth_type" :options="sftpAuthOptions" />
        </n-form-item>
        <n-form-item
          v-if="createModel.info.auth_type === 'password'"
          :label="$gettext('Password')"
          required
        >
          <n-input
            v-model:value="createModel.info.password"
            type="password"
            show-password-on="click"
            :placeholder="$gettext('Enter password')"
          />
        </n-form-item>
        <n-form-item
          v-if="createModel.info.auth_type === 'private_key'"
          :label="$gettext('Private Key')"
          required
        >
          <n-input
            v-model:value="createModel.info.private_key"
            type="textarea"
            :rows="5"
            :placeholder="$gettext('Enter private key')"
          />
        </n-form-item>
        <n-form-item :label="$gettext('Path')" required>
          <n-input
            v-model:value="createModel.info.path"
            :placeholder="$gettext('Enter remote path')"
          />
        </n-form-item>
      </template>

      <!-- WebDAV Fields -->
      <template v-if="createModel.type === 'webdav'">
        <n-form-item :label="$gettext('URL')" required>
          <n-input
            v-model:value="createModel.info.url"
            :placeholder="$gettext('Enter WebDAV URL')"
          />
        </n-form-item>
        <n-form-item :label="$gettext('Username')" required>
          <n-input
            v-model:value="createModel.info.username"
            :placeholder="$gettext('Enter username')"
          />
        </n-form-item>
        <n-form-item :label="$gettext('Password')" required>
          <n-input
            v-model:value="createModel.info.password"
            type="password"
            show-password-on="click"
            :placeholder="$gettext('Enter password')"
          />
        </n-form-item>
        <n-form-item :label="$gettext('Path')">
          <n-input
            v-model:value="createModel.info.path"
            :placeholder="$gettext('Enter path (optional)')"
          />
        </n-form-item>
      </template>
    </n-form>
    <n-button
      type="info"
      block
      :loading="createLoading"
      :disabled="createLoading"
      @click="handleCreate"
    >
      {{ $gettext('Submit') }}
    </n-button>
  </n-modal>

  <!-- Edit Modal -->
  <n-modal
    v-model:show="editModal"
    preset="card"
    :title="$gettext('Edit Storage')"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
    @close="editModal = false"
  >
    <n-form :model="editModel">
      <n-form-item :label="$gettext('Name')" required>
        <n-input v-model:value="editModel.name" :placeholder="$gettext('Enter storage name')" />
      </n-form-item>
      <n-form-item :label="$gettext('Type')" required>
        <n-select v-model:value="editModel.type" :options="typeOptions" />
      </n-form-item>

      <!-- S3 Fields -->
      <template v-if="editModel.type === 's3'">
        <n-form-item :label="$gettext('Access Key')" required>
          <n-input
            v-model:value="editModel.info.access_key"
            :placeholder="$gettext('Enter access key')"
          />
        </n-form-item>
        <n-form-item :label="$gettext('Secret Key')" required>
          <n-input
            v-model:value="editModel.info.secret_key"
            type="password"
            show-password-on="click"
            :placeholder="$gettext('Enter secret key')"
          />
        </n-form-item>
        <n-form-item :label="$gettext('Style')">
          <n-select v-model:value="editModel.info.style" :options="styleOptions" />
        </n-form-item>
        <n-form-item :label="$gettext('Endpoint')" required>
          <n-input
            v-model:value="editModel.info.endpoint"
            :placeholder="$gettext('Enter endpoint URL')"
          />
        </n-form-item>
        <n-form-item :label="$gettext('Scheme')">
          <n-select v-model:value="editModel.info.scheme" :options="schemeOptions" />
        </n-form-item>
        <n-form-item :label="$gettext('Bucket')" required>
          <n-input
            v-model:value="editModel.info.bucket"
            :placeholder="$gettext('Enter bucket name')"
          />
        </n-form-item>
        <n-form-item :label="$gettext('Path')">
          <n-input
            v-model:value="editModel.info.path"
            :placeholder="$gettext('Enter path (optional)')"
          />
        </n-form-item>
      </template>

      <!-- SFTP Fields -->
      <template v-if="editModel.type === 'sftp'">
        <n-form-item :label="$gettext('Host')" required>
          <n-input v-model:value="editModel.info.host" :placeholder="$gettext('Enter host')" />
        </n-form-item>
        <n-form-item :label="$gettext('Port')" required>
          <n-input-number
            v-model:value="editModel.info.port"
            :min="1"
            :max="65535"
            :placeholder="$gettext('Enter port')"
          />
        </n-form-item>
        <n-form-item :label="$gettext('Username')" required>
          <n-input
            v-model:value="editModel.info.username"
            :placeholder="$gettext('Enter username')"
          />
        </n-form-item>
        <n-form-item :label="$gettext('Auth Type')" required>
          <n-select v-model:value="editModel.info.auth_type" :options="sftpAuthOptions" />
        </n-form-item>
        <n-form-item
          v-if="editModel.info.auth_type === 'password'"
          :label="$gettext('Password')"
          required
        >
          <n-input
            v-model:value="editModel.info.password"
            type="password"
            show-password-on="click"
            :placeholder="$gettext('Enter password')"
          />
        </n-form-item>
        <n-form-item
          v-if="editModel.info.auth_type === 'private_key'"
          :label="$gettext('Private Key')"
          required
        >
          <n-input
            v-model:value="editModel.info.private_key"
            type="textarea"
            :rows="5"
            :placeholder="$gettext('Enter private key')"
          />
        </n-form-item>
        <n-form-item :label="$gettext('Path')" required>
          <n-input
            v-model:value="editModel.info.path"
            :placeholder="$gettext('Enter remote path')"
          />
        </n-form-item>
      </template>

      <!-- WebDAV Fields -->
      <template v-if="editModel.type === 'webdav'">
        <n-form-item :label="$gettext('URL')" required>
          <n-input v-model:value="editModel.info.url" :placeholder="$gettext('Enter WebDAV URL')" />
        </n-form-item>
        <n-form-item :label="$gettext('Username')" required>
          <n-input
            v-model:value="editModel.info.username"
            :placeholder="$gettext('Enter username')"
          />
        </n-form-item>
        <n-form-item :label="$gettext('Password')" required>
          <n-input
            v-model:value="editModel.info.password"
            type="password"
            show-password-on="click"
            :placeholder="$gettext('Enter password')"
          />
        </n-form-item>
        <n-form-item :label="$gettext('Path')">
          <n-input
            v-model:value="editModel.info.path"
            :placeholder="$gettext('Enter path (optional)')"
          />
        </n-form-item>
      </template>
    </n-form>
    <n-button
      type="info"
      block
      :loading="editLoading"
      :disabled="editLoading"
      @click="handleUpdate"
    >
      {{ $gettext('Submit') }}
    </n-button>
  </n-modal>
</template>

<style scoped lang="scss"></style>
