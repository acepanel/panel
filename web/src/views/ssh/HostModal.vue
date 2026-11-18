<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

import ssh from '@/api/panel/ssh'

const { $gettext } = useGettext()
const show = defineModel<boolean>('show', { type: Boolean, required: true })
const props = defineProps<{
  editId?: number
}>()

const isEdit = computed(() => !!props.editId)

const defaultModel = () => ({
  name: '',
  host: '127.0.0.1',
  port: 22,
  auth_method: 'password',
  user: 'root',
  password: '',
  key: '',
  passphrase: '',
  remark: '',
})

const model = ref(defaultModel())
// 编辑时回填前表单还是默认值，期间禁止提交
const loading = ref(false)

watch(show, (val) => {
  if (!val) return
  model.value = defaultModel()
  if (!isEdit.value) return
  loading.value = true
  useRequest(ssh.get(props.editId!))
    .onSuccess(({ data }: any) => {
      model.value = {
        name: data.name,
        host: data.host,
        port: data.port,
        auth_method: data.config.auth_method,
        user: data.config.user,
        password: data.config.password,
        key: data.config.key,
        passphrase: data.config.passphrase || '',
        remark: data.remark,
      }
    })
    .onComplete(() => {
      loading.value = false
    })
})

const handleSubmit = async () => {
  await (isEdit.value ? ssh.update(props.editId!, model.value) : ssh.create(model.value))
  show.value = false
  window.$bus.emit('ssh:refresh')
  window.$message.success(
    isEdit.value ? $gettext('Updated successfully') : $gettext('Created successfully'),
  )
}
</script>

<template>
  <app-modal
    v-model:show="show"
    :title="isEdit ? $gettext('Update Host') : $gettext('Create Host')"
  >
    <n-spin :show="loading">
      <n-form>
        <n-form-item :label="$gettext('Name')">
          <n-input v-model:value="model.name" placeholder="127.0.0.1" />
        </n-form-item>
        <n-grid cols="1 m:24" responsive="screen" pt-5>
          <n-form-item-gi :span="15" :label="$gettext('Host')">
            <n-input v-model:value="model.host" placeholder="127.0.0.1" />
          </n-form-item-gi>
          <n-gi :span="2" />
          <n-form-item-gi :span="7" :label="$gettext('Port')">
            <n-input-number v-model:value="model.port" :min="1" :max="65535" />
          </n-form-item-gi>
        </n-grid>
        <n-form-item :label="$gettext('Authentication Method')">
          <n-select
            v-model:value="model.auth_method"
            :options="[
              { label: $gettext('Password'), value: 'password' },
              { label: $gettext('Private Key'), value: 'publickey' },
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
    </n-spin>
    <app-button type="info" block class="mt-5" :disabled="loading" @click="handleSubmit">
      {{ $gettext('Submit') }}
    </app-button>
  </app-modal>
</template>
