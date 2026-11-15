<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

import database from '@/api/panel/database'

const { $gettext } = useGettext()

const show = defineModel<boolean>('show', { type: Boolean, required: true })
const props = defineProps<{
  editId?: number
}>()

const isEdit = computed(() => !!props.editId)
const loading = ref(false)

const defaultPort = (type: string) => {
  if (type === 'postgresql') return 5432
  if (type === 'redis') return 6379
  if (type === 'clickhouse') return 8123
  if (type === 'mongodb') return 27017
  if (type === 'sqlite') return 0
  if (type === 'elasticsearch') return 9200
  return 3306
}

const defaultModel = () => ({
  name: '',
  host: '127.0.0.1',
  port: 3306,
  username: '',
  password: '',
  remark: '',
})

// 类型只在创建时提交，更新接口不接收
const type = ref('mysql')
const model = ref(defaultModel())

const typeOptions = [
  { label: 'MySQL', value: 'mysql' },
  { label: 'PostgreSQL', value: 'postgresql' },
  { label: 'ClickHouse', value: 'clickhouse' },
  { label: 'MongoDB', value: 'mongodb' },
  { label: 'SQLite', value: 'sqlite' },
  { label: 'Elasticsearch', value: 'elasticsearch' },
  { label: 'Redis', value: 'redis' },
]

// 切换类型时自动更新端口和主机
const handleTypeChange = (value: string) => {
  model.value.port = defaultPort(value)
  model.value.host = value === 'sqlite' ? '' : '127.0.0.1'
}

watch(show, (val) => {
  if (!val) return
  type.value = 'mysql'
  model.value = defaultModel()
  if (!props.editId) return
  useRequest(database.serverGet(props.editId)).onSuccess(({ data }: { data: any }) => {
    type.value = data.type
    model.value = {
      name: data.name,
      host: data.host,
      port: data.port,
      username: data.username,
      password: data.password,
      remark: data.remark,
    }
  })
})

const showPathSelector = ref(false)
const pathSelectorPath = ref('/')

watch(showPathSelector, (val) => {
  if (!val && pathSelectorPath.value) {
    model.value.host = pathSelectorPath.value
  }
})

const handleSubmit = () => {
  loading.value = true
  const req = isEdit.value
    ? database.serverUpdate(props.editId!, model.value)
    : database.serverCreate({ ...model.value, type: type.value })
  useRequest(req)
    .onSuccess(() => {
      show.value = false
      window.$message.success(
        isEdit.value ? $gettext('Modified successfully') : $gettext('Added successfully'),
      )
      window.$bus.emit('database-user:refresh')
      window.$bus.emit('database-server:refresh')
    })
    .onComplete(() => {
      loading.value = false
    })
}
</script>

<template>
  <app-modal
    v-model:show="show"
    :title="isEdit ? $gettext('Modify Server') : $gettext('Add Server')"
  >
    <n-form :model="model">
      <n-form-item v-if="!isEdit" path="type" :label="$gettext('Type')">
        <n-select
          v-model:value="type"
          @keydown.enter.prevent
          :placeholder="$gettext('Select type')"
          :options="typeOptions"
          @update:value="handleTypeChange"
        />
      </n-form-item>
      <n-form-item path="name" :label="$gettext('Name')">
        <n-input
          v-model:value="model.name"
          type="text"
          @keydown.enter.prevent
          :placeholder="$gettext('Enter database server name')"
        />
      </n-form-item>
      <template v-if="type === 'sqlite'">
        <n-form-item path="host" :label="$gettext('File Path')">
          <n-input-group>
            <n-input
              v-model:value="model.host"
              type="text"
              @keydown.enter.prevent
              :placeholder="$gettext('Enter SQLite database file path, e.g. /data/app.db')"
            />
            <n-button
              type="primary"
              ghost
              @click="
                () => {
                  pathSelectorPath = model.host || '/'
                  showPathSelector = true
                }
              "
            >
              {{ $gettext('Select') }}
            </n-button>
          </n-input-group>
        </n-form-item>
      </template>
      <template v-else>
        <n-grid cols="1 m:24" responsive="screen">
          <n-form-item-gi :span="15" path="host" :label="$gettext('Host')">
            <n-input
              v-model:value="model.host"
              type="text"
              @keydown.enter.prevent
              :placeholder="$gettext('Enter database server host')"
            />
          </n-form-item-gi>
          <n-gi :span="2" />
          <n-form-item-gi :span="7" path="port" :label="$gettext('Port')">
            <n-input-number
              w-full
              v-model:value="model.port"
              @keydown.enter.prevent
              :placeholder="$gettext('Enter database server port')"
            />
          </n-form-item-gi>
        </n-grid>
        <n-form-item v-if="type !== 'redis'" path="username" :label="$gettext('Username')">
          <n-input
            v-model:value="model.username"
            type="text"
            @keydown.enter.prevent
            :placeholder="$gettext('Enter database server username')"
          />
        </n-form-item>
        <n-form-item path="password" :label="$gettext('Password')">
          <n-input
            v-model:value="model.password"
            type="password"
            show-password-on="click"
            @keydown.enter.prevent
            :placeholder="$gettext('Enter database server password')"
          />
        </n-form-item>
      </template>
      <n-form-item path="remark" :label="$gettext('Comment')">
        <n-input
          v-model:value="model.remark"
          type="textarea"
          @keydown.enter.prevent
          :placeholder="$gettext('Enter database server comment')"
        />
      </n-form-item>
    </n-form>
    <n-button type="info" block :loading="loading" :disabled="loading" @click="handleSubmit">
      {{ $gettext('Submit') }}
    </n-button>
  </app-modal>
  <path-selector v-model:show="showPathSelector" v-model:path="pathSelectorPath" :dir="false" />
</template>
