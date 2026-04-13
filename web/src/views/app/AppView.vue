<script setup lang="ts">
defineOptions({
  name: 'app-index'
})

import { NButton, NDataTable, NFlex, NInput, NPopconfirm, NSwitch, NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import app from '@/api/panel/app'
import { router } from '@/router'
import { renderLocalIcon } from '@/utils'
import VersionModal from '@/views/app/VersionModal.vue'

const { $gettext } = useGettext()

const versionModalShow = ref(false)
const versionModalOperation = ref($gettext('Install'))
const versionModalInfo = ref<any>({})

// 当前选中的分类
const selectedCategory = ref<string>('')
const searchQuery = ref<string>('')

const columns: any = [
  {
    key: 'icon',
    fixed: 'left',
    width: 80,
    align: 'center',
    render(row: any) {
      return renderLocalIcon('app', row.slug, { size: 26 })()
    }
  },
  {
    title: $gettext('App Name'),
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
    title: $gettext('Installed Version'),
    key: 'installed_version',
    width: 160,
    ellipsis: { tooltip: true }
  },
  {
    title: $gettext('Show in Home'),
    key: 'show',
    width: 140,
    render(row: any) {
      return h(NSwitch, {
        size: 'small',
        rubberBand: false,
        value: row.show,
        onUpdateValue: () => handleShowChange(row)
      })
    }
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 350,
    hideInExcel: true,
    render(row: any) {
      return h(NFlex, null, {
        default: () => [
          row.installed && row.update_exist
            ? h(
                NPopconfirm,
                {
                  onPositiveClick: () => handleUpdate(row.slug)
                },
                {
                  default: () => {
                    const targetVersion =
                      row.channels?.find((ch: any) => ch.slug === row.installed_channel)
                        ?.version ?? ''
                    return $gettext(
                      'Updating app %{ app } to %{ version } may reset related configurations to default state, are you sure to continue?',
                      { app: row.name, version: targetVersion }
                    )
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
                  onClick: () => handleManage(row.slug)
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
                  onPositiveClick: () => handleUninstall(row.slug)
                },
                {
                  default: () => {
                    // Web 服务器卸载时的提示信息
                    if (row.categories.includes('webserver')) {
                      return $gettext(
                        'Reinstalling/Switching to a different web server will reset the configuration of all websites, are you sure to continue?'
                      )
                    }
                    return $gettext('Are you sure to uninstall app %{ app }?', { app: row.name })
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
                NButton,
                {
                  size: 'small',
                  type: 'success',
                  onClick: () => {
                    versionModalShow.value = true
                    versionModalOperation.value = $gettext('Install')
                    versionModalInfo.value = row
                  }
                },
                {
                  default: () => $gettext('Install')
                }
              )
            : null
        ]
      })
    }
  }
]

const { data: categories } = useRequest(app.categories, {
  initialData: []
})

const { loading, data, page, total, pageSize, pageCount, refresh } = usePagination(
  (page, pageSize) =>
    app.list(page, pageSize, selectedCategory.value || undefined, searchQuery.value || undefined),
  {
    initialData: { total: 0, list: [] },
    initialPageSize: 20,
    total: (res: any) => res.total,
    data: (res: any) => res.items,
    watchingStates: [selectedCategory, searchQuery]
  }
)

// 处理分类切换
const handleCategoryChange = (category: string) => {
  selectedCategory.value = category
  page.value = 1
}

const handleShowChange = (row: any) => {
  useRequest(app.updateShow(row.slug, !row.show)).onSuccess(() => {
    row.show = !row.show
    window.$message.success($gettext('Setup successfully'))
  })
}

const handleUpdate = (slug: string) => {
  useRequest(app.update(slug)).onSuccess(() => {
    window.$message.success(
      $gettext('Task submitted, please check the progress in background tasks')
    )
  })
}

const handleUninstall = (slug: string) => {
  useRequest(app.uninstall(slug)).onSuccess(() => {
    window.$message.success(
      $gettext('Task submitted, please check the progress in background tasks')
    )
  })
}

const handleManage = (slug: string) => {
  router.push({ name: 'apps-' + slug + '-index' })
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
          :type="selectedCategory === '' ? 'primary' : 'default'"
          :bordered="selectedCategory !== ''"
          style="cursor: pointer"
          @click="handleCategoryChange('')"
        >
          {{ $gettext('All') }}
        </n-tag>
        <n-tag
          v-for="cat in categories"
          :key="cat.value"
          :type="selectedCategory === cat.value ? 'primary' : 'default'"
          :bordered="selectedCategory !== cat.value"
          style="cursor: pointer"
          @click="handleCategoryChange(cat.value)"
        >
          {{ cat.label }}
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
  <version-modal
    v-model:show="versionModalShow"
    v-model:operation="versionModalOperation"
    v-model:info="versionModalInfo"
  />
</template>
