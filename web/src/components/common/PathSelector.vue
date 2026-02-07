<script setup lang="ts">
import file from '@/api/panel/file'
import TheIcon from '@/components/custom/TheIcon.vue'
import { checkName, checkPath, getExt, getIconByExt } from '@/utils'
import type { DataTableColumns, InputInst } from 'naive-ui'
import { NButton, NDataTable, NEllipsis, NFlex, NSpin, NTag, useThemeVars } from 'naive-ui'
import type { RowData } from 'naive-ui/es/data-table/src/interface'
import { useGettext } from 'vue3-gettext'

const { $gettext } = useGettext()
const themeVars = useThemeVars()
const show = defineModel<boolean>('show', { type: Boolean, required: true })
const path = defineModel<string>('path', { type: String, required: true })
const props = defineProps({
  dir: {
    type: Boolean,
    required: true
  }
})

const currentPath = ref('/')

// 目录大小计算状态
const sizeLoading = ref<Map<string, boolean>>(new Map())
const sizeCache = ref<Map<string, string>>(new Map())

const title = computed(() => (props.dir ? $gettext('Select Directory') : $gettext('Select File')))
const isInput = ref(false)
const pathInput = ref<InputInst | null>(null)
const input = ref('www')
const sort = ref<string>('')
const selected = ref<any[]>([])
const create = ref(false)
const createLoading = ref(false)
const createModel = ref({
  dir: false,
  path: ''
})

const columns: DataTableColumns<RowData> = [
  {
    type: 'selection',
    multiple: false,
    fixed: 'left',
    disabled(row) {
      return props.dir ? !row.dir : row.dir
    }
  },
  {
    title: $gettext('Name'),
    key: 'name',
    minWidth: 180,
    defaultSortOrder: false,
    sorter: 'default',
    render(row) {
      let icon = 'mdi:file-outline'
      if (row.dir) {
        icon = 'mdi:folder-outline'
      } else {
        icon = getIconByExt(getExt(row.name))
      }

      return h(
        NFlex,
        {
          class: 'cursor-pointer hover:opacity-60',
          onClick: () => {
            if (row.dir) {
              currentPath.value = row.full
            }
          }
        },
        () => [
          h(TheIcon, { icon, size: 24 }),
          h(NEllipsis, null, {
            default: () => {
              if (row.symlink) {
                return row.name + ' -> ' + row.link
              } else {
                return row.name
              }
            }
          })
        ]
      )
    }
  },
  {
    title: $gettext('Permissions'),
    key: 'mode',
    minWidth: 80,
    render(row: any): any {
      return h(
        NTag,
        { type: 'success', size: 'small', bordered: false },
        { default: () => row.mode }
      )
    }
  },
  {
    title: $gettext('Owner / Group'),
    key: 'owner/group',
    minWidth: 120,
    render(row: any): any {
      return h('div', null, [
        h(NTag, { type: 'primary', size: 'small', bordered: false }, { default: () => row.owner }),
        ' / ',
        h(NTag, { type: 'primary', size: 'small', bordered: false }, { default: () => row.group })
      ])
    }
  },
  {
    title: $gettext('Size'),
    key: 'size',
    minWidth: 100,
    render(row: any): any {
      // 文件
      if (!row.dir) {
        return h(
          NTag,
          { type: 'info', size: 'small', bordered: false },
          { default: () => row.size }
        )
      }
      // 目录
      const cachedSize = sizeCache.value.get(row.full)
      if (cachedSize) {
        return h(
          NTag,
          { type: 'info', size: 'small', bordered: false },
          { default: () => cachedSize }
        )
      }
      const isLoading = sizeLoading.value.get(row.full)
      if (isLoading) {
        return h(NSpin, { size: 16, style: { paddingTop: '4px' } })
      }
      return h(
        'span',
        {
          style: { cursor: 'pointer', fontSize: '14px', color: themeVars.value.primaryColor },
          onClick: (e: MouseEvent) => {
            e.preventDefault()
            e.stopPropagation()
            calculateDirSize(row.full)
          }
        },
        $gettext('Calculate')
      )
    }
  },
  {
    title: $gettext('Modification Time'),
    key: 'modify',
    minWidth: 200,
    render(row: any): any {
      return h(
        NTag,
        { type: 'warning', size: 'small', bordered: false },
        { default: () => row.modify }
      )
    }
  }
]

const { loading, data, page, total, pageSize, pageCount, reload } = usePagination(
  (page, pageSize) =>
    file.list(encodeURIComponent(currentPath.value), '', false, sort.value, page, pageSize),
  {
    initialData: { total: 0, list: [] },
    initialPageSize: 100,
    total: (res: any) => res.total,
    data: (res: any) => res.items
  }
)

const handleInput = () => {
  isInput.value = true
  nextTick(() => {
    pathInput.value?.focus()
  })
}

const handleBlur = () => {
  input.value = input.value.replace(/(^\/)|(\/$)/g, '')
  if (!checkPath(input.value)) {
    window.$message.error($gettext('Invalid path'))
    return
  }

  isInput.value = false
  currentPath.value = '/' + input.value
}

const handleUp = () => {
  const count = splitPath(currentPath.value, '/').length
  setPath(count - 2)
}

