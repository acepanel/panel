<script setup lang="ts">
import {
  NButton,
  NDataTable,
  NEllipsis,
  NFlex,
  NInput,
  NPopconfirm,
  NPopselect,
  NSpin,
  NTag,
  useThemeVars
} from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import type { DataTableColumns, DropdownOption } from 'naive-ui'
import type { RowData } from 'naive-ui/es/data-table/src/interface'

import file from '@/api/panel/file'
import TheIcon from '@/components/custom/TheIcon.vue'
import {
  checkName,
  checkPath,
  getExt,
  getFilename,
  getIconByExt,
  isCompress,
  isImage
} from '@/utils/file'
import EditModal from '@/views/file/EditModal.vue'
import PreviewModal from '@/views/file/PreviewModal.vue'
import type { Marked } from '@/views/file/types'

const { $gettext } = useGettext()
const themeVars = useThemeVars()
const sort = ref<string>('')
const path = defineModel<string>('path', { type: String, required: true }) // 当前路径
const keyword = defineModel<string>('keyword', { type: String, default: '' }) // 搜索关键词
const sub = defineModel<boolean>('sub', { type: Boolean, default: false }) // 搜索是否包括子目录
const selected = defineModel<any[]>('selected', { type: Array, default: () => [] })
const marked = defineModel<Marked[]>('marked', { type: Array, default: () => [] })
const markedType = defineModel<string>('markedType', { type: String, required: true })
const compress = defineModel<boolean>('compress', { type: Boolean, required: true })
const permission = defineModel<boolean>('permission', { type: Boolean, required: true })
const editorModal = ref(false)
const previewModal = ref(false)
const currentFile = ref('')

const showDropdown = ref(false)
const selectedRow = ref<any>()
const dropdownX = ref(0)
const dropdownY = ref(0)

// 目录大小计算状态
const sizeLoading = ref<Map<string, boolean>>(new Map())
const sizeCache = ref<Map<string, string>>(new Map())

const renameModal = ref(false)
const renameModel = ref({
  source: '',
  target: ''
})
const unCompressModal = ref(false)
const unCompressModel = ref({
  path: '',
  file: ''
})

// 检查文件是否有 immutable 属性，如果有则弹出确认对话框
const confirmImmutableOperation = (row: any, operation: string, callback: () => void) => {
  if (row.immutable) {
    window.$dialog.warning({
      title: $gettext('Warning'),
      content: $gettext(
        'The file %{ name } has immutable attribute. The system will temporarily remove the immutable attribute, perform the operation, and then restore the immutable attribute. Do you want to continue?',
        { name: row.name }
      ),
      positiveText: $gettext('Continue'),
      negativeText: $gettext('Cancel'),
      onPositiveClick: callback
    })
  } else {
    callback()
  }
}

const options = computed<DropdownOption[]>(() => {
  if (selectedRow.value == null) return []
  const options = [
    {
      label: selectedRow.value.dir
        ? $gettext('Open')
        : isImage(selectedRow.value.name)
          ? $gettext('Preview')
          : $gettext('Edit'),
      key: selectedRow.value.dir ? 'open' : isImage(selectedRow.value.name) ? 'preview' : 'edit'
    },
    { label: $gettext('Copy'), key: 'copy' },
    { label: $gettext('Move'), key: 'move' },
    { label: $gettext('Permission'), key: 'permission' },
    {
      label: selectedRow.value.dir ? $gettext('Compress') : $gettext('Download'),
      key: selectedRow.value.dir ? 'compress' : 'download'
    },
    {
      label: $gettext('Uncompress'),
      key: 'uncompress',
      show: isCompress(selectedRow.value.full),
      disabled: !isCompress(selectedRow.value.full)
    },
    { label: $gettext('Rename'), key: 'rename' },
    { label: () => h('span', { style: { color: 'red' } }, $gettext('Delete')), key: 'delete' }
  ]
  if (marked.value.length) {
    options.unshift({
      label: $gettext('Paste'),
      key: 'paste'
    })
  }

  return options
})

