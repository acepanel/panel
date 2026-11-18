<script setup lang="ts">
import { NInput } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import database from '@/api/panel/database'

const { $gettext } = useGettext()
const show = defineModel<boolean>('show', { type: Boolean, required: true })
const id = defineModel<number>('id', { type: Number, required: true })
const updateModel = ref({
  password: '',
  privileges: [],
  remark: '',
})

const loading = ref(false)

const handleUpdate = async () => {
  await database.userUpdate(id.value, updateModel.value)
  show.value = false
  window.$message.success($gettext('Modified successfully'))
  window.$bus.emit('database-user:refresh')
}

watch(
  () => show.value,
  (value) => {
    if (value && id.value) {
      loading.value = true
      useRequest(database.userGet(id.value))
        .onSuccess(({ data }: { data: any }) => {
          updateModel.value.password = data.password
          updateModel.value.privileges = data.privileges
          updateModel.value.remark = data.remark
        })
        .onComplete(() => {
          loading.value = false
        })
    }
  },
)
</script>

<template>
  <app-modal v-model:show="show" :title="$gettext('Modify User')">
    <n-flex vertical>
      <n-alert type="info">
        {{
          $gettext('If the privilege databases does not exist, it will be created automatically.')
        }}
      </n-alert>
      <n-spin :show="loading">
        <n-form :model="updateModel">
          <n-form-item path="password" :label="$gettext('Password')">
            <n-input
              v-model:value="updateModel.password"
              type="password"
              show-password-on="click"
              @keydown.enter.prevent
              :placeholder="$gettext('Enter password')"
            />
          </n-form-item>
          <n-form-item path="privileges" :label="$gettext('Privileges')">
            <ListInput
              v-model:value="updateModel.privileges"
              :placeholder="$gettext('Enter database name')"
            />
          </n-form-item>
          <n-form-item path="remark" :label="$gettext('Comment')">
            <n-input
              v-model:value="updateModel.remark"
              type="textarea"
              @keydown.enter.prevent
              :placeholder="$gettext('Enter database user comment')"
            />
          </n-form-item>
        </n-form>
      </n-spin>
      <app-button type="info" block :disabled="loading" @click="handleUpdate">{{
        $gettext('Submit')
      }}</app-button>
    </n-flex>
  </app-modal>
</template>

<style scoped lang="scss"></style>
