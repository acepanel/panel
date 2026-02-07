<script setup lang="ts">
import database from '@/api/panel/database'
import { generateRandomString } from '@/utils'
import { NButton, NInput } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

const { $gettext } = useGettext()
const show = defineModel<boolean>('show', { type: Boolean, required: true })
const createModel = ref({
  server_id: null,
  username: '',
  password: '',
  host: 'localhost',
  privileges: [],
  remark: ''
})

const servers = ref<{ label: string; value: string }[]>([])

const hostTypeOptions = [
  { label: $gettext('Local (localhost)'), value: 'localhost' },
  { label: $gettext('All (%)'), value: '%' },
  { label: $gettext('Specific'), value: 'specific' }
]
const hostType = ref('localhost')

const loading = ref(false)

// 监听 hostType 变化，同步到 createModel.host
watch(hostType, (val) => {
  if (val !== 'specific') {
    createModel.value.host = val
  } else {
    createModel.value.host = ''
  }
})

const handleCreate = () => {
  loading.value = true
  useRequest(() => database.userCreate(createModel.value))
    .onSuccess(() => {
      show.value = false
      window.$message.success($gettext('Created successfully'))
      window.$bus.emit('database-user:refresh')
    })
    .onComplete(() => {
      loading.value = false
    })
}

watch(
  () => show.value,
  (value) => {
    if (value) {
      useRequest(database.serverList(1, 10000)).onSuccess(({ data }: { data: any }) => {
        servers.value = []
        for (const server of data.items) {
          servers.value.push({
            label: server.name,
            value: server.id
          })
        }
      })
    }
  }
)
</script>

<template>
  <n-modal
    v-model:show="show"
    preset="card"
    :title="$gettext('Create User')"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
    @close="show = false"
  >
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
        <n-form-item path="host-select" :label="$gettext('Host (MySQL only)')">
          <n-select
            v-model:value="hostType"
            @keydown.enter.prevent
            :placeholder="$gettext('Select host')"
            :options="hostTypeOptions"
          />
        </n-form-item>
        <n-form-item v-if="hostType === 'specific'" path="host" :label="$gettext('Specific Host')">
          <n-input
            v-model:value="createModel.host"
            type="text"
            @keydown.enter.prevent
            :placeholder="$gettext('Enter supported host address')"
          />
        </n-form-item>
        <n-form-item path="privileges" :label="$gettext('Privileges')">
          <n-dynamic-input
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
      <n-button type="info" block :loading="loading" :disabled="loading" @click="handleCreate">{{ $gettext('Submit') }}</n-button>
    </n-flex>
  </n-modal>
</template>

<style scoped lang="scss"></style>
