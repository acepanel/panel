<script setup lang="ts">
import database from '@/api/panel/database'
import { NButton, NInput } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

const { $gettext } = useGettext()
const show = defineModel<boolean>('show', { type: Boolean, required: true })
const createModel = ref({
  name: '',
  type: 'mysql',
  host: '127.0.0.1',
  port: 3306,
  username: '',
  password: '',
  remark: ''
})

const databaseType = [
  { label: 'MySQL', value: 'mysql' },
  { label: 'PostgreSQL', value: 'postgresql' }
]

watch(
  () => createModel.value.type,
  (value) => {
    if (value === 'mysql') {
      createModel.value.port = 3306
    } else if (value === 'postgresql') {
      createModel.value.port = 5432
    }
  }
)

const loading = ref(false)

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
      <n-form-item path="name" :label="$gettext('Name')">
        <n-input
          v-model:value="createModel.name"
          type="text"
          @keydown.enter.prevent
          :placeholder="$gettext('Enter database server name')"
        />
      </n-form-item>
      <n-form-item path="type" :label="$gettext('Type')">
        <n-select
          v-model:value="createModel.type"
          @keydown.enter.prevent
          :placeholder="$gettext('Select database type')"
          :options="databaseType"
        />
      </n-form-item>
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
      <n-form-item path="username" :label="$gettext('Username')">
        <n-input
          v-model:value="createModel.username"
          type="text"
          @keydown.enter.prevent
          :placeholder="$gettext('Enter database server username')"
        />
      </n-form-item>
      <n-form-item path="password" :label="$gettext('Password')">
        <n-input
          v-model:value="createModel.password"
          type="password"
          show-password-on="click"
          @keydown.enter.prevent
          :placeholder="$gettext('Enter database server password')"
        />
      </n-form-item>
      <n-form-item path="remark" :label="$gettext('Comment')">
        <n-input
          v-model:value="createModel.remark"
          type="textarea"
          @keydown.enter.prevent
          :placeholder="$gettext('Enter database server comment')"
        />
      </n-form-item>
    </n-form>
    <n-button type="info" block :loading="loading" :disabled="loading" @click="handleCreate">{{ $gettext('Submit') }}</n-button>
  </n-modal>
</template>

<style scoped lang="scss"></style>
