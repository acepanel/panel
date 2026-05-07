<script setup lang="ts">
import { NButton, NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import database from '@/api/panel/database'
import { useConfirm } from '@/components/system/composables/useConfirm'
import ElasticsearchDocModal from '@/views/database/ElasticsearchDocModal.vue'

const { confirmDelete } = useConfirm()

const props = defineProps<{
  type: string
}>()

const { $gettext } = useGettext()

// 服务器选择
const selectedServer = ref<number | null>(null)
const serverOptions = ref<any[]>([])
const serverLoading = ref(false)

// 索引选择
const selectedIndex = ref('')
const indices = ref<any[]>([])
const indicesLoading = ref(false)

// 搜索
const search = ref('')
const searchInput = ref('')

// 文档弹窗
const docModalShow = ref(false)
const docModalMode = ref<'view' | 'create'>('view')
const docModalId = ref('')

// 创建索引弹窗
const createIndexShow = ref(false)
const createIndexName = ref('')
const createIndexLoading = ref(false)

// 健康状态颜色
const healthColorMap: Record<string, 'success' | 'warning' | 'error' | 'default'> = {
  green: 'success',
  yellow: 'warning',
  red: 'error',
}

// 加载服务器列表
const loadServers = () => {
  serverLoading.value = true
  useRequest(database.serverList(1, 10000, props.type))
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

// 加载索引列表
const loadIndices = () => {
  if (!selectedServer.value) return
  indicesLoading.value = true
  useRequest(database.esIndices(selectedServer.value))
    .onSuccess(({ data }: { data: any }) => {
      indices.value = data || []
    })
    .onComplete(() => {
      indicesLoading.value = false
    })
}

// 切换服务器时加载索引
watch(selectedServer, () => {
  selectedIndex.value = ''
  loadIndices()
})

// 索引列表列
const indexColumns: any = [
  {
    title: $gettext('Index Name'),
    key: 'name',
    minWidth: 200,
    resizable: true,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Health'),
    key: 'health',
    width: 100,
    render(row: any) {
      return h(
        NTag,
        { type: healthColorMap[row.health] || 'default', size: 'small' },
        { default: () => row.health },
      )
    },
  },
  {
    title: $gettext('Documents'),
    key: 'docs_count',
    width: 120,
  },
  {
    title: $gettext('Size'),
    key: 'store_size',
    width: 120,
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 250,
    hideInExcel: true,
    render(row: any) {
      return [
        h(
          NButton,
          {
            size: 'small',
            type: 'info',
            onClick: () => {
              selectedIndex.value = row.name
            },
          },
          { default: () => $gettext('Browse') },
        ),
        h(
          NButton,
          {
            size: 'small',
            type: 'error',
            onClick: async () => {
              const ok = await confirmDelete({
                content: $gettext('Are you sure you want to delete this index?'),
              })
              if (ok) handleDeleteIndex(row.name)
            },
          },
          { default: () => $gettext('Delete') },
        ),
      ]
    },
  },
]

// 文档列表列
const docColumns: any = [
  {
    title: 'ID',
    key: 'id',
    width: 250,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Source'),
    key: 'source',
    minWidth: 300,
    resizable: true,
    ellipsis: { tooltip: true },
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
            type: 'info',
            onClick: () => {
              docModalMode.value = 'view'
              docModalId.value = row.id
              docModalShow.value = true
            },
          },
          { default: () => $gettext('View') },
        ),
        h(
          NButton,
          {
            size: 'small',
            type: 'error',
            onClick: async () => {
              const ok = await confirmDelete({
                content: $gettext('Are you sure you want to delete this document?'),
              })
              if (ok) handleDeleteDoc(row.id)
            },
          },
          { default: () => $gettext('Delete') },
        ),
      ]
    },
  },
]

// 文档分页
const { loading, data, page, total, pageSize, pageCount, refresh } = usePagination(
  (page, pageSize) =>
    database.esData(selectedServer.value || 0, selectedIndex.value, page, pageSize, search.value),
  {
    initialData: { total: 0, list: [] },
    initialPageSize: 20,
    total: (res: any) => res.total,
    data: (res: any) => res.items,
    watchingStates: [search],
    immediate: false,
  },
)

// 选中索引时加载文档
watch(selectedIndex, (val) => {
  if (val) {
    search.value = ''
    searchInput.value = ''
    page.value = 1
    refresh()
  }
})

const handleSearch = () => {
  search.value = searchInput.value
  page.value = 1
}

