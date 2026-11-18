<script setup lang="ts">
defineOptions({
  name: 'apps-pgadmin-index',
})

import { useGettext } from 'vue3-gettext'

import pgadmin from '@/api/apps/pgadmin'

const { $gettext } = useGettext()
const currentTab = ref('status')
const hostname = ref(window.location.hostname)
const port = ref(0)
const email = ref('')
const password = ref('')
const newPort = ref(0)
const newUsername = ref('')
const newPassword = ref('')

const url = computed(() => {
  return `http://${hostname.value}:${port.value}`
})

const getInfo = async () => {
  const data = await pgadmin.info()
  port.value = data.port
  newPort.value = data.port
  email.value = data.email
  password.value = data.password
}

const handleSavePort = async () => {
  await pgadmin.port(newPort.value)
  window.$message.success($gettext('Saved successfully'))
  getInfo()
}

const handleUpdateUsername = async () => {
  await pgadmin.updateUsername(newUsername.value)
  window.$message.success($gettext('Saved successfully'))
  newUsername.value = ''
  getInfo()
}

const handleResetPassword = async () => {
  await pgadmin.resetPassword(newPassword.value)
  window.$message.success($gettext('Password reset successfully'))
  newPassword.value = ''
  getInfo()
}

onMounted(() => {
  getInfo()
})
</script>

<template>
  <PageContainer :show-footer="true">
    <n-tabs v-model:value="currentTab" type="line" animated>
      <n-tab-pane name="status" :tab="$gettext('Running Status')">
        <n-flex vertical>
          <service-status service="pgadmin" />
          <n-card :title="$gettext('Access Information')">
            <n-flex vertical>
              <n-alert type="info">
                {{ $gettext('Access URL:') }} <a :href="url" target="_blank">{{ url }}</a>
              </n-alert>
              <n-descriptions label-placement="left" :column="1" bordered>
                <n-descriptions-item :label="$gettext('Login Email')">
                  {{ email }}
                </n-descriptions-item>
                <n-descriptions-item :label="$gettext('Login Password')">
                  {{ password }}
                </n-descriptions-item>
              </n-descriptions>
              <n-text depth="3">
                {{
                  $gettext(
                    'If the password has been changed in pgAdmin, the actual password shall prevail.',
                  )
                }}
              </n-text>
            </n-flex>
          </n-card>
        </n-flex>
      </n-tab-pane>
      <n-tab-pane name="settings" :tab="$gettext('Settings')">
        <n-flex vertical>
          <n-card :title="$gettext('Modify Port')">
            <n-flex>
              <n-input-number v-model:value="newPort" :min="1" :max="65535" />
              <app-button type="primary" @click="handleSavePort">
                {{ $gettext('Save') }}
              </app-button>
            </n-flex>
            {{ $gettext('Modify pgAdmin access port') }}
          </n-card>
          <n-card :title="$gettext('Modify Username')">
            <n-flex>
              <n-input v-model:value="newUsername" class="md:!w-60" :placeholder="email" />
              <app-button type="warning" :disabled="!newUsername" @click="handleUpdateUsername">
                {{ $gettext('Save') }}
              </app-button>
            </n-flex>
            {{
              $gettext(
                'Modify the login email of the pgAdmin administrator account, server configurations will be migrated automatically',
              )
            }}
          </n-card>
          <n-card :title="$gettext('Reset Password')">
            <n-flex>
              <n-input
                v-model:value="newPassword"
                type="password"
                show-password-on="click"
                class="md:!w-60"
                :placeholder="$gettext('New password')"
              />
              <app-button type="warning" :disabled="!newPassword" @click="handleResetPassword">
                {{ $gettext('Reset') }}
              </app-button>
            </n-flex>
            {{ $gettext('Reset the login password of the pgAdmin administrator account') }}
          </n-card>
        </n-flex>
      </n-tab-pane>
      <n-tab-pane name="run-log" :tab="$gettext('Runtime Logs')">
        <realtime-log service="pgadmin" />
      </n-tab-pane>
    </n-tabs>
  </PageContainer>
</template>
