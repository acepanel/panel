import { NButton, NFlex } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import environment from '@/api/panel/environment'
import AppButton from '@/components/system/AppButton.vue'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { router } from '@/router'
import { renderLocalIcon } from '@/utils'

// 环境列表与已安装列表共用的表格列
export function useEnvColumns(
  options: { showLatest?: boolean; onInstall?: (row: any) => void | Promise<void> } = {},
) {
  const { $gettext } = useGettext()
  const { confirmDelete, confirmAction } = useConfirm()

  const handleUpdate = async (type: string, slug: string) => {
    await environment.update(type, slug)
    window.$message.success(
      $gettext('Task submitted, please check the progress in background tasks'),
    )
  }

  const handleUninstall = async (type: string, slug: string) => {
    await environment.uninstall(type, slug)
    window.$message.success(
      $gettext('Task submitted, please check the progress in background tasks'),
    )
  }

  const handleManage = (type: string, slug: string) => {
    router.push({ name: 'environment-' + type, params: { slug } })
  }

  const latestColumn = {
    title: $gettext('Latest Version'),
    key: 'version',
    width: 160,
    ellipsis: { tooltip: true },
  }

  const columns: any = [
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
    ...(options.showLatest ? [latestColumn] : []),
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
            if (row.installed && row.has_update) {
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
                        content: $gettext('Are you sure to update environment %{ environment }?', {
                          environment: row.name,
                        }),
                      })
                      if (ok) await handleUpdate(row.type, row.slug)
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
                    onClick: () => handleManage(row.type, row.slug),
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
                        content: $gettext(
                          'Are you sure to uninstall environment %{ environment }?',
                          { environment: row.name },
                        ),
                        positiveText: $gettext('Uninstall'),
                        countdown: 5,
                      })
                      if (ok) await handleUninstall(row.type, row.slug)
                    },
                  },
                  { default: () => $gettext('Uninstall') },
                ),
              )
            } else {
              items.push(
                h(
                  AppButton,
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
