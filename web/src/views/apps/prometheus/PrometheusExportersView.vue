<script setup lang="ts">
defineOptions({
  name: 'prometheus-exporters',
})

import { NButton, NDataTable, NSpace, NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import prometheus from '@/api/apps/prometheus'
import { useConfirm } from '@/components/system/composables/useConfirm'

const { $gettext } = useGettext()
const { confirmDelete, confirmAction } = useConfirm()

const showConfigModal = ref(false)
const configSlug = ref('')
const configContent = ref('')
const saveConfigLoading = ref(false)

const { data: exporters, send: refreshExporters } = useRequest(prometheus.exporters, {
  initialData: [],
})

const columns: any = [
  {
    title: $gettext('Name'),
    key: 'name',
    minWidth: 200,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Description'),
    key: 'description',
    minWidth: 250,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Status'),
    key: 'status',
    width: 150,
    render(row: any) {
      if (!row.installed) {
        return h(
          NTag,
          { type: 'default', size: 'small' },
          { default: () => $gettext('Not Installed') },
        )
      }
      return row.running
        ? h(NTag, { type: 'success', size: 'small' }, { default: () => $gettext('Running') })
        : h(NTag, { type: 'warning', size: 'small' }, { default: () => $gettext('Stopped') })
    },
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 360,
    render(row: any) {
      const buttons: any[] = []

      if (!row.installed) {
        buttons.push(
          h(
            NButton,
            {
              size: 'small',
              type: 'info',
              onClick: async () => {
                const ok = await confirmAction({
                  type: 'info',
                  title: $gettext('Confirm Install'),
                  content: $gettext('Are you sure you want to install %{ name }?', {
                    name: row.name,
                  }),
                })
                if (ok) handleInstall(row.slug)
              },
            },
            { default: () => $gettext('Install') },
          ),
        )
      } else {
        if (!row.running) {
          buttons.push(
            h(
              NButton,
              { size: 'small', type: 'success', onClick: () => handleStart(row.slug) },
              { default: () => $gettext('Start') },
            ),
          )
        } else {
          buttons.push(
            h(
              NButton,
              { size: 'small', type: 'warning', onClick: () => handleStop(row.slug) },
              { default: () => $gettext('Stop') },
            ),
          )
          buttons.push(
            h(
              NButton,
              { size: 'small', onClick: () => handleRestart(row.slug) },
              { default: () => $gettext('Restart') },
            ),
          )
        }
        if (row.has_config) {
          buttons.push(
            h(
              NButton,
              { size: 'small', onClick: () => handleOpenConfig(row.slug) },
              { default: () => $gettext('Config') },
            ),
          )
        }
        buttons.push(
          h(
            NButton,
            {
              size: 'small',
              type: 'error',
              onClick: async () => {
                const ok = await confirmDelete({
                  title: $gettext('Confirm Uninstall'),
                  content: $gettext('Are you sure you want to uninstall %{ name }?', {
                    name: row.name,
                  }),
                  positiveText: $gettext('Uninstall'),
                })
                if (ok) handleUninstall(row.slug)
              },
            },
            { default: () => $gettext('Delete') },
          ),
        )
      }

      return h(NSpace, { size: 'small' }, { default: () => buttons })
    },
  },
]

const handleInstall = (slug: string) => {
  useRequest(prometheus.installExporter(slug)).onSuccess(() => {
    window.$message.success($gettext('Task submitted, please check progress in background tasks'))
    refreshExporters()
  })
}

const handleUninstall = (slug: string) => {
  useRequest(prometheus.uninstallExporter(slug)).onSuccess(() => {
    window.$message.success($gettext('Task submitted, please check progress in background tasks'))
    refreshExporters()
  })
}

const handleStart = (slug: string) => {
  useRequest(prometheus.startExporter(slug)).onSuccess(() => {
    window.$message.success($gettext('Started successfully'))
    refreshExporters()
  })
}

const handleStop = (slug: string) => {
  useRequest(prometheus.stopExporter(slug)).onSuccess(() => {
    window.$message.success($gettext('Stopped successfully'))
    refreshExporters()
  })
}

const handleRestart = (slug: string) => {
  useRequest(prometheus.restartExporter(slug)).onSuccess(() => {
    window.$message.success($gettext('Restarted successfully'))
    refreshExporters()
  })
}

const handleOpenConfig = (slug: string) => {
  configSlug.value = slug
  useRequest(prometheus.exporterConfig(slug)).onSuccess(({ data }: any) => {
    configContent.value = data
    showConfigModal.value = true
  })
}

const handleSaveConfig = () => {
  saveConfigLoading.value = true
  useRequest(prometheus.saveExporterConfig(configSlug.value, configContent.value))
    .onSuccess(() => {
      window.$message.success($gettext('Saved successfully'))
      showConfigModal.value = false
    })
    .onComplete(() => {
      saveConfigLoading.value = false
    })
}
</script>

<template>
  <n-flex vertical>
    <n-alert type="info">
      {{
        $gettext('Manage Prometheus exporters. Exporters collect metrics from various services.')
      }}
    </n-alert>
    <n-data-table striped :columns="columns" :data="exporters" :scroll-x="960" />
    <n-modal
      v-model:show="showConfigModal"
      preset="card"
      :title="$gettext('Exporter Configuration') + ' - ' + configSlug"
      class="w-200"
    >
      <n-flex vertical>
        <n-alert type="warning">
          {{ $gettext('Modifying the exporter configuration will restart the exporter service.') }}
        </n-alert>
        <common-editor v-model:value="configContent" height="40vh" />
        <n-flex>
          <n-button
            type="primary"
            :loading="saveConfigLoading"
            :disabled="saveConfigLoading"
            @click="handleSaveConfig"
          >
            {{ $gettext('Save') }}
          </n-button>
        </n-flex>
      </n-flex>
    </n-modal>
  </n-flex>
</template>
