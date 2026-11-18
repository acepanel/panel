import { NButton, NFlex, NSwitch, NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import app from '@/api/panel/app'
import AppButton from '@/components/system/AppButton.vue'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { router } from '@/router'
import { renderLocalIcon } from '@/utils'

// 应用列表与已安装列表共用的表格列
export function useAppColumns(
  options: { showStatus?: boolean; onInstall?: (row: any) => void } = {},
) {
  const { $gettext } = useGettext()
  const { confirmDelete, confirmAction } = useConfirm()

  // 运行状态映射，无运行状态概念的应用（如挂载工具）不在此列，渲染为 -
  const statusMap: Record<string, { type: 'success' | 'error' | 'warning'; label: string }> = {
    running: { type: 'success', label: $gettext('Running') },
    stopped: { type: 'error', label: $gettext('Stopped') },
    partial: { type: 'warning', label: $gettext('Partial') },
  }

  const switching = reactive(new Set<string>())

  const handleShowChange = (row: any) => {
    switching.add(row.slug)
    useRequest(app.updateShow(row.slug, !row.show))
      .onSuccess(() => {
        row.show = !row.show
        window.$message.success($gettext('Setup successfully'))
      })
      .onComplete(() => switching.delete(row.slug))
  }

  const handleUpdate = async (slug: string) => {
    await app.update(slug)
    window.$message.success(
      $gettext('Task submitted, please check the progress in background tasks'),
    )
  }

  const handleUninstall = async (slug: string) => {
    await app.uninstall(slug)
    window.$message.success(
      $gettext('Task submitted, please check the progress in background tasks'),
    )
  }

  const handleManage = (slug: string) => {
    router.push({ name: 'apps-' + slug + '-index' })
  }

  const statusColumn = {
    title: $gettext('Status'),
    key: 'status',
    width: 120,
    render(row: any) {
      const meta = statusMap[row.status]
      if (!meta) return '-'
      return h(NTag, { type: meta.type, size: 'small', round: true }, { default: () => meta.label })
    },
  }

  const columns: any = [
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
    ...(options.showStatus ? [statusColumn] : []),
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
          onUpdateValue: () => handleShowChange(row),
        })
      },
    },
    {
      title: $gettext('Actions'),
      key: 'actions',
      width: 430,
      hideInExcel: true,
      render(row: any) {
        return h(NFlex, null, {
          default: () => {
            const items: any[] = []
            if (row.installed && row.update_exist) {
              const targetVersion =
                row.channels?.find((ch: any) => ch.slug === row.installed_channel)?.version ?? ''
              items.push(
                h(
                  AppButton,
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
                      if (ok) await handleUpdate(row.slug)
                    },
                  },
                  { default: () => $gettext('Update') },
                ),
              )
            }
            if (row.installed) {
              items.push(
                h(
                  NButton,
                  {
                    size: 'small',
                    type: 'info',
                    onClick: () => handleManage(row.slug),
                  },
                  { default: () => $gettext('Manage') },
                ),
                h(
                  AppButton,
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
                      if (ok) await handleUninstall(row.slug)
                    },
                  },
                  { default: () => $gettext('Uninstall') },
                ),
              )
            } else {
              items.push(
                h(
                  NButton,
                  {
                    size: 'small',
                    type: 'success',
                    onClick: () => options.onInstall?.(row),
                  },
                  { default: () => $gettext('Install') },
                ),
              )
            }
            return items
          },
        })
      },
    },
  ]

  return columns
}
