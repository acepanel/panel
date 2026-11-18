<script setup lang="ts">
import type { DataTableColumns, DataTableInst, InputInst } from 'naive-ui'
import { NButton, NDataTable, NEllipsis, NFlex, NInput, NSpin, NTag, useThemeVars } from 'naive-ui'
import type { RowData } from 'naive-ui/es/data-table/src/interface'
import { useGettext } from 'vue3-gettext'

import file from '@/api/panel/file'
import TheIcon from '@/components/custom/TheIcon.vue'
import { usePagedTable } from '@/components/system/composables/usePagedTable'
import { checkName, checkPath, getExt, getIconByExt, joinPath } from '@/utils'

const { $gettext } = useGettext()
const themeVars = useThemeVars()
const show = defineModel<boolean>('show', { type: Boolean, required: true })
const path = defineModel<string>('path', { type: String, required: true })
const props = defineProps({
  dir: {
    type: Boolean,
    required: true,
  },
})
// 选中与起始路径相同时 path 不变，调用方靠它区分选中和取消
const emit = defineEmits<{ select: [path: string] }>()

const currentPath = ref('/')

// 目录大小计算状态
const sizeLoading = ref<Map<string, boolean>>(new Map())
const sizeCache = ref<Map<string, string>>(new Map())

const title = computed(() => (props.dir ? $gettext('Select Directory') : $gettext('Select File')))
const isInput = ref(false)
const pathInput = ref<InputInst | null>(null)
const input = ref('')
const sort = ref<string>('')
const selected = ref<any[]>([])
const tableRef = ref<DataTableInst | null>(null)
const create = ref(false)
const createLoading = ref(false)
const createInput = ref<InputInst | null>(null)
const createModel = ref({
  dir: false,
  path: '',
})

// 新建行的 row key，真实路径都以 / 开头不会冲突
const CREATE_KEY = '__create__'

const columns: DataTableColumns<RowData> = [
  {
    type: 'selection',
    multiple: false,
    fixed: 'left',
    disabled(row) {
      return row.full === CREATE_KEY || (props.dir ? !row.dir : row.dir)
    },
  },
  {
    title: $gettext('Name'),
    key: 'name',
    minWidth: 180,
    defaultSortOrder: false,
    sorter: 'default',
    colSpan: (row) => (row.full === CREATE_KEY ? columns.length - 1 : 1),
    render(row) {
      let icon = 'mdi:file-outline'
      if (row.dir) {
        icon = 'mdi:folder-outline'
      } else {
        icon = getIconByExt(getExt(row.name))
      }

      if (row.full === CREATE_KEY) {
        return h(NFlex, { align: 'center', wrap: false }, () => [
          h(TheIcon, { icon, size: 24 }),
          h(NInput, {
            ref: createInput,
            size: 'small',
            value: createModel.value.path,
            disabled: createLoading.value,
            loading: createLoading.value || undefined,
            placeholder: row.dir ? $gettext('Folder name') : $gettext('File name'),
            onUpdateValue: (v: string) => (createModel.value.path = v),
            onKeydown: handleCreateKeydown,
            onBlur: handleCreate,
          }),
        ])
      }

      return h(
        NFlex,
        {
          class: 'cursor-pointer hover:opacity-60',
          onClick: () => {
            if (row.dir) {
              currentPath.value = row.full
            }
          },
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
            },
          }),
        ],
      )
    },
  },
  {
    title: $gettext('Permissions'),
    key: 'mode',
    minWidth: 80,
    render(row: any): any {
      return h(
        NTag,
        { type: 'success', size: 'small', bordered: false },
        { default: () => row.mode },
      )
    },
  },
  {
    title: $gettext('Owner / Group'),
    key: 'owner/group',
    minWidth: 120,
    render(row: any): any {
      return h('div', null, [
        h(NTag, { type: 'primary', size: 'small', bordered: false }, { default: () => row.owner }),
        ' / ',
        h(NTag, { type: 'primary', size: 'small', bordered: false }, { default: () => row.group }),
      ])
    },
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
          { default: () => row.size },
        )
      }
      // 目录
      const cachedSize = sizeCache.value.get(row.full)
      if (cachedSize) {
        return h(
          NTag,
          { type: 'info', size: 'small', bordered: false },
          { default: () => cachedSize },
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
          },
        },
        $gettext('Calculate'),
      )
    },
  },
  {
    title: $gettext('Modification Time'),
    key: 'modify',
    minWidth: 200,
    render(row: any): any {
      return h(
        NTag,
        { type: 'warning', size: 'small', bordered: false },
        { default: () => row.modify },
      )
    },
  },
]

const { loading, data, page, pageSize, reload, pagination } = usePagedTable(
  (page, pageSize) => file.list(currentPath.value, '', false, sort.value, page, pageSize),
  {
    immediate: false,
    initialPageSize: 100,
    pageSizes: [100, 200, 500, 1000, 1500, 2000, 5000],
  },
)

