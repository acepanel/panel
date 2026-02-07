<script setup lang="ts">
import user from '@/api/panel/user'
import { NButton, NInput } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

const { $gettext } = useGettext()
const show = defineModel<boolean>('show', { type: Boolean, required: true })
const id = defineModel<number>('id', { type: Number, required: true })
const model = ref({
  password: ''
})

const loading = ref(false)

const handleUpdate = () => {
  loading.value = true
  useRequest(() => user.updatePassword(id.value, model.value.password))
    .onSuccess(() => {
      show.value = false
      window.$message.success($gettext('Updated successfully'))
      window.$bus.emit('user:refresh')
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
    :title="$gettext('Change Password')"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
    @close="show = false"
  >
    <n-form :model="model">
      <n-form-item path="password" :label="$gettext('Password')">
        <n-input
          v-model:value="model.password"
          type="password"
          show-password-on="click"
          @keydown.enter.prevent
          :placeholder="$gettext('Enter user password')"
        />
      </n-form-item>
    </n-form>
    <n-button type="info" block :loading="loading" :disabled="loading" @click="handleUpdate">{{ $gettext('Submit') }}</n-button>
  </n-modal>
</template>

<style scoped lang="scss"></style>
