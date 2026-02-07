<script setup lang="ts">
import ssh from '@/api/panel/ssh'
import { NInput } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

const { $gettext } = useGettext()
const show = defineModel<boolean>('show', { type: Boolean, required: true })
const loading = ref(false)

const model = ref({
  name: '',
  host: '127.0.0.1',
  port: 22,
  auth_method: 'password',
  user: 'root',
  password: '',
  key: '',
  passphrase: '',
  remark: ''
})

const handleSubmit = () => {
  loading.value = true
  useRequest(ssh.create(model.value))
    .onSuccess(() => {
      loading.value = false
      show.value = false
      model.value = {
        name: '',
        host: '127.0.0.1',
        port: 22,
        auth_method: 'password',
        user: 'root',
        password: '',
        key: '',
        passphrase: '',
        remark: ''
      }
      window.$bus.emit('ssh:refresh')
      window.$message.success($gettext('Created successfully'))
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
    :title="$gettext('Create Host')"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
  >
    <n-form>
      <n-form-item :label="$gettext('Name')">
        <n-input v-model:value="model.name" placeholder="127.0.0.1" />
      </n-form-item>
      <n-row :gutter="[0, 24]" pt-20>
        <n-col :span="15">
          <n-form-item :label="$gettext('Host')">
            <n-input v-model:value="model.host" placeholder="127.0.0.1" />
          </n-form-item>
        </n-col>
        <n-col :span="2"> </n-col>
        <n-col :span="7">
          <n-form-item :label="$gettext('Port')">
            <n-input-number v-model:value="model.port" :min="1" :max="65535" />
          </n-form-item>
        </n-col>
      </n-row>
      <n-form-item :label="$gettext('Authentication Method')">
        <n-select
          v-model:value="model.auth_method"
          :options="[
            { label: $gettext('Password'), value: 'password' },
            { label: $gettext('Private Key'), value: 'publickey' }
          ]"
        >
        </n-select>
      </n-form-item>
      <n-form-item :label="$gettext('Username')">
        <n-input v-model:value="model.user" placeholder="root" />
      </n-form-item>
      <n-form-item v-if="model.auth_method == 'password'" :label="$gettext('Password')">
        <n-input v-model:value="model.password" type="password" show-password-on="click" />
      </n-form-item>
      <n-form-item v-if="model.auth_method == 'publickey'" :label="$gettext('Private Key')">
        <n-input v-model:value="model.key" type="textarea" :rows="5" />
      </n-form-item>
      <n-form-item v-if="model.auth_method == 'publickey'" :label="$gettext('Passphrase')">
        <n-input
          v-model:value="model.passphrase"
          type="password"
          show-password-on="click"
          :placeholder="$gettext('Leave empty if key has no passphrase')"
        />
      </n-form-item>
      <n-form-item :label="$gettext('Remarks')">
        <n-input v-model:value="model.remark" type="textarea" />
      </n-form-item>
    </n-form>
    <n-row :gutter="[0, 24]" pt-20>
      <n-col :span="24">
        <n-button type="info" block :loading="loading" :disabled="loading" @click="handleSubmit">
          {{ $gettext('Submit') }}
        </n-button>
      </n-col>
    </n-row>
  </n-modal>
</template>

<style scoped lang="scss"></style>
