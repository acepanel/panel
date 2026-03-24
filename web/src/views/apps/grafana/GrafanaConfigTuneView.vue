<script setup lang="ts">
defineOptions({
  name: 'grafana-config-tune'
})

import { useGettext } from 'vue3-gettext'

import grafana from '@/api/apps/grafana'

const { $gettext } = useGettext()
const currentTab = ref('server')

// [server]
const httpPort = ref<number | null>(null)
const domain = ref('')
const rootUrl = ref('')
const protocol = ref('')

// [database]
const dbType = ref('')
const dbHost = ref('')
const dbName = ref('')
const dbUser = ref('')
const dbPassword = ref('')

// [security]
const adminUser = ref('')
const adminPassword = ref('')

// [users]
const allowSignUp = ref('')
const autoAssignOrgRole = ref('')

// [smtp]
const smtpEnabled = ref('')
const smtpHost = ref('')
const smtpUser = ref('')
const smtpPassword = ref('')
const smtpFromAddress = ref('')

// [log]
const logMode = ref('')
const logLevel = ref('')

const saveLoading = ref(false)

const protocolOptions = [
  { label: 'http', value: 'http' },
  { label: 'https', value: 'https' }
]

const dbTypeOptions = [
  { label: 'sqlite3', value: 'sqlite3' },
  { label: 'mysql', value: 'mysql' },
  { label: 'postgres', value: 'postgres' }
]

const boolOptions = [
  { label: 'true', value: 'true' },
  { label: 'false', value: 'false' }
]

const orgRoleOptions = [
  { label: 'Viewer', value: 'Viewer' },
  { label: 'Editor', value: 'Editor' },
  { label: 'Admin', value: 'Admin' }
]

const logModeOptions = [
  { label: 'console', value: 'console' },
  { label: 'file', value: 'file' },
  { label: 'console file', value: 'console file' }
]

const logLevelOptions = [
  { label: 'debug', value: 'debug' },
  { label: 'info', value: 'info' },
  { label: 'warn', value: 'warn' },
  { label: 'error', value: 'error' },
  { label: 'critical', value: 'critical' }
]

useRequest(grafana.configTune()).onSuccess(({ data }: any) => {
  // [server]
  httpPort.value = Number(data.http_port) || null
  domain.value = data.domain ?? ''
  rootUrl.value = data.root_url ?? ''
  protocol.value = data.protocol || null
  // [database]
  dbType.value = data.db_type || null
  dbHost.value = data.db_host ?? ''
  dbName.value = data.db_name ?? ''
  dbUser.value = data.db_user ?? ''
  dbPassword.value = data.db_password ?? ''
  // [security]
  adminUser.value = data.admin_user ?? ''
  adminPassword.value = data.admin_password ?? ''
  // [users]
  allowSignUp.value = data.allow_sign_up || null
  autoAssignOrgRole.value = data.auto_assign_org_role || null
  // [smtp]
  smtpEnabled.value = data.smtp_enabled || null
  smtpHost.value = data.smtp_host ?? ''
  smtpUser.value = data.smtp_user ?? ''
  smtpPassword.value = data.smtp_password ?? ''
  smtpFromAddress.value = data.smtp_from_address ?? ''
  // [log]
  logMode.value = data.log_mode || null
  logLevel.value = data.log_level || null
})

const getConfigData = () => ({
  http_port: String(httpPort.value ?? ''),
  domain: domain.value,
  root_url: rootUrl.value,
  protocol: protocol.value ?? '',
  db_type: dbType.value ?? '',
  db_host: dbHost.value,
  db_name: dbName.value,
  db_user: dbUser.value,
  db_password: dbPassword.value,
  admin_user: adminUser.value,
  admin_password: adminPassword.value,
  allow_sign_up: allowSignUp.value ?? '',
  auto_assign_org_role: autoAssignOrgRole.value ?? '',
  smtp_enabled: smtpEnabled.value ?? '',
  smtp_host: smtpHost.value,
  smtp_user: smtpUser.value,
  smtp_password: smtpPassword.value,
  smtp_from_address: smtpFromAddress.value,
  log_mode: logMode.value ?? '',
  log_level: logLevel.value ?? ''
})

const handleSave = () => {
  saveLoading.value = true
  useRequest(grafana.saveConfigTune(getConfigData()))
    .onSuccess(() => {
      window.$message.success($gettext('Saved successfully'))
    })
    .onComplete(() => {
      saveLoading.value = false
    })
}
</script>

