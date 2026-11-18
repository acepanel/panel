<script setup lang="ts">
import { NButton, NInput } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import database from '@/api/panel/database'
import { generateRandomString } from '@/utils'

const props = defineProps<{
  type?: string
}>()

const { $gettext } = useGettext()
const show = defineModel<boolean>('show', { type: Boolean, required: true })
const defaultModel = () => ({
  server_id: null,
  username: '',
  password: '',
  host: 'localhost',
  privileges: [],
  remark: '',
})
const createModel = ref(defaultModel())

const servers = ref<{ label: string; value: string; type: string }[]>([])
const serversLoading = ref(false)

// 仅这些数据库类型支持用户管理
const userSupportedTypes = ['mysql', 'postgresql', 'clickhouse']

// 当前选中服务器的类型，用于决定是否显示 Host 字段（仅 MySQL 需要）
const selectedServerType = computed(
  () => servers.value.find((s) => s.value === createModel.value.server_id)?.type,
)

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
  await database.userCreate(createModel.value)
  show.value = false
  window.$message.success($gettext('Created successfully'))
  window.$bus.emit('database-user:refresh')
}

watch(
  () => show.value,
  (value) => {
    if (value) {
      // 弹窗复用同一实例，重新打开时重置表单，避免残留上次创建的用户信息
      createModel.value = defaultModel()
      hostType.value = 'localhost'
      serversLoading.value = true
      useRequest(database.serverList(1, 10000, props.type))
        .onSuccess(({ data }: { data: any }) => {
          servers.value = []
          for (const server of data.items) {
            if (!userSupportedTypes.includes(server.type)) {
              continue
            }
            servers.value.push({
              label: server.name,
              value: server.id,
              type: server.type,
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
  <app-modal v-model:show="show" :title="$gettext('Create User')">
    <n-flex vertical>
      <n-alert type="info">
        {{
          $gettext('If the privilege databases does not exist, it will be created automatically.')
        }}
      </n-alert>
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
        <n-form-item path="username" :label="$gettext('Username')">
          <n-input
            v-model:value="createModel.username"
            type="text"
            @keydown.enter.prevent
            :placeholder="$gettext('Enter username')"
          />
        </n-form-item>
        <n-form-item path="password" :label="$gettext('Password')">
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
          v-if="selectedServerType === 'mysql'"
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
          v-if="hostType === 'specific' && selectedServerType === 'mysql'"
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
        <n-form-item path="privileges" :label="$gettext('Privileges')">
          <ListInput
            v-model:value="createModel.privileges"
            :placeholder="$gettext('Enter database name')"
          />
        </n-form-item>
        <n-form-item path="remark" :label="$gettext('Comment')">
          <n-input
            v-model:value="createModel.remark"
            type="textarea"
            @keydown.enter.prevent
            :placeholder="$gettext('Enter database user comment')"
          />
        </n-form-item>
      </n-form>
      <app-button type="info" block @click="handleCreate">
        {{ $gettext('Submit') }}
      </app-button>
    </n-flex>
  </app-modal>
</template>

<style scoped lang="scss"></style>
