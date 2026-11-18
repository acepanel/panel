<script setup lang="ts">
import { NButton, NInput } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import database from '@/api/panel/database'
import { generateRandomString } from '@/utils'

const props = defineProps<{
  type: string
}>()

const { $gettext } = useGettext()
const show = defineModel<boolean>('show', { type: Boolean, required: true })
const defaultModel = () => ({
  server_id: null,
  name: '',
  create_user: false,
  username: '',
  password: '',
  host: 'localhost',
})
const createModel = ref(defaultModel())

const servers = ref<{ label: string; value: string }[]>([])
const serversLoading = ref(false)

const hostTypeOptions = [
  { label: $gettext('Local (localhost)'), value: 'localhost' },
  { label: $gettext('All (%)'), value: '%' },
  { label: $gettext('Specific'), value: 'specific' },
]
const hostType = ref('localhost')

// 监听 hostType 变化，同步到 createModel.host
watch(hostType, (val) => {
  if (val !== 'specific') {
    createModel.value.host = val
  } else {
    createModel.value.host = ''
  }
})

const handleCreate = async () => {
  await database.create(createModel.value)
  show.value = false
  window.$message.success($gettext('Created successfully'))
  window.$bus.emit('database:refresh')
}

watch(
  () => show.value,
  (value) => {
    if (value) {
      // 弹窗复用同一实例，重新打开时重置表单，避免残留上一种数据库类型选中的服务器
      createModel.value = defaultModel()
      hostType.value = 'localhost'
      serversLoading.value = true
      useRequest(database.serverList(1, 10000, props.type))
        .onSuccess(({ data }: { data: any }) => {
          servers.value = []
          for (const server of data.items) {
            servers.value.push({
              label: server.name,
              value: server.id,
            })
          }
        })
        .onComplete(() => {
          serversLoading.value = false
        })
    }
  },
)
</script>

<template>
  <app-modal v-model:show="show" :title="$gettext('Create Database')">
    <n-form :model="createModel">
      <n-form-item path="server_id" :label="$gettext('Server')">
        <n-select
          v-model:value="createModel.server_id"
          @keydown.enter.prevent
          :placeholder="$gettext('Select server')"
          :options="servers"
          :loading="serversLoading"
        />
      </n-form-item>
      <n-form-item path="database" :label="$gettext('Database Name')">
        <n-input
          v-model:value="createModel.name"
          type="text"
          @keydown.enter.prevent
          :placeholder="$gettext('Enter database name')"
        />
      </n-form-item>
      <n-form-item path="create_user" :label="$gettext('Create User')">
        <n-switch v-model:value="createModel.create_user" />
      </n-form-item>
      <n-form-item
        v-if="!createModel.create_user"
        path="username"
        :label="$gettext('Authorized User')"
      >
        <n-input
          v-model:value="createModel.username"
          type="text"
          @keydown.enter.prevent
          :placeholder="$gettext('Enter authorized username (leave empty for no authorization)')"
        />
      </n-form-item>
      <n-form-item v-if="createModel.create_user" path="username" :label="$gettext('Username')">
        <n-input
          v-model:value="createModel.username"
          type="text"
          @keydown.enter.prevent
          :placeholder="$gettext('Enter username')"
        />
      </n-form-item>
      <n-form-item v-if="createModel.create_user" path="password" :label="$gettext('Password')">
        <n-input-group>
          <n-input
            v-model:value="createModel.password"
            type="password"
            show-password-on="click"
            @keydown.enter.prevent
            :placeholder="$gettext('Enter password')"
          />
          <n-button @click="createModel.password = generateRandomString(16)">
            {{ $gettext('Generate') }}
          </n-button>
        </n-input-group>
      </n-form-item>
      <n-form-item
        v-if="createModel.create_user && props.type === 'mysql'"
        path="host-select"
        :label="$gettext('Host')"
      >
        <n-select
          v-model:value="hostType"
          @keydown.enter.prevent
          :placeholder="$gettext('Select host')"
          :options="hostTypeOptions"
        />
      </n-form-item>
      <n-form-item
        v-if="createModel.create_user && hostType === 'specific' && props.type === 'mysql'"
        path="host"
        :label="$gettext('Specific Host')"
      >
        <n-input
          v-model:value="createModel.host"
          type="text"
          @keydown.enter.prevent
          :placeholder="$gettext('Enter supported host address')"
        />
      </n-form-item>
    </n-form>
    <app-button type="info" block @click="handleCreate">
      {{ $gettext('Submit') }}
    </app-button>
  </app-modal>
</template>

<style scoped lang="scss"></style>