<template>
  <n-tabs v-model:value="currentTab" type="line" placement="left" animated>
    <n-tab-pane name="server" :tab="$gettext('Server')">
      <n-flex vertical>
        <n-alert type="info">
          {{ $gettext('Grafana server settings.') }}
        </n-alert>
        <n-form>
          <n-form-item :label="$gettext('HTTP Port (http_port)')">
            <n-input-number class="w-full" v-model:value="httpPort" :placeholder="$gettext('e.g. 3000')" :min="1" :max="65535" />
          </n-form-item>
          <n-form-item :label="$gettext('Domain (domain)')">
            <n-input v-model:value="domain" :placeholder="$gettext('e.g. localhost')" />
          </n-form-item>
          <n-form-item :label="$gettext('Root URL (root_url)')">
            <n-input v-model:value="rootUrl" :placeholder="$gettext('e.g. %(protocol)s://%(domain)s:%(http_port)s/')" />
          </n-form-item>
          <n-form-item :label="$gettext('Protocol (protocol)')">
            <n-select v-model:value="protocol" :options="protocolOptions" clearable />
          </n-form-item>
        </n-form>
        <n-flex>
          <n-button type="primary" :loading="saveLoading" :disabled="saveLoading" @click="handleSave">
            {{ $gettext('Save') }}
          </n-button>
        </n-flex>
      </n-flex>
    </n-tab-pane>
    <n-tab-pane name="database" :tab="$gettext('Database')">
      <n-flex vertical>
        <n-alert type="info">
          {{ $gettext('Grafana database settings.') }}
        </n-alert>
        <n-form>
          <n-form-item :label="$gettext('Type (type)')">
            <n-select v-model:value="dbType" :options="dbTypeOptions" clearable />
          </n-form-item>
          <n-form-item :label="$gettext('Host (host)')">
            <n-input v-model:value="dbHost" :placeholder="$gettext('e.g. 127.0.0.1:3306')" />
          </n-form-item>
          <n-form-item :label="$gettext('Name (name)')">
            <n-input v-model:value="dbName" :placeholder="$gettext('e.g. grafana')" />
          </n-form-item>
          <n-form-item :label="$gettext('User (user)')">
            <n-input v-model:value="dbUser" />
          </n-form-item>
          <n-form-item :label="$gettext('Password (password)')">
            <n-input v-model:value="dbPassword" type="password" show-password-on="click" />
          </n-form-item>
        </n-form>
        <n-flex>
          <n-button type="primary" :loading="saveLoading" :disabled="saveLoading" @click="handleSave">
            {{ $gettext('Save') }}
          </n-button>
        </n-flex>
      </n-flex>
    </n-tab-pane>
    <n-tab-pane name="security" :tab="$gettext('Security')">
      <n-flex vertical>
        <n-alert type="info">
          {{ $gettext('Grafana security settings.') }}
        </n-alert>
        <n-form>
          <n-form-item :label="$gettext('Admin User (admin_user)')">
            <n-input v-model:value="adminUser" :placeholder="$gettext('e.g. admin')" />
          </n-form-item>
          <n-form-item :label="$gettext('Admin Password (admin_password)')">
            <n-input v-model:value="adminPassword" type="password" show-password-on="click" />
          </n-form-item>
        </n-form>
        <n-flex>
          <n-button type="primary" :loading="saveLoading" :disabled="saveLoading" @click="handleSave">
            {{ $gettext('Save') }}
          </n-button>
        </n-flex>
      </n-flex>
    </n-tab-pane>
    <n-tab-pane name="users" :tab="$gettext('Users')">
      <n-flex vertical>
        <n-alert type="info">
          {{ $gettext('Grafana user management settings.') }}
        </n-alert>
        <n-form>
          <n-form-item :label="$gettext('Allow Sign Up (allow_sign_up)')">
            <n-select v-model:value="allowSignUp" :options="boolOptions" clearable />
          </n-form-item>
          <n-form-item :label="$gettext('Auto Assign Org Role (auto_assign_org_role)')">
            <n-select v-model:value="autoAssignOrgRole" :options="orgRoleOptions" clearable />
          </n-form-item>
        </n-form>
        <n-flex>
          <n-button type="primary" :loading="saveLoading" :disabled="saveLoading" @click="handleSave">
            {{ $gettext('Save') }}
          </n-button>
        </n-flex>
      </n-flex>
    </n-tab-pane>
    <n-tab-pane name="smtp" tab="SMTP">
      <n-flex vertical>
        <n-alert type="info">
          {{ $gettext('Grafana SMTP email settings.') }}
        </n-alert>
        <n-form>
          <n-form-item :label="$gettext('Enabled (enabled)')">
            <n-select v-model:value="smtpEnabled" :options="boolOptions" clearable />
          </n-form-item>
          <n-form-item :label="$gettext('Host (host)')">
            <n-input v-model:value="smtpHost" :placeholder="$gettext('e.g. smtp.example.com:587')" />
          </n-form-item>
          <n-form-item :label="$gettext('User (user)')">
            <n-input v-model:value="smtpUser" />
          </n-form-item>
          <n-form-item :label="$gettext('Password (password)')">
            <n-input v-model:value="smtpPassword" type="password" show-password-on="click" />
          </n-form-item>
          <n-form-item :label="$gettext('From Address (from_address)')">
            <n-input v-model:value="smtpFromAddress" :placeholder="$gettext('e.g. admin@example.com')" />
          </n-form-item>
        </n-form>
        <n-flex>
          <n-button type="primary" :loading="saveLoading" :disabled="saveLoading" @click="handleSave">
            {{ $gettext('Save') }}
          </n-button>
        </n-flex>
      </n-flex>
    </n-tab-pane>
    <n-tab-pane name="log" :tab="$gettext('Log')">
      <n-flex vertical>
        <n-alert type="info">
          {{ $gettext('Grafana log settings.') }}
        </n-alert>
        <n-form>
          <n-form-item :label="$gettext('Mode (mode)')">
            <n-select v-model:value="logMode" :options="logModeOptions" clearable />
          </n-form-item>
          <n-form-item :label="$gettext('Level (level)')">
            <n-select v-model:value="logLevel" :options="logLevelOptions" clearable />
          </n-form-item>
        </n-form>
        <n-flex>
          <n-button type="primary" :loading="saveLoading" :disabled="saveLoading" @click="handleSave">
            {{ $gettext('Save') }}
          </n-button>
        </n-flex>
      </n-flex>
    </n-tab-pane>
  </n-tabs>
</template>
