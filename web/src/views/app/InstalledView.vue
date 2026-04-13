<script setup lang="ts">
import { NButton, NDataTable, NFlex, NPopconfirm, NSwitch, NH3 } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import app from '@/api/panel/app'
import environment from '@/api/panel/environment'
import { router } from '@/router'
import { renderLocalIcon } from '@/utils'

const { $gettext } = useGettext()

// 应用表格列
const appColumns: any = [
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
        onUpdateValue: () => handleAppShowChange(row)
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
          row.update_exist
            ? h(
                NPopconfirm,
                {
                  onPositiveClick: () => handleAppUpdate(row.slug)
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
          h(
            NButton,
            {
              size: 'small',
              type: 'info',
              onClick: () => handleAppManage(row.slug)
            },
            {
              default: () => $gettext('Manage')
            }
          ),
          h(
            NPopconfirm,
            {
              onPositiveClick: () => handleAppUninstall(row.slug)
            },
            {
              default: () => {
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
        ]
      })
    }
  }
]

// 环境表格列
const envColumns: any = [
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
          row.has_update
            ? h(
                NPopconfirm,
                {
                  onPositiveClick: () => handleEnvUpdate(row.type, row.slug)
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
          h(
            NButton,
            {
              size: 'small',
              type: 'info',
              onClick: () => handleEnvManage(row.type, row.slug)
            },
            {
              default: () => $gettext('Manage')
            }
          ),
          h(
            NPopconfirm,
            {
              onPositiveClick: () => handleEnvUninstall(row.type, row.slug)
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
        ]
      })
    }
  }
]

// 获取已安装应用
const {
  loading: appLoading,
  data: appData,
  page: appPage,
  total: appTotal,
  pageSize: appPageSize,
  pageCount: appPageCount,
  refresh: appRefresh
} = usePagination(
  (page, pageSize) => app.list(page, pageSize, undefined, undefined, true),
  {
    initialData: { total: 0, list: [] },
    initialPageSize: 20,
    total: (res: any) => res.total,
    data: (res: any) => res.items
  }
)

// 获取已安装环境
const {
  loading: envLoading,
  data: envData,
  send: envRefresh
} = useRequest(() => environment.list(1, 1000, undefined, undefined, true), {
  initialData: []
})

// 应用操作
const handleAppShowChange = (row: any) => {
  useRequest(app.updateShow(row.slug, !row.show)).onSuccess(() => {
    row.show = !row.show
    window.$message.success($gettext('Setup successfully'))
  })
}

const handleAppUpdate = (slug: string) => {
  useRequest(app.update(slug)).onSuccess(() => {
    window.$message.success(
      $gettext('Task submitted, please check the progress in background tasks')
    )
  })
}

const handleAppUninstall = (slug: string) => {
  useRequest(app.uninstall(slug)).onSuccess(() => {
    window.$message.success(
      $gettext('Task submitted, please check the progress in background tasks')
    )
  })
}

const handleAppManage = (slug: string) => {
  router.push({ name: 'apps-' + slug + '-index' })
}

// 环境操作
const handleEnvUpdate = (type: string, slug: string) => {
  useRequest(environment.update(type, slug)).onSuccess(() => {
    window.$message.success(
      $gettext('Task submitted, please check the progress in background tasks')
    )
  })
}

const handleEnvUninstall = (type: string, slug: string) => {
  useRequest(environment.uninstall(type, slug)).onSuccess(() => {
    window.$message.success(
      $gettext('Task submitted, please check the progress in background tasks')
    )
  })
}

const handleEnvManage = (type: string, slug: string) => {
  router.push({ name: 'environment-' + type, params: { slug } })
}

onMounted(() => {
  appRefresh()
  envRefresh()
})
</script>

<template>
  <n-flex vertical :size="24">
    <!-- 已安装应用 -->
    <n-flex vertical>
      <n-h3 prefix="bar">{{ $gettext('Native App') }}</n-h3>
      <n-data-table
        striped
        remote
        :scroll-x="1200"
        :loading="appLoading"
        :columns="appColumns"
        :data="appData"
        :row-key="(row: any) => row.slug"
        v-model:page="appPage"
        v-model:pageSize="appPageSize"
        :pagination="{
          page: appPage,
          pageCount: appPageCount,
          pageSize: appPageSize,
          itemCount: appTotal,
          showQuickJumper: true,
          showSizePicker: true,
          pageSizes: [20, 50, 100, 200]
        }"
      />
    </n-flex>
    <!-- 已安装环境 -->
    <n-flex vertical>
      <n-h3 prefix="bar">{{ $gettext('Operating Environment') }}</n-h3>
      <n-data-table
        striped
        :scroll-x="1000"
        :loading="envLoading"
        :columns="envColumns"
        :data="envData"
        :row-key="(row: any) => row.slug"
      />
    </n-flex>
  </n-flex>
</template>
