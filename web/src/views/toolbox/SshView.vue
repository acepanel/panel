<script setup lang="ts">
defineOptions({
  name: 'toolbox-ssh'
})

import toolboxSSH from '@/api/panel/toolbox-ssh'
import TheIcon from '@/components/custom/TheIcon.vue'
import { generateRandomString } from '@/utils'
import { useGettext } from 'vue3-gettext'

const { $gettext } = useGettext()

// SSH 基础设置
const sshStatus = ref(false)
const sshPort = ref(22)
const passwordAuth = ref(false)
const pubkeyAuth = ref(true)

// Root 设置
const rootLogin = ref('without-password')
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

// Root 登录选项
const rootLoginOptions = [
  { label: 'yes - ' + $gettext('Allow password and key login'), value: 'yes' },
  { label: 'no - ' + $gettext('Disable root login'), value: 'no' },
  {
    label: 'prohibit-password - ' + $gettext('Only allow key login (recommended)'),
    value: 'prohibit-password'
  },
  {
    label: 'forced-commands-only - ' + $gettext('Only allow key login with forced commands'),
    value: 'forced-commands-only'
  }
]

// 加载数据
const loadData = async () => {
  loading.value = true
  try {
    const info = await toolboxSSH.info()
    sshStatus.value = info.status
    sshPort.value = info.port
    passwordAuth.value = info.password_auth
    pubkeyAuth.value = info.pubkey_auth
    rootLogin.value = info.root_login

    // 加载 root 私钥
    const key = await toolboxSSH.rootKey()
    rootKey.value = key || ''
  } finally {
    loading.value = false
  }
}

// 切换 SSH 服务状态
const handleToggleSSH = async () => {
  loading.value = true
  try {
    if (sshStatus.value) {
      await toolboxSSH.stop()
      window.$message.success($gettext('SSH service stopped'))
    } else {
      await toolboxSSH.start()
      window.$message.success($gettext('SSH service started'))
    }
    sshStatus.value = !sshStatus.value
  } finally {
    loading.value = false
  }
}

// 重启 SSH 服务
const handleRestartSSH = async () => {
  loading.value = true
  try {
    await toolboxSSH.restart()
    window.$message.success($gettext('SSH service restarted'))
  } finally {
    loading.value = false
  }
}

// 更新端口
const handleUpdatePort = async () => {
  portLoading.value = true
  try {
    await toolboxSSH.updatePort(sshPort.value)
    window.$message.success($gettext('SSH port updated'))
  } finally {
    portLoading.value = false
  }
}

// 生成随机端口
const handleRandomPort = () => {
  // 生成 10000-65535 之间的随机端口
  sshPort.value = Math.floor(Math.random() * (65535 - 10000 + 1)) + 10000
}

// 切换密码认证
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

// 切换密钥认证
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

// 更新 Root 登录设置
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

// 更新 Root 密码
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

// 生成随机 Root 密码
const handleGeneratePassword = () => {
  rootPassword.value = generateRandomString(16)
}

