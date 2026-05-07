<script setup lang="ts">
defineOptions({
  name: 'toolbox-ssh',
})

import { useGettext } from 'vue3-gettext'

import toolboxSSH from '@/api/panel/toolbox-ssh'
import ServiceStatus from '@/components/common/ServiceStatus.vue'
import { generateRandomString } from '@/utils'

const { $gettext } = useGettext()

const currentTab = ref('auth')

// SSH 基础设置
const service = ref('')
const sshPort = ref(22)
const passwordAuth = ref(false)
const pubkeyAuth = ref(true)

// Root 设置
const rootLogin = ref('yes')
const rootPassword = ref('')
const rootKey = ref('')

// 加载状态
const loading = ref(false)
const portLoading = ref(false)
const passwordLoading = ref(false)
const pubkeyLoading = ref(false)
const rootLoginLoading = ref(false)
const rootPasswordLoading = ref(false)
const keyLoading = ref(false)

const rootLoginOptions = [
  { label: $gettext('Allow SSH login'), value: 'yes' },
  { label: $gettext('Disable SSH login'), value: 'no' },
  { label: $gettext('Only allow key login'), value: 'prohibit-password' },
  {
    label: $gettext('Only allow key login with predefined commands'),
    value: 'forced-commands-only',
  },
]

const loadData = async () => {
  loading.value = true
  try {
    const info = await toolboxSSH.info()
    service.value = info.service
    sshPort.value = info.port
    passwordAuth.value = info.password_auth
    pubkeyAuth.value = info.pubkey_auth
    rootLogin.value = info.root_login
    const key = await toolboxSSH.rootKey()
    rootKey.value = key || ''
  } finally {
    loading.value = false
  }
}

const handleUpdatePort = async () => {
  portLoading.value = true
  try {
    await toolboxSSH.updatePort(sshPort.value)
    window.$message.success($gettext('SSH port updated'))
  } finally {
    portLoading.value = false
  }
}

const handleRandomPort = () => {
  sshPort.value = Math.floor(Math.random() * (65535 - 10000 + 1)) + 10000
}

const handleTogglePasswordAuth = async () => {
  passwordLoading.value = true
  try {
    await toolboxSSH.updatePasswordAuth(!passwordAuth.value)
    passwordAuth.value = !passwordAuth.value
    window.$message.success($gettext('Password authentication updated'))
  } finally {
    passwordLoading.value = false
  }
}

const handleTogglePubkeyAuth = async () => {
  pubkeyLoading.value = true
  try {
    await toolboxSSH.updatePubkeyAuth(!pubkeyAuth.value)
    pubkeyAuth.value = !pubkeyAuth.value
    window.$message.success($gettext('Key authentication updated'))
  } finally {
    pubkeyLoading.value = false
  }
}

const handleUpdateRootLogin = async (value: string) => {
  rootLoginLoading.value = true
  try {
    await toolboxSSH.updateRootLogin(value)
    rootLogin.value = value
    window.$message.success($gettext('Root login setting updated'))
  } finally {
    rootLoginLoading.value = false
  }
}

const handleUpdateRootPassword = async () => {
  if (!rootPassword.value) {
    window.$message.warning($gettext('Please enter a password'))
    return
  }
  rootPasswordLoading.value = true
  try {
    await toolboxSSH.updateRootPassword(rootPassword.value)
    rootPassword.value = ''
    window.$message.success($gettext('Root password updated'))
  } finally {
    rootPasswordLoading.value = false
  }
}

const handleGeneratePassword = () => {
  rootPassword.value = generateRandomString(16)
}

const showKeyModal = ref(false)
const handleViewKey = async () => {
  if (!rootKey.value) {
    keyLoading.value = true
    try {
      const key = await toolboxSSH.generateRootKey()
      rootKey.value = key
      window.$message.success($gettext('SSH key generated'))
    } finally {
      keyLoading.value = false
    }
  }
  showKeyModal.value = true
}

const handleGenerateKey = async () => {
  keyLoading.value = true
  try {
    const key = await toolboxSSH.generateRootKey()
    rootKey.value = key
    window.$message.success($gettext('SSH key generated'))
  } finally {
    keyLoading.value = false
  }
}

