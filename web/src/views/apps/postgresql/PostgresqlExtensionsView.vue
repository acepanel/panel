<script setup lang="ts">
defineOptions({
  name: 'postgresql-extensions',
})

import { NButton, NSpace, NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import postgresql from '@/api/apps/postgresql'
import { useConfirm } from '@/components/system/composables/useConfirm'

const { $gettext } = useGettext()
const { confirmDelete, confirmAction } = useConfirm()

const showEnableModal = ref(false)
const enableSlug = ref('')
const enableExtName = ref('')
const enableDatabase = ref('')
const enableLoading = ref(false)

const { data: extensions, send: refreshExtensions } = useRequest(postgresql.extensions, {
  initialData: [],
})
const { data: databases, send: fetchDatabases } = useRequest(postgresql.databases, {
  immediate: false,
  initialData: [],
})

const databaseOptions = computed(() =>
  (databases.value as string[]).map((name) => ({ label: name, value: name })),
)

const columns: any = [
  {
    title: $gettext('Name'),
    key: 'name',
    minWidth: 150,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Extension Name'),
    key: 'ext_name',
    minWidth: 150,
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
      const label = row.installed_version
        ? `${$gettext('Installed')} (${row.installed_version})`
        : $gettext('Installed')
      return h(NTag, { type: 'success', size: 'small' }, { default: () => label })
    },
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 300,
    render(row: any) {
      if (!row.installed) {
        return h(
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
        )
      }
      return h(
        NSpace,
        { size: 'small', wrap: false },
        {
          default: () => [
            h(
              NButton,
              {
                size: 'small',
                type: 'primary',
                onClick: () => handleOpenEnable(row),
              },
              { default: () => $gettext('Enable') },
            ),
            h(
              NButton,
              {
                size: 'small',
                onClick: async () => {
                  const ok = await confirmAction({
                    type: 'info',
                    title: $gettext('Confirm Reinstall'),
                    content: $gettext(
                      'Reinstalling will recompile %{ name } to the latest version provided by the panel. Are you sure?',
                      { name: row.name },
                    ),
                  })
                  if (ok) handleInstall(row.slug)
                },
              },
              { default: () => $gettext('Reinstall') },
            ),
            h(
              NButton,
              {
                size: 'small',
                type: 'error',
                onClick: async () => {
                  const ok = await confirmDelete({
                    title: $gettext('Confirm Uninstall'),
                    content: $gettext(
                      'Please make sure the extension %{ ext_name } has been dropped (DROP EXTENSION) in all databases that use it, otherwise those databases will fail to load it. Are you sure you want to uninstall %{ name }?',
                      { name: row.name, ext_name: row.ext_name },
                    ),
                    positiveText: $gettext('Uninstall'),
                    countdown: 5,
                  })
                  if (ok) handleUninstall(row.slug)
                },
              },
              { default: () => $gettext('Uninstall') },
            ),
          ],
        },
      )
    },
  },
]

const handleOpenEnable = (row: any) => {
  enableSlug.value = row.slug
  enableExtName.value = row.ext_name
  enableDatabase.value = ''
  fetchDatabases()
  showEnableModal.value = true
}

const handleEnable = () => {
  if (!enableDatabase.value) return
  enableLoading.value = true
  useRequest(postgresql.enableExtension(enableSlug.value, enableDatabase.value))
    .onSuccess(() => {
      window.$message.success($gettext('Enabled successfully'))
      showEnableModal.value = false
    })
    .onComplete(() => {
      enableLoading.value = false
    })
}

const handleInstall = (slug: string) => {
  useRequest(postgresql.installExtension(slug)).onSuccess(() => {
    window.$message.success($gettext('Task submitted, please check progress in background tasks'))
    refreshExtensions()
  })
}

const handleUninstall = (slug: string) => {
  useRequest(postgresql.uninstallExtension(slug)).onSuccess(() => {
    window.$message.success($gettext('Task submitted, please check progress in background tasks'))
    refreshExtensions()
  })
}
</script>

<template>
  <n-flex vertical>
    <n-alert type="info">
      {{
        $gettext(
          'Extensions need to be enabled in the database with CREATE EXTENSION, and some may require restarting PostgreSQL.',
        )
      }}
    </n-alert>
    <n-data-table striped :columns="columns" :data="extensions" :scroll-x="1000" />
    <app-modal
      v-model:show="showEnableModal"
      size="sm"
      :title="$gettext('Enable Extension') + ' - ' + enableSlug"
    >
      <n-flex vertical>
        <n-alert type="info">
          {{
            $gettext(
              'CREATE EXTENSION %{ ext_name } will be executed in the selected database. Enabling in template1 makes new databases inherit it.',
              { ext_name: enableExtName },
            )
          }}
        </n-alert>
        <n-select
          v-model:value="enableDatabase"
          :options="databaseOptions"
          :placeholder="$gettext('Select database')"
        />
        <n-flex>
          <n-button
            type="primary"
            :loading="enableLoading"
            :disabled="enableLoading || !enableDatabase"
            @click="handleEnable"
          >
            {{ $gettext('Enable') }}
          </n-button>
        </n-flex>
      </n-flex>
    </app-modal>
  </n-flex>
</template>