const columns: DataTableColumns<RowData> = [
  {
    type: 'selection',
    fixed: 'left'
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
              path.value = row.full
            } else {
              currentFile.value = row.full
              editorModal.value = true
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
          }),
          // 如果文件有 immutable 属性，显示锁定图标
          row.immutable
            ? h(TheIcon, {
                icon: 'mdi:lock',
                size: 16,
                style: { color: '#f0a020', marginLeft: '4px' }
              })
            : null
        ]
      )
    }
  },
  {
    title: $gettext('Permission'),
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
  },
  {
    title: $gettext('Actions'),
    key: 'action',
    width: 400,
    render(row) {
      return h(
        NFlex,
        {},
        {
          default: () => [
            h(
              NButton,
              {
                size: 'small',
                type: row.dir ? 'success' : 'primary',
                tertiary: true,
                onClick: () => {
                  if (!row.dir && !row.symlink) {
                    currentFile.value = row.full
                    if (isImage(row.name)) {
                      previewModal.value = true
                    } else {
                      editorModal.value = true
                    }
                  } else {
                    path.value = row.full
                  }
                }
              },
              {
                default: () => {
                  if (!row.dir && !row.symlink) {
                    return isImage(row.name) ? $gettext('Preview') : $gettext('Edit')
                  } else {
                    return $gettext('Open')
                  }
                }
              }
            ),
            h(
              NButton,
              {
                size: 'small',
                type: row.dir ? 'primary' : 'info',
                tertiary: true,
                onClick: () => {
                  if (row.dir) {
                    selected.value = [row.full]
                    compress.value = true
                  } else {
                    window.open('/api/file/download?path=' + encodeURIComponent(row.full))
                  }
                }
              },
              {
                default: () => {
                  if (row.dir) {
                    return $gettext('Compress')
                  } else {
                    return $gettext('Download')
                  }
                }
              }
            ),
            h(
              NButton,
              {
                type: 'warning',
                size: 'small',
                tertiary: true,
                onClick: () => {
                  confirmImmutableOperation(row, 'rename', () => {
                    renameModel.value.source = getFilename(row.name)
                    renameModel.value.target = getFilename(row.name)
                    renameModal.value = true
                  })
                }
              },
              { default: () => $gettext('Rename') }
            ),
            h(
              NPopconfirm,
              {
                onPositiveClick: () => {
                  useRequest(file.delete(row.full)).onComplete(() => {
                    window.$bus.emit('file:refresh')
                    window.$message.success($gettext('Deleted successfully'))
                  })
                },
                onNegativeClick: () => {}
              },
              {
                default: () => {
                  if (row.immutable) {
                    return $gettext(
                      'The file %{ name } has immutable attribute. The system will temporarily remove the immutable attribute and delete the file. Do you want to continue?',
                      { name: row.name }
                    )
                  }
                  return $gettext('Are you sure you want to delete %{ name }?', { name: row.name })
                },
                trigger: () => {
                  return h(
                    NButton,
                    {
                      size: 'small',
                      type: 'error',
                      tertiary: true
                    },
                    { default: () => $gettext('Delete') }
                  )
                }
              }
            ),
            h(
              NPopselect,
              {
                options: [
                  { label: $gettext('Copy'), value: 'copy' },
                  { label: $gettext('Move'), value: 'move' },
                  { label: $gettext('Permission'), value: 'permission' },
                  { label: $gettext('Compress'), value: 'compress' },
                  {
                    label: $gettext('Uncompress'),
                    value: 'uncompress',
                    disabled: !isCompress(row.name)
                  }
                ],
                onUpdateValue: (value) => {
                  switch (value) {
                    case 'copy':
                      markedType.value = 'copy'
                      marked.value = [
                        {
                          name: row.name,
                          source: row.full,
                          force: false
                        }
                      ]
                      window.$message.success(
                        $gettext(
                          'Marked successfully, please navigate to the destination path to paste'
                        )
                      )
                      break
                    case 'move':
                      markedType.value = 'move'
                      marked.value = [
                        {
                          name: row.name,
                          source: row.full,
                          force: false
                        }
                      ]
                      window.$message.success(
                        $gettext(
                          'Marked successfully, please navigate to the destination path to paste'
                        )
                      )
                      break
                    case 'permission':
                      selected.value = [row.full]
                      permission.value = true
                      break
                    case 'compress':
                      selected.value = [row.full]
                      compress.value = true
                      break
                    case 'uncompress':
                      unCompressModel.value.file = row.full
                      unCompressModel.value.path = path.value
                      unCompressModal.value = true
                      break
                  }
                }
              },
              {
                default: () => {
                  return h(
                    NButton,
                    {
                      tertiary: true,
                      size: 'small'
                    },
                    { default: () => $gettext('More') }
                  )
                }
              }
            )
          ]
        }
      )
    }
  }
]

const rowProps = (row: any) => {
  return {
    onContextmenu: (e: MouseEvent) => {
      e.preventDefault()
      showDropdown.value = false
      nextTick().then(() => {
        showDropdown.value = true
        selectedRow.value = row
        dropdownX.value = e.clientX
        dropdownY.value = e.clientY
      })
    }
  }
}