// 新建时在列表顶部插入输入行
const rows = computed(() =>
  create.value
    ? [{ full: CREATE_KEY, name: '', dir: createModel.value.dir }, ...data.value]
    : data.value,
)

const handleInput = () => {
  isInput.value = true
  nextTick(() => {
    pathInput.value?.focus()
  })
}

// 输入框呈现完整绝对路径，复制出去可直接使用；失焦时容错补前导 / 并去掉尾部 /
const handleBlur = () => {
  const trimmed = input.value.trim().replace(/\/+$/, '')
  const full = trimmed.startsWith('/') ? trimmed : '/' + trimmed
  if (!checkPath(full.slice(1))) {
    window.$message.error($gettext('Invalid path'))
    return
  }

  isInput.value = false
  currentPath.value = full
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
  // 输入行在顶部，虚拟列表滚动过后需回到顶部才会渲染
  tableRef.value?.scrollTo({ top: 0 })
}

const handleCreate = () => {
  if (!create.value || createLoading.value) return
  const name = createModel.value.path.trim()
  if (!name) {
    create.value = false
    return
  }
  if (!checkName(name)) {
    window.$message.error($gettext('Invalid name'))
    return
  }

  createLoading.value = true
  useRequest(file.create(joinPath(currentPath.value, name), createModel.value.dir))
    .onSuccess(() => {
      create.value = false
      reload()
      window.$message.success($gettext('Created successfully'))
    })
    .onComplete(() => {
      createLoading.value = false
    })
}

const handleCreateKeydown = (e: KeyboardEvent) => {
  if (e.isComposing) return
  if (e.key === 'Enter') {
    handleCreate()
  } else if (e.key === 'Escape') {
    // 阻止冒泡到弹窗，否则 Esc 会连选择器一起关掉
    e.stopPropagation()
    create.value = false
  }
}

// 输入行渲染出来后自动聚焦
watch(createInput, (el) => el?.focus())

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

// 面包屑放不下时横向滑动，打开或切换目录后停在末尾显示当前目录
const crumbRef = ref<HTMLElement | null>(null)
watch([crumbRef, currentPath], ([el]) => el?.scrollTo({ left: el.scrollWidth }), {
  flush: 'post',
})

// 打开时、打开期间路径变化时刷新列表；并成一个 watch，打开时路径同时变了也只请求一次
watch([show, currentPath], ([visible, value]) => {
  if (!visible || !value) return
  input.value = value
  selected.value = []
  create.value = false
  sizeCache.value.clear()
  sizeLoading.value.clear()
  reload()
})

const handleSelect = (value: string) => {
  path.value = value
  emit('select', value)
  show.value = false
}

// 选择后更新外部path并关闭
watch(selected, (val) => {
  if (val.length > 0) {
    handleSelect(val[0])
    selected.value = []
  }
})
</script>

<template>
  <app-modal v-model:show="show" :title="title">
    <n-flex>
      <n-popselect
        :options="[
          { label: $gettext('File'), value: 'file' },
          { label: $gettext('Folder'), value: 'folder' },
        ]"
        @update:value="showCreate"
      >
        <n-button type="primary"> {{ $gettext('Create') }} </n-button>
      </n-popselect>
      <n-button @click="handleUp">
        <i-mdi-arrow-up class="text-lg" />
      </n-button>
      <n-input-group class="flex-1 min-w-0">
        <n-tag size="large" v-if="!isInput" class="path-tag flex-1 min-w-0" @click="handleInput">
          <div ref="crumbRef" class="path-crumb">
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
          </div>
        </n-tag>
        <n-input
          ref="pathInput"
          v-model:value="input"
          v-if="isInput"
          @keyup.enter="handleBlur"
          @blur="handleBlur"
        />
      </n-input-group>
      <n-button @click="reload">
        <i-mdi-refresh class="text-lg" />
      </n-button>
    </n-flex>
    <n-data-table
      ref="tableRef"
      remote
      striped
      virtual-scroll
      pt-5
      size="small"
      :scroll-x="600"
      :columns="columns"
      :data="rows"
      :loading="loading"
      :row-key="(row: any) => row.full"
      max-height="60vh"
      @update:sorter="handleSorterChange"
      v-model:checked-row-keys="selected"
      v-model:page="page"
      v-model:pageSize="pageSize"
      :pagination="pagination"
    />
    <template v-if="dir" #footer>
      <n-flex justify="end">
        <n-button type="primary" @click="handleSelect(currentPath)">
          {{ $gettext('Select Current Directory') }}
        </n-button>
      </n-flex>
    </template>
  </app-modal>
</template>

<style scoped lang="scss">
.path-tag :deep(.n-tag__content) {
  min-width: 0;
}

// 目录很深时横向滑动而不截断，写法同编辑器状态栏
.path-crumb {
  overflow-x: auto;
  scrollbar-width: none;

  &::-webkit-scrollbar {
    display: none;
  }
}
</style>
