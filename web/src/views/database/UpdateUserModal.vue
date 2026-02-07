<script setup lang="ts">
import database from '@/api/panel/database'
import { NButton, NInput } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

const { $gettext } = useGettext()
const show = defineModel<boolean>('show', { type: Boolean, required: true })
const id = defineModel<number>('id', { type: Number, required: true })
const updateModel = ref({
  password: '',
  privileges: [],
  remark: ''
})

const loading = ref(false)

const handleUpdate = () => {
  loading.value = true
  useRequest(() => database.userUpdate(id.value, updateModel.value))
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
      useRequest(database.userGet(id.value)).onSuccess(({ data }: { data: any }) => {
        updateModel.value.password = data.password
        updateModel.value.privileges = data.privileges
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
    :title="$gettext('Modify User')"
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
          <n-dynamic-input
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
      <n-button type="info" block :loading="loading" :disabled="loading" @click="handleUpdate">{{ $gettext('Submit') }}</n-button>
    </n-flex>
  </n-modal>
</template>

<style scoped lang="scss"></style>