const { loading, data, page, total, pageSize, pageCount, refresh } = usePagination(
  (page, pageSize) =>
    file.list(encodeURIComponent(path.value), keyword.value, sub.value, sort.value, page, pageSize),
  {
    initialData: { total: 0, list: [] },
    initialPageSize: 100,
    total: (res: any) => res.total,
    data: (res: any) => res.items
  }
)

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

const handleRename = () => {
  const source = path.value + '/' + renameModel.value.source
  const target = path.value + '/' + renameModel.value.target
  if (!checkName(renameModel.value.source) || !checkName(renameModel.value.target)) {
    window.$message.error($gettext('Invalid name'))
    return
  }

  useRequest(file.exist([target])).onSuccess(({ data }) => {
    if (data[0]) {
      window.$dialog.warning({
        title: $gettext('Warning'),
        content: $gettext('There are items with the same name. Do you want to overwrite?'),
        positiveText: $gettext('Overwrite'),
        negativeText: $gettext('Cancel'),
        onPositiveClick: () => {
          useRequest(file.move([{ source, target, force: true }]))
            .onSuccess(() => {
              window.$bus.emit('file:refresh')
              window.$message.success(
                $gettext('Renamed %{ source } to %{ target } successfully', {
                  source: renameModel.value.source,
                  target: renameModel.value.target
                })
              )
            })
            .onComplete(() => {
              renameModal.value = false
            })
        }
      })
    } else {
      useRequest(file.move([{ source, target, force: false }]))
        .onSuccess(() => {
          window.$bus.emit('file:refresh')
          window.$message.success(
            $gettext('Renamed %{ source } to %{ target } successfully', {
              source: renameModel.value.source,
              target: renameModel.value.target
            })
          )
        })
        .onComplete(() => {
          renameModal.value = false
        })
    }
  })
}

const handleUnCompress = () => {
  // 移除首位的 / 去检测
  if (
    !unCompressModel.value.path.startsWith('/') ||
    !checkPath(unCompressModel.value.path.slice(1))
  ) {
    window.$message.error($gettext('Invalid path'))
    return
  }
  const message = window.$message.loading($gettext('Uncompressing...'), {
    duration: 0
  })
  useRequest(file.unCompress(unCompressModel.value.file, unCompressModel.value.path))
    .onSuccess(() => {
      unCompressModal.value = false
      window.$bus.emit('file:refresh')
      window.$message.success($gettext('Uncompressed successfully'))
    })
    .onComplete(() => {
      message?.destroy()
    })
}

const handlePaste = () => {
  if (!marked.value.length) {
    window.$message.error($gettext('Please mark the files/folders to copy or move first'))
    return
  }

  // 查重
  let flag = false
  const paths = marked.value.map((item) => {
    return {
      name: item.name,
      source: item.source,
      target: path.value + '/' + item.name,
      force: false
    }
  })
  const sources = paths.map((item: any) => item.target)
  useRequest(file.exist(sources)).onSuccess(({ data }) => {
    for (let i = 0; i < data.length; i++) {
      if (data[i]) {
        flag = true
        paths[i].force = true
      }
    }
    if (flag) {
      window.$dialog.warning({
        title: $gettext('Warning'),
        content: $gettext(
          'There are items with the same name %{ items } Do you want to overwrite?',
          {
            items: `${paths
              .filter((item) => item.force)
              .map((item) => item.name)
              .join(', ')}`
          }
        ),
        positiveText: $gettext('Overwrite'),
        negativeText: $gettext('Cancel'),
        onPositiveClick: () => {
          if (markedType.value == 'copy') {
            useRequest(file.copy(paths)).onSuccess(() => {
              marked.value = []
              window.$bus.emit('file:refresh')
              window.$message.success($gettext('Copied successfully'))
            })
          } else {
            useRequest(file.move(paths)).onSuccess(() => {
              marked.value = []
              window.$bus.emit('file:refresh')
              window.$message.success($gettext('Moved successfully'))
            })
          }
        },
        onNegativeClick: () => {
          marked.value = []
          window.$message.info($gettext('Canceled'))
        }
      })
    } else {
      if (markedType.value == 'copy') {
        useRequest(file.copy(paths)).onSuccess(() => {
          marked.value = []
          window.$bus.emit('file:refresh')
          window.$message.success($gettext('Copied successfully'))
        })
      } else {
        useRequest(file.move(paths)).onSuccess(() => {
          marked.value = []
          window.$bus.emit('file:refresh')
          window.$message.success($gettext('Moved successfully'))
        })
      }
    }
  })
}

