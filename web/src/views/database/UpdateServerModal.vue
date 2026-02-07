<script setup lang="ts">
import database from '@/api/panel/database'
import { NButton, NInput } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

const { $gettext } = useGettext()
const show = defineModel<boolean>('show', { type: Boolean, required: true })
const id = defineModel<number>('id', { type: Number, required: true })
const updateModel = ref({
  name: '',
  host: '127.0.0.1',
  port: 3306,
  username: '',
  password: '',
  remark: ''
})

const loading = ref(false)

const handleUpdate = () => {
  loading.value = true
  useRequest(() => database.serverUpdate(id.value, updateModel.value))
    .onSuccess(() => {
      show.value = false
      window.$message.success($gettext('Modified successfully'))
      window.$bus.emit('database-user:refresh')
    })
    .onComplete(() => {
      loading.value = false
    })
}

watch(
  () => show.value,
  (value) => {
    if (value && id.value) {
      useRequest(database.serverGet(id.value)).onSuccess(({ data }: { data: any }) => {
        updateModel.value.name = data.name
        updateModel.value.host = data.host
        updateModel.value.port = data.port
        updateModel.value.username = data.username
        updateModel.value.password = data.password
        updateModel.value.remark = data.remark
      })
    }
  }
)
</script>

<template>
  <n-modal
    v-model:show="show"
    preset="card"
    :title="$gettext('Modify Server')"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
    @close="show = false"
  >
    <n-form :model="updateModel">
      <n-form-item path="name" :label="$gettext('Name')">
        <n-input
          v-model:value="updateModel.name"
          type="text"
          @keydown.enter.prevent
          :placeholder="$gettext('Enter database server name')"
        />
      </n-form-item>
      <n-row :gutter="[0, 24]">
        <n-col :span="15">
          <n-form-item path="host" :label="$gettext('Host')">
            <n-input
              v-model:value="updateModel.host"
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
              v-model:value="updateModel.port"
              @keydown.enter.prevent
              :placeholder="$gettext('Enter database server port')"
            />
          </n-form-item>
        </n-col>
      </n-row>
      <n-form-item path="username" :label="$gettext('Username')">
        <n-input
          v-model:value="updateModel.username"
          type="text"
          @keydown.enter.prevent
          :placeholder="$gettext('Enter database server username')"
        />
      </n-form-item>
      <n-form-item path="password" :label="$gettext('Password')">
        <n-input
          v-model:value="updateModel.password"
          type="password"
          show-password-on="click"
          @keydown.enter.prevent
          :placeholder="$gettext('Enter database server password')"
        />
      </n-form-item>
      <n-form-item path="remark" :label="$gettext('Comment')">
        <n-input
          v-model:value="updateModel.remark"
          type="textarea"
          @keydown.enter.prevent
          :placeholder="$gettext('Enter database server comment')"
        />
      </n-form-item>
    </n-form>
    <n-button type="info" block :loading="loading" :disabled="loading" @click="handleUpdate">{{ $gettext('Submit') }}</n-button>
  </n-modal>
</template>

<style scoped lang="scss"></style>