const handleDownloadKey = () => {
  if (!rootKey.value) {
    window.$message.warning($gettext('No SSH key found'))
    return
  }
  const blob = new Blob([rootKey.value], { type: 'text/plain' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  if (rootKey.value.includes('OPENSSH PRIVATE KEY')) {
    link.download = 'id_ed25519'
  } else if (rootKey.value.includes('RSA PRIVATE KEY')) {
    link.download = 'id_rsa'
  } else {
    link.download = 'id_key'
  }
  link.click()
  URL.revokeObjectURL(url)
}

onMounted(() => {
  loadData()
})
</script>

<template>
  <n-spin :show="loading">
    <n-flex vertical :size="16">
      <service-status v-if="service != ''" :service="service" />

      <n-tabs v-model:value="currentTab" type="line" placement="left" animated>
        <!-- 基础设置 -->
        <n-tab-pane name="auth" :tab="$gettext('Service & Auth')">
          <n-flex vertical :size="0">
            <!-- SSH 端口 -->
            <div class="setting-row">
              <div class="setting-row__info">
                <div class="setting-row__title">{{ $gettext('SSH Port') }}</div>
                <div class="setting-row__desc">
                  {{ $gettext('Current SSH port, default is 22') }}
                </div>
              </div>
              <n-flex :size="8" align="center">
                <n-input-number
                  v-model:value="sshPort"
                  :min="1"
                  :max="65535"
                  :show-button="false"
                  class="!w-25"
                />
                <n-button
                  quaternary
                  circle
                  :title="$gettext('Random Port')"
                  @click="handleRandomPort"
                >
                  <template #icon>
                    <i-mdi-refresh />
                  </template>
                </n-button>
                <n-button
                  type="primary"
                  :loading="portLoading"
                  :disabled="portLoading"
                  @click="handleUpdatePort"
                >
                  {{ $gettext('Save') }}
                </n-button>
              </n-flex>
            </div>

            <n-divider class="!my-4" />

            <!-- 密码登录 -->
            <div class="setting-row">
              <div class="setting-row__info">
                <div class="setting-row__title">{{ $gettext('Password Login') }}</div>
                <div class="setting-row__desc">
                  {{ $gettext('Allow password authentication for SSH login') }}
                </div>
              </div>
              <n-switch
                :value="passwordAuth"
                :loading="passwordLoading"
                @update:value="handleTogglePasswordAuth"
              />
            </div>

            <n-divider class="!my-4" />

            <!-- 密钥登录 -->
            <div class="setting-row">
              <div class="setting-row__info">
                <div class="setting-row__title">{{ $gettext('Key Login') }}</div>
                <div class="setting-row__desc">
                  {{ $gettext('Allow key authentication for SSH login') }}
                </div>
              </div>
              <n-switch
                :value="pubkeyAuth"
                :loading="pubkeyLoading"
                @update:value="handleTogglePubkeyAuth"
              />
            </div>
          </n-flex>
        </n-tab-pane>

        <!-- Root 账户 -->
        <n-tab-pane name="root" :tab="$gettext('Root Account')">
          <n-flex vertical :size="0">
            <!-- Root 登录策略 -->
            <div class="setting-row">
              <div class="setting-row__info">
                <div class="setting-row__title">{{ $gettext('Root Login Policy') }}</div>
                <div class="setting-row__desc">
                  {{ $gettext('Control how root user can login via SSH') }}
                </div>
              </div>
              <n-select
                :value="rootLogin"
                :options="rootLoginOptions"
                :loading="rootLoginLoading"
                class="!w-87"
                @update:value="handleUpdateRootLogin"
              />
            </div>

            <n-divider class="!my-4" />

            <!-- Root 密码 -->
            <div class="setting-row setting-row--vertical">
              <div class="setting-row__info">
                <div class="setting-row__title">{{ $gettext('Reset Root Password') }}</div>
                <div class="setting-row__desc">
                  {{ $gettext('Use a complex password. Field clears after refresh.') }}
                </div>
              </div>
              <n-flex :size="8" align="center" class="!w-full">
                <n-input
                  v-model:value="rootPassword"
                  type="password"
                  show-password-on="click"
                  :placeholder="$gettext('Enter new password')"
                  class="!w-75"
                />
                <n-button
                  quaternary
                  circle
                  :title="$gettext('Generate Random')"
                  @click="handleGeneratePassword"
                >
                  <template #icon>
                    <i-mdi-refresh />
                  </template>
                </n-button>
                <n-button
                  type="warning"
                  :loading="rootPasswordLoading"
                  @click="handleUpdateRootPassword"
                >
                  {{ $gettext('Reset') }}
                </n-button>
              </n-flex>
            </div>

            <n-divider class="!my-4" />

            <!-- Root 密钥 -->
            <div class="setting-row setting-row--vertical">
              <div class="setting-row__info">
                <div class="setting-row__title">{{ $gettext('Root SSH Key') }}</div>
                <div class="setting-row__desc">
                  {{ $gettext('Use key login with password disabled for higher security') }}
                </div>
              </div>
              <n-flex :size="8">
                <n-button
                  type="primary"
                  :loading="keyLoading"
                  :disabled="keyLoading"
                  @click="handleViewKey"
                >
                  <template #icon>
                    <i-mdi-eye-outline />
                  </template>
                  {{ $gettext('View Key') }}
                </n-button>
                <n-button :loading="keyLoading" :disabled="keyLoading" @click="handleDownloadKey">
                  <template #icon>
                    <i-mdi-download />
                  </template>
                  {{ $gettext('Download') }}
                </n-button>
              </n-flex>
            </div>
          </n-flex>
        </n-tab-pane>
      </n-tabs>
    </n-flex>
  </n-spin>

  <n-modal
    v-model:show="showKeyModal"
    preset="card"
    :title="$gettext('Root Private Key')"
    style="width: 60vw"
    :bordered="false"
  >
    <n-flex vertical :size="16">
      <n-alert type="warning">
        {{
          $gettext(
            'This is the private key of the root user. Keep it safe and use it to login to this server.',
          )
        }}
      </n-alert>
      <n-input
        :value="rootKey"
        type="textarea"
        :rows="10"
        readonly
        :placeholder="$gettext('No private key generated')"
      />
      <n-flex justify="end" :size="12">
        <n-button :loading="keyLoading" :disabled="keyLoading" @click="handleGenerateKey">
          {{ $gettext('Regenerate') }}
        </n-button>
        <n-button type="primary" @click="handleDownloadKey">
          {{ $gettext('Download Private Key') }}
        </n-button>
      </n-flex>
    </n-flex>
  </n-modal>
</template>

<style scoped lang="scss">
.setting-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 12px 0;

  &--vertical {
    flex-direction: column;
    align-items: flex-start;
  }
}

.setting-row__info {
  flex: 1;
  min-width: 0;
}

.setting-row__title {
  font-size: 14px;
  font-weight: 500;
  color: var(--color-text-primary);
}

.setting-row__desc {
  margin-top: 4px;
  font-size: 13px;
  color: var(--color-text-tertiary);
  line-height: 1.5;
}
</style>