const handleSelect = (key: string) => {
  switch (key) {
    case 'paste':
      handlePaste()
      break
    case 'open':
      path.value = selectedRow.value.full
      break
    case 'edit':
      currentFile.value = selectedRow.value.full
      editorModal.value = true
      break
    case 'preview':
      currentFile.value = selectedRow.value.full
      previewModal.value = true
      break
    case 'copy':
      markedType.value = 'copy'
      marked.value = [
        {
          name: selectedRow.value.name,
          source: selectedRow.value.full,
          force: false
        }
      ]
      window.$message.success(
        $gettext('Marked successfully, please navigate to the destination path to paste')
      )
      break
    case 'move':
      markedType.value = 'move'
      marked.value = [
        {
          name: selectedRow.value.name,
          source: selectedRow.value.full,
          force: false
        }
      ]
      window.$message.success(
        $gettext('Marked successfully, please navigate to the destination path to paste')
      )
      break
    case 'permission':
      selected.value = [selectedRow.value.full]
      permission.value = true
      break
    case 'compress':
      selected.value = [selectedRow.value.full]
      compress.value = true
      break
    case 'download':
      window.open('/api/file/download?path=' + encodeURIComponent(selectedRow.value.full))
      break
    case 'uncompress':
      unCompressModel.value.file = selectedRow.value.full
      unCompressModel.value.path = path.value
      unCompressModal.value = true
      break
    case 'rename':
      confirmImmutableOperation(selectedRow.value, 'rename', () => {
        renameModel.value.source = getFilename(selectedRow.value.name)
        renameModel.value.target = getFilename(selectedRow.value.name)
        renameModal.value = true
      })
      break
    case 'delete':
      confirmImmutableOperation(selectedRow.value, 'delete', () => {
        useRequest(file.delete(selectedRow.value.full)).onSuccess(() => {
          window.$bus.emit('file:refresh')
          window.$message.success($gettext('Deleted successfully'))
        })
      })
      break
  }
  onCloseDropdown()
}

const onCloseDropdown = () => {
  selectedRow.value = null
  showDropdown.value = false
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
          nextTick(() => {
            refresh()
          })
          break
        case 'descend':
          sort.value = 'desc'
          nextTick(() => {
            refresh()
          })
          break
        default:
          sort.value = ''
          nextTick(() => {
            refresh()
          })
          break
      }
    }
  }
}

onMounted(() => {
  // 监听路径变化并刷新列表
  watch(
    path,
    () => {
      selected.value = []
      keyword.value = ''
      sub.value = false
      sizeCache.value.clear()
      sizeLoading.value.clear()
      nextTick(() => {
        refresh()
      })
      window.$bus.emit('file:push-history', path.value)
    },
    { immediate: true }
  )
  // 监听搜索事件
  window.$bus.on('file:search', () => {
    selected.value = []
    nextTick(() => {
      refresh()
    })
    window.$bus.emit('file:push-history', path.value)
  })
  // 监听刷新事件
  window.$bus.on('file:refresh', refresh)
})

onUnmounted(() => {
  window.$bus.off('file:refresh')
})
</script>

<template>
  <n-data-table
    remote
    striped
    virtual-scroll
    size="small"
    :scroll-x="1200"
    :columns="columns"
    :data="data"
    :row-props="rowProps"
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
  <n-dropdown
    placement="bottom-start"
    trigger="manual"
    :x="dropdownX"
    :y="dropdownY"
    :options="options"
    :show="showDropdown"
    :on-clickoutside="onCloseDropdown"
    @select="handleSelect"
  />
  <edit-modal v-model:show="editorModal" v-model:file="currentFile" />
  <preview-modal v-model:show="previewModal" v-model:path="currentFile" />
  <n-modal
    v-model:show="renameModal"
    preset="card"
    :title="$gettext('Rename - %{ source }', { source: renameModel.source })"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
  >
    <n-flex vertical>
      <n-form>
        <n-form-item :label="$gettext('New Name')">
          <n-input v-model:value="renameModel.target" />
        </n-form-item>
      </n-form>
      <n-button type="primary" @click="handleRename">{{ $gettext('Save') }}</n-button>
    </n-flex>
  </n-modal>
  <n-modal
    v-model:show="unCompressModal"
    preset="card"
    :title="$gettext('Uncompress - %{ file }', { file: unCompressModel.file })"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
  >
    <n-flex vertical>
      <n-form>
        <n-form-item :label="$gettext('Uncompress to')">
          <n-input v-model:value="unCompressModel.path" />
        </n-form-item>
      </n-form>
      <n-button type="primary" @click="handleUnCompress">{{ $gettext('Uncompress') }}</n-button>
    </n-flex>
  </n-modal>
</template>
