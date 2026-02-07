<script setup lang="ts">
import user from '@/api/panel/user'
import { formatDateTime } from '@/utils'
import copy2clipboard from '@vavt/copy2clipboard'
import { NAlert, NButton, NDataTable, NFlex, NInput, NPopconfirm } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

const { $gettext } = useGettext()
const show = defineModel<boolean>('show', { type: Boolean, required: true })
const id = defineModel<number>('id', { type: Number, required: true })

const createModal = ref(false)
const updateModal = ref(false)
const createLoading = ref(false)
const updateLoading = ref(false)

const currentID = ref(0)
const createModel = ref({
  ips: [] as Array<string>,
  expired_at: new Date().getTime() + 31536000 * 1000 // 1 year
})
const updateModel = ref({
  ips: [] as Array<string>,
  expired_at: new Date().getTime() + 31536000 * 1000 // 1 year
})

const columns: any = [
  {
    title: $gettext('ID'),
    key: 'id',
    width: 100,
    resizable: true,
    ellipsis: { tooltip: true }
  },
  {
    title: $gettext('Creation Time'),
    key: 'created_at',
    minWidth: 200,
    ellipsis: { tooltip: true },
    render(row: any) {
      return formatDateTime(row.created_at)
    }
  },
  {
    title: $gettext('Expiration Time'),
    key: 'expired_at',
    minWidth: 200,
    ellipsis: { tooltip: true },
    render(row: any) {
      return formatDateTime(row.expired_at)
    }
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 260,
    hideInExcel: true,
    render(row: any) {
      return [
        h(
          NButton,
          {
            size: 'small',
            type: 'primary',
            onClick: () => {
              currentID.value = row.id
              updateModal.value = true
            }
          },
          {
            default: () => $gettext('Modify')
          }
        ),
        h(
          NPopconfirm,
          {
            style: 'margin-left: 15px;',
            onPositiveClick: () => handleDelete(row.id)
          },
          {
            default: () => {
              return $gettext('Are you sure you want to delete this access token?')
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

const { loading, data, page, total, pageSize, pageCount, refresh } = usePagination(
  (page, pageSize) => user.tokenList(id.value, page, pageSize),
  {
    initialData: { total: 0, list: [] },
    initialPageSize: 20,
    total: (res: any) => res.total,
    data: (res: any) => res.items
  }
)

const handleDelete = (id: number) => {
  useRequest(() => user.tokenDelete(id)).onSuccess(() => {
    window.$message.success($gettext('Deleted successfully'))
    refresh()
  })
}

const handleCreate = () => {
  createLoading.value = true
  useRequest(() =>
    user.tokenCreate(id.value, createModel.value.ips, createModel.value.expired_at)
  ).onSuccess(({ data }) => {
    createModal.value = false
    window.$dialog.success({
      title: $gettext('Created successfully'),
      content: () => {
        return [
          h(
            NFlex,
            {
              vertical: true
            },
            {
              default: () => [
                h(
                  NAlert,
                  {
                    type: 'warning'
                  },
                  {
                    default: () =>
                      $gettext(
                        'Token is only displayed once, please save it before closing the dialog.'
                      )
                  }
                ),
                h(NInput, {
                  value: data.token,
                  type: 'password',
                  showPasswordOn: 'click',
                  readonly: true
                })
              ]
            }
          )
        ]
      },
      maskClosable: false,
      positiveText: $gettext('Copy and close'),
      onPositiveClick: () => {
        copy2clipboard(data.token)
          .then(() => {
            window.$message.success($gettext('Copied successfully'))
          })
          .catch(() => {
            window.$message.error($gettext('Copy failed'))
          })
          .finally(() => {
            createModal.value = false
          })
      }
    })
    refresh()
  })
    .onComplete(() => {
      createLoading.value = false
    })
}

const handleUpdate = () => {
  updateLoading.value = true
  useRequest(() =>
    user.tokenUpdate(currentID.value, updateModel.value.ips, updateModel.value.expired_at)
  ).onSuccess(() => {
    window.$message.success($gettext('Updated successfully'))
    updateModal.value = false
    refresh()
  })
    .onComplete(() => {
      updateLoading.value = false
    })
}

watch(
  () => show.value,
  (val) => {
    if (val) {
      refresh()
    }
  },
  { immediate: true }
)
</script>

<template>
  <n-modal
    v-model:show="show"
    preset="card"
    :title="$gettext('Access Tokens')"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
    @close="show = false"
  >
    <n-flex vertical>
      <n-flex>
        <n-button type="primary" @click="createModal = true">
          {{ $gettext('Create Access Token') }}
        </n-button>
      </n-flex>
      <n-data-table
        striped
        remote
        :scroll-x="600"
        :loading="loading"
        :columns="columns"
        :data="data"
        :row-key="(row: any) => row.name"
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
  </n-modal>
  <n-modal
    v-model:show="createModal"
    preset="card"
    :title="$gettext('Create Access Token')"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
    @close="createModal = false"
  >
    <n-flex vertical>
      <n-form>
        <n-form-item :label="$gettext('IP White List')">
          <n-dynamic-input
            v-model:value="createModel.ips"
            :placeholder="$gettext('127.0.0.1')"
            show-sort-button
          />
        </n-form-item>
        <n-form-item :label="$gettext('Expiration Time')">
          <n-date-picker
            v-model:value="createModel.expired_at"
            type="datetime"
            :placeholder="$gettext('Please select the expiration time')"
            w-full
          />
        </n-form-item>
      </n-form>
      <n-button type="primary" :loading="createLoading" :disabled="createLoading" @click="handleCreate">
        {{ $gettext('Create') }}
      </n-button>
    </n-flex>
  </n-modal>
  <n-modal
    v-model:show="updateModal"
    preset="card"
    :title="$gettext('Modify Access Token')"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
    @close="updateModal = false"
  >
    <n-flex vertical>
      <n-form>
        <n-form-item :label="$gettext('IP White List')">
          <n-dynamic-input
            v-model:value="updateModel.ips"
            :placeholder="$gettext('127.0.0.1')"
            show-sort-button
          />
        </n-form-item>
        <n-form-item :label="$gettext('Expiration Time')">
          <n-date-picker
            v-model:value="updateModel.expired_at"
            type="datetime"
            :placeholder="$gettext('Please select the expiration time')"
            w-full
          />
        </n-form-item>
      </n-form>
      <n-button type="primary" :loading="updateLoading" :disabled="updateLoading" @click="handleUpdate">
        {{ $gettext('Update') }}
      </n-button>
    </n-flex>
  </n-modal>
</template>

<style scoped lang="scss"></style>
