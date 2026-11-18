<script setup lang="ts">
import { NButton, NFlex, NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import database from '@/api/panel/database'
import AppButton from '@/components/system/AppButton.vue'
import { useBus } from '@/components/system/composables/useBus'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { usePagedTable } from '@/components/system/composables/usePagedTable'
import { formatBytes } from '@/utils'
import RedisKeyModal from '@/views/database/RedisKeyModal.vue'

const { $gettext } = useGettext()
const { confirmDelete } = useConfirm()

// 服务器选择
const selectedServer = ref<number | null>(null)
const serverOptions = ref<any[]>([])
const serverLoading = ref(false)

// DB 选择
const selectedDB = ref(0)
const dbCount = ref(16)
const dbLoading = ref(false)
const dbOptions = computed(() => {
  const opts = []
  for (let i = 0; i < dbCount.value; i++) {
    opts.push({ label: `DB${i}`, value: i })
  }
  return opts
})

// 搜索
const search = ref('')
const searchInput = ref('')

// Key 弹窗
const keyModalShow = ref(false)
const keyModalMode = ref<'view' | 'create'>('view')
const keyModalKeyName = ref('')

// TTL 弹窗
const ttlModalShow = ref(false)
const ttlKey = ref('')
const ttlValue = ref(0)

// Rename 弹窗
const renameModalShow = ref(false)
const renameOldKey = ref('')
const renameNewKey = ref('')

// 类型对应颜色
const typeColorMap: Record<string, 'info' | 'success' | 'warning' | 'default' | 'error'> = {
  string: 'info',
  list: 'success',
  set: 'warning',
  hash: 'default',
  zset: 'error',
}

// 加载服务器列表
const loadServers = () => {
  serverLoading.value = true
  useRequest(database.serverList(1, 10000, 'redis'))
    .onSuccess(({ data }: { data: any }) => {
      serverOptions.value = (data.items || []).map((s: any) => ({
        label: `${s.name} (${s.host}:${s.port})`,
        value: s.id,
      }))
      if (serverOptions.value.length > 0 && !selectedServer.value) {
        selectedServer.value = serverOptions.value[0].value
      }
    })
    .onComplete(() => {
      serverLoading.value = false
    })
}

// 连续切换服务器时只认最后一次请求，先发后到的结果不能覆盖 DB 数量或提前结束加载态
let dbSeq = 0

// 切换服务器时获取 DB 数量
watch(selectedServer, (val) => {
  if (!val) return
  const current = ++dbSeq
  selectedDB.value = 0
  dbLoading.value = true
  useRequest(database.redisDatabases(val))
    .onSuccess(({ data }: { data: any }) => {
      if (current !== dbSeq) return
      dbCount.value = data || 16
      refresh()
    })
    .onComplete(() => {
      if (current === dbSeq) dbLoading.value = false
    })
})

const columns: any = [
  {
    title: 'Key',
    key: 'key',
    minWidth: 200,
    resizable: true,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Type'),
    key: 'type',
    width: 100,
    render(row: any) {
      return h(
        NTag,
        { type: typeColorMap[row.type] || 'default', size: 'small' },
        { default: () => row.type },
      )
    },
  },
  {
    title: 'TTL',
    key: 'ttl',
    width: 120,
    render(row: any) {
      if (row.ttl === -1) return $gettext('Permanent')
      if (row.ttl === -2) return $gettext('Expired')
      return `${row.ttl}s`
    },
  },
  {
    title: $gettext('Size'),
    key: 'size',
    width: 100,
    render(row: any) {
      return formatBytes(row.size)
    },
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 380,
    hideInExcel: true,
    render(row: any) {
      return h(NFlex, { size: 'small', align: 'center' }, () => [
        h(
          NButton,
          {
            size: 'small',
            type: 'info',
            onClick: () => {
              keyModalMode.value = 'view'
              keyModalKeyName.value = row.key
              keyModalShow.value = true
            },
          },
          { default: () => $gettext('View') },
        ),
        h(
          NButton,
          {
            size: 'small',
            type: 'warning',
            onClick: () => {
              ttlKey.value = row.key
              ttlValue.value = row.ttl > 0 ? row.ttl : 0
              ttlModalShow.value = true
            },
          },
          { default: () => 'TTL' },
        ),
        h(
          NButton,
          {
            size: 'small',
            type: 'primary',
            ghost: true,
            onClick: () => {
              renameOldKey.value = row.key
              renameNewKey.value = row.key
              renameModalShow.value = true
            },
          },
          { default: () => $gettext('Rename') },
        ),
        h(
          AppButton,
          {
            size: 'small',
            type: 'error',
            onClick: async () => {
              const ok = await confirmDelete({
                content: $gettext('Are you sure you want to delete this key?'),
              })
              if (ok) await handleDelete(row.key)
            },
          },
          { default: () => $gettext('Delete') },
        ),
      ])
    },
  },
]

const { loading, data, page, pageSize, refresh, pagination } = usePagedTable(
  (page, pageSize) =>
    database.redisData(selectedServer.value || 0, selectedDB.value, page, pageSize, search.value),
  {
    watchingStates: [selectedServer, selectedDB, search],
    immediate: false,
  },
)

const handleSearch = () => {
  search.value = searchInput.value
  page.value = 1
}

const handleDelete = async (key: string) => {
  if (!selectedServer.value) return
  await database.redisKeyDelete(selectedServer.value, selectedDB.value, key)
  refresh()
  window.$message.success($gettext('Deleted successfully'))
}

const handleSetTTL = async () => {
  if (!selectedServer.value) return
  await database.redisKeyTTL(selectedServer.value, selectedDB.value, ttlKey.value, ttlValue.value)
  ttlModalShow.value = false
  refresh()
  window.$message.success($gettext('Modified successfully'))
}

const handleRename = async () => {
  if (!selectedServer.value) return
  await database.redisKeyRename(
    selectedServer.value,
    selectedDB.value,
    renameOldKey.value,
    renameNewKey.value,
  )
  renameModalShow.value = false
  refresh()
  window.$message.success($gettext('Renamed successfully'))
}

const openCreate = () => {
  keyModalMode.value = 'create'
  keyModalKeyName.value = ''
  keyModalShow.value = true
}

const handleClear = async () => {
  if (!selectedServer.value) return
  await database.redisClear(selectedServer.value, selectedDB.value)
  refresh()
  window.$message.success($gettext('Cleared successfully'))
}

onMounted(() => {
  loadServers()
})

useBus('database-server:refresh', loadServers)
</script>

<template>
  <n-flex vertical>
    <n-flex justify="space-between" align="center">
      <n-flex class="w-full md:w-auto">
        <n-select
          v-model:value="selectedServer"
          :options="serverOptions"
          :loading="serverLoading"
          :placeholder="$gettext('Select Server')"
          class="md:!w-62.5"
        />
        <n-select
          v-model:value="selectedDB"
          :options="dbOptions"
          :loading="dbLoading"
          class="md:!w-30"
        />
      </n-flex>
      <n-flex class="w-full md:w-auto">
        <n-input-group class="md:!w-auto">
          <n-input
            v-model:value="searchInput"
            :placeholder="$gettext('Search key pattern, e.g. user:*')"
            clearable
            class="md:!w-62.5"
            @keydown.enter="handleSearch"
          />
          <n-button type="primary" @click="handleSearch">
            {{ $gettext('Search') }}
          </n-button>
        </n-input-group>
        <n-button type="primary" @click="openCreate">
          {{ $gettext('Create Key') }}
        </n-button>
        <ConfirmDialog
          type="danger"
          :content="$gettext('Are you sure you want to clear the current database?')"
          @confirm="handleClear"
        >
          <template #trigger>
            <app-button type="error">
              {{ $gettext('Clear DB') }}
            </app-button>
          </template>
        </ConfirmDialog>
      </n-flex>
    </n-flex>
    <n-data-table
      v-model:page="page"
      v-model:pageSize="pageSize"
      striped
      remote
      :scroll-x="900"
      :loading="loading"
      :columns="columns"
      :data="data"
      :row-key="(row: any) => row.key"
      :pagination="pagination"
    />
  </n-flex>
  <redis-key-modal
    v-model:show="keyModalShow"
    :server-id="selectedServer || 0"
    :db="selectedDB"
    :key-name="keyModalKeyName"
    :mode="keyModalMode"
    @saved="refresh"
  />
  <!-- TTL 弹窗 -->
  <app-modal size="sm" v-model:show="ttlModalShow" :title="$gettext('Set TTL')">
    <n-form>
      <n-form-item label="Key">
        <n-input :value="ttlKey" disabled />
      </n-form-item>
      <n-form-item label="TTL (s)">
        <n-input-number
          v-model:value="ttlValue"
          w-full
          :min="-1"
          :placeholder="$gettext('-1 means no expiration')"
        />
      </n-form-item>
    </n-form>
    <app-button type="info" block @click="handleSetTTL">
      {{ $gettext('Submit') }}
    </app-button>
  </app-modal>
  <!-- Rename 弹窗 -->
  <app-modal size="sm" v-model:show="renameModalShow" :title="$gettext('Rename Key')">
    <n-form>
      <n-form-item :label="$gettext('Old Key')">
        <n-input :value="renameOldKey" disabled />
      </n-form-item>
      <n-form-item :label="$gettext('New Key')">
        <n-input v-model:value="renameNewKey" :placeholder="$gettext('Enter new key name')" />
      </n-form-item>
    </n-form>
    <app-button type="info" block @click="handleRename">
      {{ $gettext('Submit') }}
    </app-button>
  </app-modal>
</template>
