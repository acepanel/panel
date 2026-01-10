<script setup lang="ts">
import { NButton, NCheckbox, NDataTable, NFlex, NInput, NPopconfirm, NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import container from '@/api/panel/container'
import { useFileStore } from '@/store'
import { formatDateTime } from '@/utils'

const { $gettext } = useGettext()
const fileStore = useFileStore()
const router = useRouter()

const forcePull = ref(false)

const createModel = ref({
  name: '',
  compose: '',
  envs: []
})
const createModal = ref(false)

const selectedRowKeys = ref<any>([])

const updateModel = ref({
  name: '',
  compose: '',
  envs: []
})
const updateModal = ref(false)

const columns: any = [
  { type: 'selection', fixed: 'left' },
  {
    title: $gettext('Name'),
    key: 'name',
    minWidth: 150,
    resizable: true,
    ellipsis: { tooltip: true }
  },
  {
    title: $gettext('Directory'),
    key: 'path',
    minWidth: 150,
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
    title: $gettext('Status'),
    key: 'status',
    width: 250,
    resizable: true,
    ellipsis: { tooltip: true }
  },
  {
    title: $gettext('Creation Time'),
    key: 'created_at',
    width: 200,
    resizable: true,
    render(row: any) {
      return formatDateTime(row.created_at)
    }
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 280,
    hideInExcel: true,
    render(row: any) {
      return [
        h(
          NButton,
          {
            size: 'small',
            onClick: () => {
              useRequest(container.composeGet(row.name)).onSuccess(({ data }: { data: any }) => {
                updateModel.value = {
                  name: row.name,
                  compose: data.compose,
                  envs: data.envs
                }
                updateModal.value = true
              })
            }
          },
          {
            default: () => $gettext('Edit')
          }
        ),
        h(
          NPopconfirm,
          {
            showIcon: false,
            onPositiveClick: () => {
              const messageReactive = window.$message.loading($gettext('Starting...'), {
                duration: 0
              })
              useRequest(container.composeUp(row.name, forcePull.value))
                .onSuccess(() => {
                  refresh()
                  forcePull.value = false
                  window.$message.success($gettext('Start successful'))
                })
                .onComplete(() => {
                  messageReactive?.destroy()
                })
            }
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
                          $gettext(`Are you sure you want to start compose %{ name }?`, {
                            name: row.name
                          })
                      }
                    ),
                    h(
                      NCheckbox,
                      {
                        checked: forcePull.value,
                        onUpdateChecked: (v) => (forcePull.value = v)
                      },
                      { default: () => $gettext('Force pull images') }
                    )
                  ]
                }
              )
            },
            trigger: () => {
              return h(
                NButton,
                {
                  style: 'margin-left: 15px;',
                  size: 'small',
                  type: 'success'
                },
                {
                  default: () => $gettext('Start')
                }
              )
            }
          }
        ),
        h(
          NPopconfirm,
          {
            onPositiveClick: () => {
              const messageReactive = window.$message.loading($gettext('Stopping...'), {
                duration: 0
              })
              useRequest(container.composeDown(row.name))
                .onSuccess(() => {
                  refresh()
                  forcePull.value = false
                  window.$message.success($gettext('Stop successful'))
                })
                .onComplete(() => {
                  messageReactive?.destroy()
                })
            }
          },
          {
            default: () => {
              return $gettext(`Are you sure you want to stop compose %{ name }?`, {
                name: row.name
              })
            },
            trigger: () => {
              return h(
                NButton,
                {
                  style: 'margin-left: 15px;',
                  size: 'small',
                  type: 'warning'
                },
                {
                  default: () => $gettext('Stop')
                }
              )
            }
          }
        ),
        h(
          NPopconfirm,
          {
            onPositiveClick: () => {
              useRequest(container.composeRemove(row.name)).onSuccess(() => {
                refresh()
                window.$message.success($gettext('Delete successful'))
              })
            }
          },
          {
            default: () => {
              return $gettext(`Are you sure you want to delete compose %{ name }?`, {
                name: row.name
              })
            },
            trigger: () => {
              return h(
                NButton,
                {
                  style: 'margin-left: 15px;',
                  size: 'small',
                  type: 'error'
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

const { loading, data, page, total, pageSize, pageCount, refresh } = usePagination(
  (page, pageSize) => container.composeList(page, pageSize),
  {
    initialData: { total: 0, list: [] },
    initialPageSize: 20,
    total: (res: any) => res.total,
    data: (res: any) => res.items
  }
)

const handleCreate = () => {
  loading.value = true
  useRequest(container.composeCreate(createModel.value))
    .onSuccess(() => {
      refresh()
      window.$message.success($gettext('Created successfully'))
    })
    .onComplete(() => {
      loading.value = false
      createModal.value = false
      createModel.value = {
        name: '',
        compose: '',
        envs: []
      }
    })
}

const handleUpdate = () => {
  loading.value = true
  useRequest(container.composeUpdate(updateModel.value.name, updateModel.value))
    .onSuccess(() => {
      refresh()
      window.$message.success($gettext('Update successful'))
    })
    .onComplete(() => {
      loading.value = false
      updateModal.value = false
      updateModel.value = {
        name: '',
        compose: '',
        envs: []
      }
    })
}

const handleBatchDelete = async () => {
  const promises = selectedRowKeys.value.map((name: any) => container.composeRemove(name))
  await Promise.all(promises)

  selectedRowKeys.value = []
  refresh()
  window.$message.success($gettext('Delete successful'))
}

onMounted(() => {
  refresh()
})
</script>

<template>
  <n-flex vertical :size="20">
    <n-flex>
      <n-button type="primary" @click="createModal = true">
        {{ $gettext('Create Compose') }}
      </n-button>
      <n-popconfirm @positive-click="handleBatchDelete">
        <template #trigger>
          <n-button type="error" :disabled="selectedRowKeys.length === 0" ghost>
            {{ $gettext('Delete') }}
          </n-button>
        </template>
        {{ $gettext('Are you sure you want to delete the selected composes?') }}
      </n-popconfirm>
    </n-flex>
    <n-data-table
      striped
      remote
      :loading="loading"
      :scroll-x="1100"
      :data="data"
      :columns="columns"
      :row-key="(row: any) => row.name"
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
  <n-modal
    v-model:show="createModal"
    preset="card"
    :title="$gettext('Create Compose')"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
  >
    <n-form :model="createModel">
      <n-form-item path="name" :label="$gettext('Compose Name')">
        <n-input v-model:value="createModel.name" type="text" />
      </n-form-item>
      <n-form-item path="compose" :label="$gettext('Compose')">
        <common-editor v-model:value="createModel.compose" lang="yaml" height="40vh" />
      </n-form-item>
      <n-form-item path="envs" :label="$gettext('Environment Variables')">
        <n-dynamic-input
          v-model:value="createModel.envs"
          preset="pair"
          :key-placeholder="$gettext('Variable Name')"
          :value-placeholder="$gettext('Variable Value')"
        />
      </n-form-item>
    </n-form>
    <n-button type="info" block :loading="loading" :disabled="loading" @click="handleCreate">
      {{ $gettext('Submit') }}
    </n-button>
  </n-modal>
  <n-modal
    v-model:show="updateModal"
    preset="card"
    :title="$gettext('Edit Compose')"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
  >
    <n-form :model="updateModel">
      <n-form-item path="compose" :label="$gettext('Compose')">
        <common-editor v-model:value="updateModel.compose" lang="yaml" height="40vh" />
      </n-form-item>
      <n-form-item path="envs" :label="$gettext('Environment Variables')">
        <n-dynamic-input
          v-model:value="updateModel.envs"
          preset="pair"
          :key-placeholder="$gettext('Variable Name')"
          :value-placeholder="$gettext('Variable Value')"
        />
      </n-form-item>
    </n-form>
    <n-button type="info" block :loading="loading" :disabled="loading" @click="handleUpdate">
      {{ $gettext('Submit') }}
    </n-button>
  </n-modal>
</template>
