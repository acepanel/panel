<script setup lang="ts">
import app from '@/api/panel/app'
import storage from '@/api/panel/backup-storage'
import cron from '@/api/panel/cron'
import home from '@/api/panel/home'
import website from '@/api/panel/website'
import CronSelector from '@/components/common/CronSelector.vue'
import { NInput } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

const { $gettext } = useGettext()
const show = defineModel<boolean>('show', { type: Boolean, required: true })
const loading = ref(false)

const createModel = ref({
  name: '',
  type: 'shell',
  target: '',
  keep: 1,
  backup_type: 'website',
  backup_storage: 0,
  script:
    `#!/bin/bash\nexport PATH=/bin:/sbin:/usr/bin:/usr/sbin:/usr/local/bin:/usr/local/sbin:$PATH\n\n` +
    $gettext('# Enter your script content here') +
    `\n`,
  time: '*/30 * * * *'
})

const websites = ref<any>([])
const storages = ref<any[]>([])

const { data: installedEnvironment } = useRequest(home.installedEnvironment, {
  initialData: {
    db: [
      {
        label: '',
        value: ''
      }
    ]
  }
})

const mySQLInstalled = computed(() => {
  return installedEnvironment.value.db.find((item: any) => item.value === 'mysql')
})

const postgreSQLInstalled = computed(() => {
  return installedEnvironment.value.db.find((item: any) => item.value === 'postgresql')
})

const handleSubmit = async () => {
  loading.value = true
  useRequest(cron.create(createModel.value))
    .onSuccess(() => {
      window.$message.success($gettext('Created successfully'))
      window.$bus.emit('task:refresh-cron')
    })
    .onComplete(() => {
      show.value = false
      loading.value = false
    })
}

watch(createModel, (value) => {
  if (value.backup_type === 'website') {
    createModel.value.target = websites.value[0]?.value
  } else {
    createModel.value.target = ''
  }
})

const generateTaskName = () => {
  const type = createModel.value.type
  const target = createModel.value.target

  if (type === 'backup') {
    const backupTypeMap: Record<string, string> = {
      website: $gettext('Backup Website'),
      mysql: $gettext('Backup MySQL'),
      postgres: $gettext('Backup PostgreSQL')
    }
    const prefix = backupTypeMap[createModel.value.backup_type] || $gettext('Backup')
    createModel.value.name = target ? `${prefix} - ${target}` : prefix
  } else if (type === 'cutoff') {
    createModel.value.name = target
      ? `${$gettext('Log Rotation')} - ${target}`
      : $gettext('Log Rotation')
  }
}

watch(
  () => [createModel.value.type, createModel.value.backup_type, createModel.value.target],
  () => {
    generateTaskName()
  }
)

onMounted(() => {
  useRequest(app.isInstalled('nginx')).onSuccess(({ data }) => {
    if (data) {
      useRequest(website.list('all', 1, 10000)).onSuccess(({ data }: { data: any }) => {
        for (const item of data.items) {
          websites.value.push({
            label: item.name,
            value: item.name
          })
        }
        createModel.value.target = websites.value[0]?.value
      })
    }
  })
  useRequest(storage.list(1, 10000)).onSuccess(({ data }: { data: any }) => {
    for (const item of data.items) {
      storages.value.push({
        label: item.name,
        value: item.id
      })
    }
    createModel.value.backup_storage = storages.value[0]?.value || 0
  })
})
</script>

<template>
  <n-modal
    v-model:show="show"
    preset="card"
    :title="$gettext('Create Scheduled Task')"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
  >
    <n-form>
      <n-form-item :label="$gettext('Task Type')">
        <n-select
          v-model:value="createModel.type"
          :options="[
            { label: $gettext('Run Script'), value: 'shell' },
            { label: $gettext('Backup Data'), value: 'backup' },
            { label: $gettext('Log Rotation'), value: 'cutoff' }
          ]"
        >
        </n-select>
      </n-form-item>
      <n-form-item :label="$gettext('Task Name')">
        <n-input v-model:value="createModel.name" :placeholder="$gettext('Task Name')" />
      </n-form-item>
      <n-form-item :label="$gettext('Task Schedule')">
        <cron-selector v-model:value="createModel.time" />
      </n-form-item>
      <div v-if="createModel.type === 'shell'">
        <n-text>{{ $gettext('Script Content') }}</n-text>
        <common-editor v-model:value="createModel.script" lang="shell" height="40vh" />
      </div>
      <n-form-item v-if="createModel.type === 'backup'" :label="$gettext('Backup Type')">
        <n-radio-group v-model:value="createModel.backup_type">
          <n-radio value="website">{{ $gettext('Website') }}</n-radio>
          <n-radio value="mysql" :disabled="!mySQLInstalled">
            {{ $gettext('MySQL Database') }}</n-radio
          >
          <n-radio value="postgres" :disabled="!postgreSQLInstalled">
            {{ $gettext('PostgreSQL Database') }}
          </n-radio>
        </n-radio-group>
      </n-form-item>
      <n-form-item
        v-if="
          (createModel.backup_type === 'website' && createModel.type === 'backup') ||
          createModel.type === 'cutoff'
        "
        :label="$gettext('Select Website')"
      >
        <n-select
          v-model:value="createModel.target"
          :options="websites"
          :placeholder="$gettext('Select Website')"
        />
      </n-form-item>
      <n-form-item
        v-if="createModel.backup_type !== 'website' && createModel.type === 'backup'"
        :label="$gettext('Database Name')"
      >
        <n-input v-model:value="createModel.target" :placeholder="$gettext('Database Name')" />
      </n-form-item>
      <n-form-item v-if="createModel.type === 'backup'" :label="$gettext('Backup Storage')">
        <n-select
          v-model:value="createModel.backup_storage"
          :options="storages"
          :placeholder="$gettext('Select backup storage')"
        />
      </n-form-item>
      <n-form-item v-if="createModel.type !== 'shell'" :label="$gettext('Retention Count')">
        <n-input-number v-model:value="createModel.keep" />
      </n-form-item>
    </n-form>
    <n-button type="info" :loading="loading" :disabled="loading" @click="handleSubmit" mt-10 block>
      {{ $gettext('Submit') }}
    </n-button>
  </n-modal>
</template>

<style scoped lang="scss"></style>
