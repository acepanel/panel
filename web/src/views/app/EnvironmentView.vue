<script setup lang="ts">
import environment from '@/api/panel/environment'
import { router } from '@/router'
import { renderLocalIcon } from '@/utils'
import { NButton, NDataTable, NFlex, NInput, NPopconfirm, NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

const { $gettext } = useGettext()

const selectedType = ref<string>('')
const searchQuery = ref<string>('')

const { data: types } = useRequest(environment.types, {
  initialData: []
})

const columns: any = [
  {
    key: 'icon',
    fixed: 'left',
    width: 80,
    align: 'center',
    render(row: any) {
      return renderLocalIcon('environment', row.type, { size: 26 })()
    }
  },
  {
    title: $gettext('Name'),
    key: 'name',
    width: 200,
    ellipsis: { tooltip: true }
  },
  {
    title: $gettext('Description'),
    key: 'description',
    minWidth: 300,
    ellipsis: { tooltip: true }
  },
  {
    title: $gettext('Latest Version'),
    key: 'version',
    width: 160,
    ellipsis: { tooltip: true }
  },
  {
    title: $gettext('Installed Version'),
    key: 'installed_version',
    width: 160,
    ellipsis: { tooltip: true }
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 240,
    hideInExcel: true,
    render(row: any) {
      return h(NFlex, null, {
        default: () => [
          row.installed && row.has_update
            ? h(
                NPopconfirm,
                {
                  onPositiveClick: () => handleUpdate(row.type, row.slug)
                },
                {
                  default: () => {
                    return $gettext('Are you sure to update environment %{ environment }?', {
                      environment: row.name
                    })
                  },
                  trigger: () => {
                    return h(
                      NButton,
                      {
                        size: 'small',
                        type: 'warning'
                      },
                      {
                        default: () => $gettext('Update')
                      }
                    )
                  }
                }
              )
            : null,
          row.installed
            ? h(
                NButton,
                {
                  size: 'small',
                  type: 'info',
                  onClick: () => handleManage(row.type, row.slug)
                },
                {
                  default: () => $gettext('Manage')
                }
              )
            : null,
          row.installed
            ? h(
                NPopconfirm,
                {
                  onPositiveClick: () => handleUninstall(row.type, row.slug)
                },
                {
                  default: () => {
                    return $gettext('Are you sure to uninstall environment %{ environment }?', {
                      environment: row.name
                    })
                  },
                  trigger: () => {
                    return h(
                      NButton,
                      {
                        size: 'small',
                        type: 'error'
                      },
                      {
                        default: () => $gettext('Uninstall')
                      }
                    )
                  }
                }
              )
            : null,
          !row.installed
            ? h(
                NPopconfirm,
                {
                  onPositiveClick: () => handleInstall(row.type, row.slug)
                },
                {
                  default: () => {
                    return $gettext('Are you sure to install environment %{ environment }?', {
                      environment: row.name
                    })
                  },
                  trigger: () => {
                    return h(
                      NButton,
                      {
                        size: 'small',
                        type: 'success'
                      },
                      {
                        default: () => $gettext('Install')
                      }
                    )
                  }
                }
              )
            : null
        ]
      })
    }
  }
]

const { loading, data, page, total, pageSize, pageCount, refresh } = usePagination(
  (page, pageSize) =>
    environment.list(page, pageSize, selectedType.value || undefined, searchQuery.value || undefined),
  {
    initialData: { total: 0, list: [] },
    initialPageSize: 20,
    total: (res: any) => res.total,
    data: (res: any) => res.items,
    watchingStates: [selectedType, searchQuery]
  }
)

// 处理类型切换
const handleTypeChange = (type: string) => {
  selectedType.value = type
  page.value = 1
}

const handleInstall = (type: string, slug: string) => {
  useRequest(environment.install(type, slug)).onSuccess(() => {
    window.$message.success(
      $gettext('Task submitted, please check the progress in background tasks')
    )
  })
}

const handleUpdate = (type: string, slug: string) => {
  useRequest(environment.update(type, slug)).onSuccess(() => {
    window.$message.success(
      $gettext('Task submitted, please check the progress in background tasks')
    )
  })
}

const handleUninstall = (type: string, slug: string) => {
  useRequest(environment.uninstall(type, slug)).onSuccess(() => {
    window.$message.success(
      $gettext('Task submitted, please check the progress in background tasks')
    )
  })
}

const handleManage = (type: string, slug: string) => {
  router.push({ name: 'environment-' + type, params: { slug } })
}

onMounted(() => {
  refresh()
})
</script>

<template>
  <n-flex vertical>
    <n-flex justify="space-between">
      <n-flex>
        <n-tag
          :type="selectedType === '' ? 'primary' : 'default'"
          :bordered="selectedType !== ''"
          style="cursor: pointer"
          @click="handleTypeChange('')"
        >
          {{ $gettext('All') }}
        </n-tag>
        <n-tag
          v-for="type in types"
          :key="type.value"
          :type="selectedType === type.value ? 'primary' : 'default'"
          :bordered="selectedType !== type.value"
          style="cursor: pointer"
          @click="handleTypeChange(type.value)"
        >
          {{ type.label }}
        </n-tag>
      </n-flex>
      <n-input
        v-model:value="searchQuery"
        :placeholder="$gettext('Search')"
        clearable
        style="width: 240px"
      />
    </n-flex>
    <n-data-table
      striped
      remote
      :scroll-x="1200"
      :loading="loading"
      :columns="columns"
      :data="data"
      :row-key="(row: any) => row.slug"
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

<style scoped lang="scss"></style>
