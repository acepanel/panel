<script lang="ts" setup>
import { NButton, NCheckbox, NDataTable, NFlex, NInput, NPopover, NSwitch, NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import website from '@/api/panel/website'
import DeleteConfirm from '@/components/common/DeleteConfirm.vue'
import TheIcon from '@/components/custom/TheIcon.vue'
import { useFileStore } from '@/store'
import { isNullOrUndef } from '@/utils'
import copy2clipboard from '@vavt/copy2clipboard'

const type = defineModel<string>('type', { type: String, required: true }) // 网站类型
const createModal = defineModel<boolean>('createModal', { type: Boolean, required: true }) // 创建网站
const bulkCreateModal = defineModel<boolean>('bulkCreateModal', { type: Boolean, required: true }) // 批量创建网站

const fileStore = useFileStore()
const { $gettext } = useGettext()
const router = useRouter()
const selectedRowKeys = ref<any>([])

const columns: any = [
  { type: 'selection', fixed: 'left' },
  {
    title: $gettext('Website Name'),
    key: 'name',
    width: 200,
    resizable: true,
    ellipsis: { tooltip: true },
    render(row: any) {
      const elements = [h('span', {}, row.name)]
      if (row.domains && row.domains.length > 0) {
        elements.push(
          h(
            NPopover,
            { trigger: 'hover', placement: 'right' },
            {
              trigger: () =>
                h(
                  'span',
                  {
                    class: 'cursor-pointer hover:opacity-60 inline-flex',
                    onDblclick: () => {
                      const protocol = row.ssl ? 'https' : 'http'
                      window.open(`${protocol}://${row.domains[0]}`, '_blank')
                    }
                  },
                  [h(TheIcon, { icon: 'mdi:link-variant', size: 16 })]
                ),
              default: () =>
                h(
                  NFlex,
                  { vertical: true, size: 'small' },
                  {
                    default: () =>
                      row.domains.map((domain: string) => {
                        const protocol = row.ssl ? 'https' : 'http'
                        const url = `${protocol}://${domain}`
                        return h(
                          NFlex,
                          { align: 'center', size: 'small' },
                          {
                            default: () => [
                              h(
                                'a',
                                {
                                  href: url,
                                  target: '_blank',
                                  class: 'hover:underline'
                                },
                                url
                              ),
                              h(
                                'span',
                                {
                                  class: 'cursor-pointer hover:opacity-60 ml-1 inline-flex',
                                  onClick: () => {
                                    copy2clipboard(url).then(() => {
                                      window.$message.success($gettext('Copied'))
                                    })
                                  }
                                },
                                [h(TheIcon, { icon: 'mdi:content-copy', size: 14 })]
                              )
                            ]
                          }
                        )
                      })
                  }
                )
            }
          )
        )
      }
      return h(NFlex, { align: 'center', wrap: false }, { default: () => elements })
    }
  },
  {
    title: $gettext('Website Type'),
    key: 'type',
    width: 150,
    resizable: true,
    render(row: any) {
      const typeMap: any = {
        proxy: { label: $gettext('Reverse Proxy'), type: 'warning' },
        php: { label: $gettext('PHP'), type: 'info' },
        static: { label: $gettext('Pure Static'), type: 'success' }
      }
      const config = typeMap[row.type] || { label: row.type, type: 'default' }
      return h(NTag, { type: config.type }, { default: () => config.label })
    }
  },
  {
    title: $gettext('Running'),
    key: 'status',
    width: 150,
    render(row: any) {
      return h(NSwitch, {
        size: 'small',
        rubberBand: false,
        value: row.status,
        onUpdateValue: () => handleStatusChange(row)
      })
    }
  },
  {
    title: $gettext('Directory'),
    key: 'path',
    minWidth: 200,
    resizable: true,
    render(row: any) {
      return h(
        NTag,
        {
          class: 'cursor-pointer hover:opacity-60',
          type: 'info',
          onClick: () => {
            fileStore.path = row.path
            router.push({ name: 'file-index' })
          }
        },
        { default: () => row.path }
      )
    }
  },
  {
    title: 'HTTPS',
    key: 'ssl',
    width: 150,
    render(row: any) {
      return h(NSwitch, {
        size: 'small',
        rubberBand: false,
        value: row.ssl,
        onClick: () => handleEdit(row)
      })
    }
  },
  {
    title: $gettext('Certificate expiration'),
    key: 'cert_expire',
    width: 200,
    render(row: any) {
      return h(
        NTag,
        {
          type: row.cert_expire == 0 ? 'default' : row.cert_expire > 0 ? 'success' : 'error',
          class: 'cursor-pointer hover:opacity-60',
          onClick: () => handleEdit(row)
        },
        {
          default: () => {
            if (row.cert_expire == 0) {
              return $gettext('Not configured')
            }
            if (row.cert_expire < 0) {
              return $gettext('Expired %{ days } days ago', {
                days: Math.abs(row.cert_expire)
              })
            }
            if (row.cert_expire > 0) {
              return $gettext('Expires in %{ days } days', {
                days: row.cert_expire
              })
            }
          }
        }
      )
    }
  },
  {
    title: $gettext('Remark'),
    key: 'remark',
    minWidth: 200,
    resizable: true,
    render(row: any) {
      return h(NInput, {
        size: 'small',
        class: 'w-full',
        value: row.remark,
        onBlur: () => handleRemark(row),
        onUpdateValue(v) {
          row.remark = v
        }
      })
    }
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 220,
    hideInExcel: true,
    render(row: any) {
      return [
        h(
          NButton,
          {
            size: 'small',
            type: 'primary',
            style: 'margin-left: 15px;',
            onClick: () => handleEdit(row)
          },
          {
            default: () => $gettext('Edit')
          }
        ),
        h(
          DeleteConfirm,
          {
            showIcon: false,
            onPositiveClick: () => handleDelete(row.id)
          },
          {
            default: () => {
              return h(
                NFlex,
                {
                  vertical: true
                },
                {
                  default: () => [
                    h(
                      'strong',
                      {},
                      {
                        default: () =>
                          $gettext('Are you sure you want to delete website %{ name }?', {
                            name: row.name
                          })
                      }
                    ),
                    h(
                      NCheckbox,
                      {
                        checked: deleteModel.value.path,
                        onUpdateChecked: (v) => (deleteModel.value.path = v)
                      },
                      { default: () => $gettext('Delete website directory') }
                    ),
                    h(
                      NCheckbox,
                      {
                        checked: deleteModel.value.db,
                        onUpdateChecked: (v) => (deleteModel.value.db = v)
                      },
                      { default: () => $gettext('Delete local database with the same name') }
                    )
                  ]
                }
              )
            },
            trigger: () => {
              return h(
                NButton,
                {
                  size: 'small',
                  type: 'error',
                  style: 'margin-left: 15px;'
                },
                {
                  default: () => $gettext('Delete')
                }
              )
            }
          }
        )
      ]
    }
  }
]

const deleteModel = ref({
  path: true,
  db: false
})

const { loading, data, page, total, pageSize, pageCount, refresh } = usePagination(
  (page, pageSize) => website.list(type.value, page, pageSize),
  {
    initialData: { total: 0, list: [] },
    initialPageSize: 20,
    total: (res: any) => res.total,
    data: (res: any) => res.items
  }
)

// 修改运行状态
const handleStatusChange = (row: any) => {
  if (isNullOrUndef(row.id)) return

  useRequest(website.status(row.id, !row.status)).onSuccess(() => {
    row.status = !row.status
    if (row.status) {
      window.$message.success($gettext('Started successfully'))
    } else {
      window.$message.success($gettext('Stopped successfully'))
    }
  })
}

const handleRemark = (row: any) => {
  useRequest(website.updateRemark(row.id, row.remark)).onSuccess(() => {
    window.$message.success($gettext('Modified successfully'))
  })
}

const handleEdit = (row: any) => {
  router.push({
    name: 'website-edit',
    params: {
      id: row.id
    }
  })
}

const handleDelete = (id: number) => {
  useRequest(website.delete(id, deleteModel.value.path, deleteModel.value.db)).onSuccess(() => {
    refresh()
    deleteModel.value.path = true
    window.$message.success($gettext('Deleted successfully'))
  })
}

const bulkDelete = async () => {
  const promises = selectedRowKeys.value.map((id: any) => website.delete(id, true, false))
  await Promise.all(promises)

  selectedRowKeys.value = []
  refresh()
  window.$message.success($gettext('Deleted successfully'))
}

watch(type, () => {
  refresh()
})

onMounted(() => {
  refresh()
  window.$bus.on('website:refresh', refresh)
})
</script>

<template>
  <n-flex vertical>
    <n-flex>
      <n-button type="primary" @click="createModal = true">
        {{ $gettext('Create Website') }}
      </n-button>
      <n-button type="primary" @click="bulkCreateModal = true">
        {{ $gettext('Bulk Create Website') }}
      </n-button>
      <delete-confirm @positive-click="bulkDelete">
        <template #trigger>
          <n-button type="error" :disabled="selectedRowKeys.length === 0" ghost>
            {{ $gettext('Delete') }}
          </n-button>
        </template>
        {{
          $gettext(
            'This will delete the website directory but not the database with the same name. Are you sure you want to delete the selected websites?'
          )
        }}
      </delete-confirm>
    </n-flex>
    <n-data-table
      striped
      remote
      :loading="loading"
      :scroll-x="1500"
      :columns="columns"
      :data="data"
      :row-key="(row: any) => row.id"
      v-model:checked-row-keys="selectedRowKeys"
      v-model:page="page"
      v-model:pageSize="pageSize"
      :pagination="{
        page: page,
        pageCount: pageCount,
        pageSize: pageSize,
        itemCount: total,
        showQuickJumper: true,
        showSizePicker: true,
        pageSizes: [20, 50, 100, 200]
      }"
    />
  </n-flex>
</template>
