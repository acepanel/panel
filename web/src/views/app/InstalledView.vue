<script setup lang="ts">
import { NButton, NDataTable, NFlex, NSwitch, NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import app from '@/api/panel/app'
import environment from '@/api/panel/environment'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { useTablePagination } from '@/components/system/composables/useTablePagination'
import { router } from '@/router'
import { renderLocalIcon } from '@/utils'

const { $gettext } = useGettext()
const { confirmDelete, confirmAction } = useConfirm()

// 运行状态映射，无运行状态概念的应用（如挂载工具）不在此列，渲染为 -
const statusMap: Record<string, { type: 'success' | 'error' | 'warning'; label: string }> = {
  running: { type: 'success', label: $gettext('Running') },
  stopped: { type: 'error', label: $gettext('Stopped') },
  partial: { type: 'warning', label: $gettext('Partial') },
}

// 应用表格列
const appColumns: any = [
  {
    key: 'icon',
    fixed: 'left',
    width: 80,
    align: 'center',
    render(row: any) {
      return renderLocalIcon('app', row.slug, { size: 26 })()
    },
  },
  {
    title: $gettext('App Name'),
    key: 'name',
    width: 200,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Description'),
    key: 'description',
    minWidth: 300,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Installed Version'),
    key: 'installed_version',
    width: 160,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Status'),
    key: 'status',
    width: 120,
    render(row: any) {
      const meta = statusMap[row.status]
      if (!meta) return '-'
      return h(NTag, { type: meta.type, size: 'small', round: true }, { default: () => meta.label })
    },
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
        loading: switching.has(row.slug),
        onUpdateValue: () => handleAppShowChange(row),
      })
    },
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 430,
    hideInExcel: true,
    render(row: any) {
      const targetVersion =
        row.channels?.find((ch: any) => ch.slug === row.installed_channel)?.version ?? ''
      return h(NFlex, null, {
        default: () => {
          const items: any[] = []
          if (row.update_exist) {
            items.push(
              h(
                NButton,
                {
                  size: 'small',
                  type: 'warning',
                  onClick: async () => {
                    const ok = await confirmAction({
                      type: 'warning',
                      title: $gettext('Confirm Update'),
                      content: $gettext(
                        'Updating app %{ app } to %{ version } may reset related configurations to default state, are you sure to continue?',
                        { app: row.name, version: targetVersion },
                      ),
                    })
                    if (ok) handleAppUpdate(row.slug)
                  },
                },
                { default: () => $gettext('Update') },
              ),
            )
          }
          items.push(
            h(
              NButton,
              {
                size: 'small',
                type: 'info',
                onClick: () => handleAppManage(row.slug),
              },
              { default: () => $gettext('Manage') },
            ),
            h(
              NButton,
              {
                size: 'small',
                type: 'error',
                onClick: async () => {
                  const ok = await confirmDelete({
                    title: $gettext('Confirm Uninstall'),
                    content: row.categories.includes('webserver')
                      ? $gettext(
                          'Reinstalling/Switching to a different web server will reset the configuration of all websites, are you sure to continue?',
                        )
                      : $gettext('Are you sure to uninstall app %{ app }?', { app: row.name }),
                    positiveText: $gettext('Uninstall'),
                    countdown: 5,
                  })
                  if (ok) handleAppUninstall(row.slug)
                },
              },
              { default: () => $gettext('Uninstall') },
            ),
          )
          return items
        },
      })
    },
  },
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
    },
  },
  {
    title: $gettext('Name'),
    key: 'name',
    width: 200,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Description'),
    key: 'description',
    minWidth: 300,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Installed Version'),
    key: 'installed_version',
    width: 160,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 320,
    hideInExcel: true,
    render(row: any) {
      return h(NFlex, null, {
        default: () => {
          const items: any[] = []
          if (row.has_update) {
            items.push(
              h(
                NButton,
                {
                  size: 'small',
                  type: 'warning',
                  onClick: async () => {
                    const ok = await confirmAction({
                      type: 'warning',
                      title: $gettext('Confirm Update'),
                      content: $gettext('Are you sure to update environment %{ environment }?', {
                        environment: row.name,
                      }),
                    })
                    if (ok) handleEnvUpdate(row.type, row.slug)
                  },
                },
                { default: () => $gettext('Update') },
              ),
            )
          }
          items.push(
            h(
              NButton,
              {
                size: 'small',
                type: 'info',
                onClick: () => handleEnvManage(row.type, row.slug),
              },
              { default: () => $gettext('Manage') },
            ),
            h(
              NButton,
              {
                size: 'small',
                type: 'error',
                onClick: async () => {
                  const ok = await confirmDelete({
                    title: $gettext('Confirm Uninstall'),
                    content: $gettext('Are you sure to uninstall environment %{ environment }?', {
                      environment: row.name,
                    }),
                    positiveText: $gettext('Uninstall'),
                    countdown: 5,
                  })
                  if (ok) handleEnvUninstall(row.type, row.slug)
                },
              },
              { default: () => $gettext('Uninstall') },
            ),
          )
          return items
        },
      })
    },
  },
]

// 获取已安装应用
const {
  loading: appLoading,
  data: appData,
  page: appPage,
  total: appTotal,
  pageSize: appPageSize,
  refresh: appRefresh,
} = usePagination((page, pageSize) => app.list(page, pageSize, undefined, undefined, true), {
  initialData: { total: 0, list: [] },
  initialPageSize: 20,
  total: (res: any) => res.total,
  data: (res: any) => res.items,
})
const appPagination = useTablePagination(appPage, appPageSize, appTotal)

// 获取已安装环境
const {
  loading: envLoading,
  data: envData,
  send: envRefresh,
} = useRequest(() => environment.list(1, 1000, undefined, undefined, true), {
  initialData: [],
})

// 应用操作
const switching = reactive(new Set<string>())

const handleAppShowChange = (row: any) => {
  switching.add(row.slug)
  useRequest(app.updateShow(row.slug, !row.show))
    .onSuccess(() => {
      row.show = !row.show
      window.$message.success($gettext('Setup successfully'))
    })
    .onComplete(() => switching.delete(row.slug))
}

const handleAppUpdate = (slug: string) => {
  useRequest(app.update(slug)).onSuccess(() => {
    window.$message.success(
      $gettext('Task submitted, please check the progress in background tasks'),
    )
  })
}

const handleAppUninstall = (slug: string) => {
  useRequest(app.uninstall(slug)).onSuccess(() => {
    window.$message.success(
      $gettext('Task submitted, please check the progress in background tasks'),
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
      $gettext('Task submitted, please check the progress in background tasks'),
    )
  })
}

const handleEnvUninstall = (type: string, slug: string) => {
  useRequest(environment.uninstall(type, slug)).onSuccess(() => {
    window.$message.success(
      $gettext('Task submitted, please check the progress in background tasks'),
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
        v-model:page="appPage"
        v-model:pageSize="appPageSize"
        striped
        remote
        :scroll-x="1500"
        :loading="appLoading"
        :columns="appColumns"
        :data="appData"
        :row-key="(row: any) => row.slug"
        :pagination="appPagination"
      />
    </n-flex>
    <!-- 已安装环境 -->
    <n-flex vertical>
      <n-h3 prefix="bar">{{ $gettext('Operating Environment') }}</n-h3>
      <n-data-table
        striped
        :scroll-x="1080"
        :loading="envLoading"
        :columns="envColumns"
        :data="envData"
        :row-key="(row: any) => row.slug"
      />
    </n-flex>
  </n-flex>
</template>