const handleDeleteIndex = (name: string) => {
  if (!selectedServer.value) return
  useRequest(database.esIndexDelete(selectedServer.value, name)).onSuccess(() => {
    loadIndices()
    if (selectedIndex.value === name) selectedIndex.value = ''
    window.$message.success($gettext('Deleted successfully'))
  })
}

const handleDeleteDoc = (id: string) => {
  if (!selectedServer.value) return
  useRequest(database.esDocumentDelete(selectedServer.value, selectedIndex.value, id)).onSuccess(
    () => {
      refresh()
      window.$message.success($gettext('Deleted successfully'))
    },
  )
}

const handleCreateIndex = () => {
  if (!selectedServer.value || !createIndexName.value) return
  createIndexLoading.value = true
  useRequest(database.esIndexCreate(selectedServer.value, createIndexName.value))
    .onSuccess(() => {
      createIndexShow.value = false
      createIndexName.value = ''
      loadIndices()
      window.$message.success($gettext('Created successfully'))
    })
    .onComplete(() => {
      createIndexLoading.value = false
    })
}

const openCreateDoc = () => {
  docModalMode.value = 'create'
  docModalId.value = ''
  docModalShow.value = true
}

onMounted(() => {
  loadServers()
  window.$bus.on('database-server:refresh', loadServers)
})

onUnmounted(() => {
  window.$bus.off('database-server:refresh', loadServers)
})
</script>

<template>
  <n-flex vertical>
    <n-flex justify="space-between" align="center" :wrap="false">
      <n-flex :wrap="false">
        <n-select
          v-model:value="selectedServer"
          :options="serverOptions"
          :loading="serverLoading"
          :placeholder="$gettext('Select Server')"
          class="w-62.5"
        />
        <n-button v-if="!selectedIndex" type="primary" @click="createIndexShow = true">
          {{ $gettext('Create Index') }}
        </n-button>
        <n-button v-if="selectedIndex" @click="selectedIndex = ''">
          {{ $gettext('Back to Indices') }}
        </n-button>
      </n-flex>
      <n-flex v-if="selectedIndex" :wrap="false">
        <n-input-group>
          <n-input
            v-model:value="searchInput"
            :placeholder="$gettext('Search query, e.g. field:value')"
            clearable
            class="!w-75"
            @keydown.enter="handleSearch"
          />
          <n-button type="primary" @click="handleSearch">
            {{ $gettext('Search') }}
          </n-button>
        </n-input-group>
        <n-button type="primary" @click="openCreateDoc">
          {{ $gettext('Create Document') }}
        </n-button>
      </n-flex>
    </n-flex>

    <!-- 索引列表 -->
    <n-data-table
      v-if="!selectedIndex"
      striped
      :loading="indicesLoading"
      :columns="indexColumns"
      :data="indices"
      :row-key="(row: any) => row.name"
    />

    <!-- 文档列表 -->
    <template v-else>
      <n-tag type="info">{{ $gettext('Index') }}: {{ selectedIndex }}</n-tag>
      <n-data-table
        striped
        remote
        :scroll-x="800"
        :loading="loading"
        :columns="docColumns"
        :data="data"
        :row-key="(row: any) => row.id"
        v-model:page="page"
        v-model:pageSize="pageSize"
        :pagination="{
          page: page,
          pageSize: pageSize,
          itemCount: total,
          showQuickJumper: true,
          showSizePicker: true,
          pageSizes: [20, 50, 100, 200],
        }"
      />
    </template>
  </n-flex>

  <!-- 文档弹窗 -->
  <elasticsearch-doc-modal
    v-model:show="docModalShow"
    :server-id="selectedServer || 0"
    :index="selectedIndex"
    :doc-id="docModalId"
    :mode="docModalMode"
    @saved="refresh"
  />

  <!-- 创建索引弹窗 -->
  <n-modal
    v-model:show="createIndexShow"
    preset="card"
    :title="$gettext('Create Index')"
    class="w-100"
    :bordered="false"
  >
    <n-form>
      <n-form-item :label="$gettext('Index Name')">
        <n-input
          v-model:value="createIndexName"
          :placeholder="$gettext('Enter index name')"
          @keydown.enter.prevent="handleCreateIndex"
        />
      </n-form-item>
    </n-form>
    <n-button
      type="info"
      block
      :loading="createIndexLoading"
      :disabled="createIndexLoading"
      @click="handleCreateIndex"
    >
      {{ $gettext('Submit') }}
    </n-button>
  </n-modal>
</template>