const splitPath = (str: string, delimiter: string) => {
  if (str === delimiter || str === '') {
    return []
  }
  return str.split(delimiter).slice(1)
}

const setPath = (index: number) => {
  const newPath = splitPath(currentPath.value, '/')
    .slice(0, index + 1)
    .join('/')
  currentPath.value = '/' + newPath
  input.value = newPath
}

const handleSorterChange = (sorter: {
  columnKey: string | number | null
  order: 'ascend' | 'descend' | false
}) => {
  if (!sorter || sorter.columnKey === 'name') {
    if (!loading.value) {
      switch (sorter.order) {
        case 'ascend':
          sort.value = 'asc'
          reload()
          break
        case 'descend':
          sort.value = 'desc'
          reload()
          break
        default:
          sort.value = ''
          reload()
          break
      }
    }
  }
}

const showCreate = (value: string) => {
  createModel.value.dir = value !== 'file'
  createModel.value.path = ''
  create.value = true
}

const handleCreate = () => {
  if (!checkName(createModel.value.path)) {
    window.$message.error($gettext('Invalid name'))
    return
  }

  createLoading.value = true
  const fullPath = currentPath.value + '/' + createModel.value.path
  useRequest(file.create(fullPath, createModel.value.dir))
    .onSuccess(() => {
      create.value = false
      reload()
      window.$message.success($gettext('Created successfully'))
    })
    .onComplete(() => {
      createLoading.value = false
    })
}

// 计算目录大小
const calculateDirSize = (dirPath: string) => {
  sizeLoading.value.set(dirPath, true)
  useRequest(file.size(dirPath))
    .onSuccess(({ data }) => {
      sizeCache.value.set(dirPath, data)
    })
    .onComplete(() => {
      sizeLoading.value.set(dirPath, false)
    })
}

// 打开选择器时，用外部path初始化内部currentPath
watch(show, (val) => {
  if (val) {
    currentPath.value = path.value || '/'
  }
})

// 监听内部路径变化，刷新列表
watch(currentPath, (value) => {
  if (!value) return
  input.value = value.slice(1)
  selected.value = []
  sizeCache.value.clear()
  sizeLoading.value.clear()
  reload()
})

// 选择后更新外部path并关闭
watch(selected, (val) => {
  if (val.length > 0) {
    path.value = selected.value[0]
    selected.value = []
    show.value = false
  }
})
</script>

<template>
  <n-modal
    v-model:show="show"
    preset="card"
    :title="title"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
    @close="show = false"
    @mask-click="show = false"
  >
    <n-flex>
      <n-popselect
        :options="[
          { label: $gettext('File'), value: 'file' },
          { label: $gettext('Folder'), value: 'folder' }
        ]"
        @update:value="showCreate"
      >
        <n-button type="primary"> {{ $gettext('Create') }} </n-button>
      </n-popselect>
      <n-button @click="handleUp">
        <i-mdi-arrow-up :size="16" />
      </n-button>
      <n-input-group flex-1>
        <n-tag size="large" v-if="!isInput" flex-1 @click="handleInput">
          <n-breadcrumb separator=">">
            <n-breadcrumb-item @click.stop="setPath(-1)">
              {{ $gettext('Root Directory') }}
            </n-breadcrumb-item>
            <n-breadcrumb-item
              v-for="(item, index) in splitPath(currentPath, '/')"
              :key="index"
              @click.stop="setPath(index)"
            >
              {{ item }}
            </n-breadcrumb-item>
          </n-breadcrumb>
        </n-tag>
        <n-input-group-label v-if="isInput">/</n-input-group-label>
        <n-input
          ref="pathInput"
          v-model:value="input"
          v-if="isInput"
          @keyup.enter="handleBlur"
          @blur="handleBlur"
        />
      </n-input-group>
      <n-button @click="reload">
        <i-mdi-refresh :size="16" />
      </n-button>
    </n-flex>
    <n-data-table
      remote
      striped
      virtual-scroll
      pt-20
      size="small"
      :scroll-x="600"
      :columns="columns"
      :data="data"
      :loading="loading"
      :row-key="(row: any) => row.full"
      max-height="60vh"
      @update:sorter="handleSorterChange"
      v-model:checked-row-keys="selected"
      v-model:page="page"
      v-model:pageSize="pageSize"
      :pagination="{
        page: page,
        pageCount: pageCount,
        pageSize: pageSize,
        itemCount: total,
        showQuickJumper: true,
        showSizePicker: true,
        pageSizes: [100, 200, 500, 1000, 1500, 2000, 5000]
      }"
    />
  </n-modal>
  <n-modal
    v-model:show="create"
    preset="card"
    :title="$gettext('Create')"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
  >
    <n-space vertical>
      <n-form :model="createModel">
        <n-form-item :label="$gettext('Name')">
          <n-input v-model:value="createModel.path" />
        </n-form-item>
      </n-form>
      <n-button type="info" block :loading="createLoading" :disabled="createLoading" @click="handleCreate">{{ $gettext('Submit') }}</n-button>
    </n-space>
  </n-modal>
</template>

<style scoped lang="scss"></style>
