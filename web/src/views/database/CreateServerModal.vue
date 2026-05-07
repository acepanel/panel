<script setup lang="ts">
import { NButton, NInput, NInputGroup } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import database from '@/api/panel/database'
import PathSelector from '@/components/common/PathSelector.vue'

const props = defineProps<{
  type?: string
}>()

const { $gettext } = useGettext()
const show = defineModel<boolean>('show', { type: Boolean, required: true })
const defaultPort = (type: string) => {
  if (type === 'postgresql') return 5432
  if (type === 'redis') return 6379
  if (type === 'clickhouse') return 8123
  if (type === 'mongodb') return 27017
  if (type === 'sqlite') return 0
  if (type === 'elasticsearch') return 9200
  return 3306
}

const createModel = ref({
  name: '',
  type: props.type || 'mysql',
  host: '127.0.0.1',
  port: defaultPort(props.type || 'mysql'),
  username: '',
  password: '',
  remark: '',
})

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
watch(
  () => createModel.value.type,
  (val) => {
    createModel.value.port = defaultPort(val)
    createModel.value.host = val === 'sqlite' ? '' : '127.0.0.1'
  },
)

// 每次弹窗打开时重置 type 和端口
watch(
  () => show.value,
  (value) => {
    if (value) {
      createModel.value.type = props.type || 'mysql'
      createModel.value.port = defaultPort(props.type || 'mysql')
    }
  },
)

const showPathSelector = ref(false)
const pathSelectorPath = ref('/')
const loading = ref(false)

watch(showPathSelector, (val) => {
  if (!val && pathSelectorPath.value) {
    createModel.value.host = pathSelectorPath.value
  }
})

const handleCreate = () => {
  loading.value = true
  useRequest(() => database.serverCreate(createModel.value))
    .onSuccess(() => {
      show.value = false
      window.$message.success($gettext('Added successfully'))
      window.$bus.emit('database-server:refresh')
    })
    .onComplete(() => {
      loading.value = false
    })
}
</script>

<template>
  <n-modal
    v-model:show="show"
    preset="card"
    :title="$gettext('Add Server')"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
    @close="show = false"
  >
    <n-form :model="createModel">
      <n-form-item v-if="!props.type" path="type" :label="$gettext('Type')">
        <n-select
          v-model:value="createModel.type"
          @keydown.enter.prevent
          :placeholder="$gettext('Select type')"
          :options="typeOptions"
        />
      </n-form-item>
      <n-form-item path="name" :label="$gettext('Name')">
        <n-input
          v-model:value="createModel.name"
          type="text"
          @keydown.enter.prevent
          :placeholder="$gettext('Enter database server name')"
        />
      </n-form-item>
      <template v-if="createModel.type === 'sqlite'">
        <n-form-item path="host" :label="$gettext('File Path')">
          <n-input-group>
            <n-input
              v-model:value="createModel.host"
              type="text"
              @keydown.enter.prevent
              :placeholder="$gettext('Enter SQLite database file path, e.g. /data/app.db')"
            />
            <n-button
              type="primary"
              ghost
              @click="
                () => {
                  pathSelectorPath = createModel.host || '/'
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
        <n-row :gutter="[0, 24]">
          <n-col :span="15">
            <n-form-item path="host" :label="$gettext('Host')">
              <n-input
                v-model:value="createModel.host"
                type="text"
                @keydown.enter.prevent
                :placeholder="$gettext('Enter database server host')"
              />
            </n-form-item>
          </n-col>
          <n-col :span="2"></n-col>
          <n-col :span="7">
            <n-form-item path="port" :label="$gettext('Port')">
              <n-input-number
                w-full
                v-model:value="createModel.port"
                @keydown.enter.prevent
                :placeholder="$gettext('Enter database server port')"
              />
            </n-form-item>
          </n-col>
        </n-row>
        <n-form-item
          v-if="!['redis', 'sqlite'].includes(createModel.type)"
          path="username"
          :label="$gettext('Username')"
        >
          <n-input
            v-model:value="createModel.username"
            type="text"
            @keydown.enter.prevent
            :placeholder="$gettext('Enter database server username')"
          />
        </n-form-item>
        <n-form-item
          v-if="createModel.type !== 'sqlite'"
          path="password"
          :label="$gettext('Password')"
        >
          <n-input
            v-model:value="createModel.password"
            type="password"
            show-password-on="click"
            @keydown.enter.prevent
            :placeholder="$gettext('Enter database server password')"
          />
        </n-form-item>
      </template>
      <n-form-item path="remark" :label="$gettext('Comment')">
        <n-input
          v-model:value="createModel.remark"
          type="textarea"
          @keydown.enter.prevent
          :placeholder="$gettext('Enter database server comment')"
        />
      </n-form-item>
    </n-form>
    <n-button type="info" block :loading="loading" :disabled="loading" @click="handleCreate">
      {{ $gettext('Submit') }}
    </n-button>
  </n-modal>
  <path-selector v-model:show="showPathSelector" v-model:path="pathSelectorPath" :dir="false" />
</template>

<style scoped lang="scss"></style>
