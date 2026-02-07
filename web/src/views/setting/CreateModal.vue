<script setup lang="ts">
import user from '@/api/panel/user'
import { NButton, NInput } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

const { $gettext } = useGettext()
const show = defineModel<boolean>('show', { type: Boolean, required: true })
const model = ref({
  username: '',
  password: '',
  email: ''
})

const loading = ref(false)

const handleCreate = () => {
  loading.value = true
  useRequest(() =>
    user.create(model.value.username, model.value.password, model.value.email)
  )
    .onSuccess(() => {
      show.value = false
      window.$message.success($gettext('Created successfully'))
      window.$bus.emit('user:refresh')
      model.value.username = ''
      model.value.password = ''
      model.value.email = ''
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
    :title="$gettext('Create User')"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
    @close="show = false"
  >
    <n-form :model="model">
      <n-form-item path="username" :label="$gettext('Username')">
        <n-input
          v-model:value="model.username"
          @keydown.enter.prevent
          :placeholder="$gettext('Enter user name')"
        />
      </n-form-item>
      <n-form-item path="password" :label="$gettext('Password')">
        <n-input
          v-model:value="model.password"
          type="password"
          show-password-on="click"
          @keydown.enter.prevent
          :placeholder="$gettext('Enter user password')"
        />
      </n-form-item>
      <n-form-item path="email" :label="$gettext('Email')">
        <n-input
          v-model:value="model.email"
          @keydown.enter.prevent
          :placeholder="$gettext('Enter user email')"
        />
      </n-form-item>
    </n-form>
    <n-button type="info" block :loading="loading" :disabled="loading" @click="handleCreate">{{ $gettext('Submit') }}</n-button>
  </n-modal>
</template>

<style scoped lang="scss"></style>