// 查看密钥
const showKeyModal = ref(false)
const handleViewKey = async () => {
  if (!rootKey.value) {
    // 没有密钥，先生成一个
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

// 生成密钥
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

// 下载私钥
const handleDownloadKey = () => {
  if (!rootKey.value) {
    window.$message.warning($gettext('No SSH key found'))
    return
  }
  const blob = new Blob([rootKey.value], { type: 'text/plain' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  // 根据私钥内容判断文件名
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
    <n-flex vertical :size="24">
      <!-- SSH 服务 -->
      <n-card :title="$gettext('SSH Service')">
        <n-flex vertical :size="16">
          <n-flex align="center" :size="12">
            <n-text strong>{{ $gettext('SSH Service Status') }}</n-text>
            <n-switch :value="sshStatus" :loading="loading" @update:value="handleToggleSSH" />
            <n-button :loading="loading" @click="handleRestartSSH">
              {{ $gettext('Restart') }}
            </n-button>
          </n-flex>
          <!-- SSH 密码登录 -->
          <n-flex vertical :size="4">
            <n-flex align="center" :size="12">
              <n-text strong>{{ $gettext('SSH Password Login') }}</n-text>
              <n-switch
                :value="passwordAuth"
                :loading="passwordLoading"
                @update:value="handleTogglePasswordAuth"
              />
            </n-flex>
            <n-text depth="3">{{ $gettext('Allow password authentication for SSH login') }}</n-text>
          </n-flex>
          <!-- SSH 密钥登录 -->
          <n-flex vertical :size="4">
            <n-flex align="center" :size="12">
              <n-text strong>{{ $gettext('SSH Key Login') }}</n-text>
              <n-switch
                :value="pubkeyAuth"
                :loading="pubkeyLoading"
                @update:value="handleTogglePubkeyAuth"
              />
            </n-flex>
            <n-text depth="3">{{
              $gettext('Allow public key authentication for SSH login')
            }}</n-text>
          </n-flex>
          <!-- SSH 端口 -->
          <n-flex vertical :size="4">
            <n-flex align="center" :size="12">
              <n-text strong>{{ $gettext('SSH Port') }}</n-text>
              <n-input-number v-model:value="sshPort" :min="1" :max="65535" style="width: 120px" />
              <n-button @click="handleRandomPort">
                <template #icon>
                  <the-icon :size="16" icon="mdi:refresh" />
                </template>
              </n-button>
              <n-button type="primary" :loading="portLoading" @click="handleUpdatePort">
                {{ $gettext('Save') }}
              </n-button>
            </n-flex>
            <n-text depth="3">{{ $gettext('Current SSH port, default is 22') }}</n-text>
          </n-flex>
        </n-flex>
      </n-card>

      <!-- Root 设置 -->
      <n-card :title="$gettext('Root Settings')">
        <n-flex vertical :size="16">
          <!-- Root 密码登录设置 -->
          <n-flex vertical :size="8">
            <n-text strong>{{ $gettext('Root Password Login Setting') }}</n-text>
            <n-select
              :value="rootLogin"
              :options="rootLoginOptions"
              :loading="rootLoginLoading"
              style="max-width: 400px"
              @update:value="handleUpdateRootLogin"
            />
          </n-flex>
          <!-- Root 密码 -->
          <n-flex vertical :size="8">
            <n-text strong>{{ $gettext('Root Password') }}</n-text>
            <n-flex align="center" :size="12">
              <n-input
                v-model:value="rootPassword"
                type="password"
                show-password-on="click"
                :placeholder="$gettext('Enter new password')"
                style="max-width: 300px"
              />
              <n-button @click="handleGeneratePassword">
                <template #icon>
                  <the-icon :size="16" icon="mdi:refresh" />
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
            <n-text depth="3">
              {{
                $gettext(
                  'It is recommended to use a complex password. Save after modification. Refresh will clear the password field.'
                )
              }}
            </n-text>
          </n-flex>
          <!-- Root 密钥 -->
          <n-flex vertical :size="4">
            <n-flex align="center" :size="12">
              <n-text strong>{{ $gettext('Root Key') }}</n-text>
              <n-button type="primary" :loading="keyLoading" @click="handleViewKey">
                {{ $gettext('View Key') }}
              </n-button>
              <n-button :loading="keyLoading" @click="handleDownloadKey">
                {{ $gettext('Download') }}
              </n-button>
            </n-flex>
            <n-text depth="3">
              {{
                $gettext('Recommended to use key login with password disabled for higher security')
              }}
            </n-text>
          </n-flex>
        </n-flex>
      </n-card>
    </n-flex>
  </n-spin>

  <!-- 查看私钥弹窗 -->
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
            'This is the private key of the root user. Keep it safe and use it to login to this server.'
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
        <n-button :loading="keyLoading" @click="handleGenerateKey">
          {{ $gettext('Regenerate') }}
        </n-button>
        <n-button type="primary" @click="handleDownloadKey">
          {{ $gettext('Download Private Key') }}
        </n-button>
      </n-flex>
    </n-flex>
  </n-modal>
</template>

<style scoped lang="scss"></style>
