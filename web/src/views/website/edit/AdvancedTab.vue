<script setup lang="ts">
import type { SelectOption } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import { useConfirm } from '@/components/system/composables/useConfirm'
import { useFormLayout } from '@/components/system/composables/useFormLayout'
import type { WebServerFeatures } from '@/utils'

import { generateId } from './id'

const setting = defineModel<any>('setting', { required: true })
defineProps<{
  features: WebServerFeatures
  phpOptions: SelectOption[]
  switching: boolean
  loading: boolean
}>()
const emit = defineEmits<{
  switchType: [target: { type: string; php: number | null; proxy: string }]
}>()

const { $gettext } = useGettext()
const { confirmAction } = useConfirm()
const formLayout = useFormLayout(140)

// ========== 网站类型切换 ==========
const targetType = ref('')
const targetPHP = ref<number | null>(null)
const targetProxy = ref('')

// 配置重新加载后清空切换目标
watch(setting, () => {
  targetType.value = ''
  targetPHP.value = null
  targetProxy.value = ''
})

const websiteTypeOptions = computed(() => [
  { label: $gettext('Reverse Proxy'), value: 'proxy', disabled: setting.value.type === 'proxy' },
  { label: $gettext('PHP'), value: 'php', disabled: setting.value.type === 'php' },
  { label: $gettext('Pure Static'), value: 'static', disabled: setting.value.type === 'static' },
])
const websiteTypeLabel = (type: string) =>
  websiteTypeOptions.value.find((item) => item.value === type)?.label ?? type
const canSwitchType = computed(() => {
  if (!targetType.value || targetType.value === setting.value.type) {
    return false
  }
  if (targetType.value === 'php') {
    return Boolean(targetPHP.value)
  }
  if (targetType.value === 'proxy') {
    return targetProxy.value.trim() !== ''
  }
  return true
})

const handleSwitchType = async () => {
  const from = websiteTypeLabel(setting.value.type)
  const to = websiteTypeLabel(targetType.value)
  const ok = await confirmAction({
    title: $gettext('Confirm Website Type Switch'),
    content: $gettext(
      'Switch the website type from %{ from } to %{ to }? Common settings and website files will be preserved. The original type-specific configuration will be deleted and rebuilt for the new type.',
      { from, to },
    ),
    positiveText: $gettext('Confirm Switch'),
    negativeText: $gettext('Cancel'),
  })
  if (!ok) return
  emit('switchType', {
    type: targetType.value,
    php: targetType.value === 'php' ? targetPHP.value : 0,
    proxy: targetType.value === 'proxy' ? targetProxy.value.trim() : '',
  })
}

// ========== 日志设置、限流限速、真实 IP ==========
// 默认日志路径
const defaultAccessLog = computed(() => `/opt/ace/sites/${setting.value.name}/log/access.log`)
const defaultErrorLog = computed(() => `/opt/ace/sites/${setting.value.name}/log/error.log`)

// 日志路径选项
const accessLogOptions = computed(() => [
  { label: $gettext('Disabled'), value: 'off' },
  { label: $gettext('Default Path'), value: defaultAccessLog.value },
])
const errorLogOptions = computed(() => [
  { label: $gettext('Disabled'), value: 'off' },
  { label: $gettext('Default Path'), value: defaultErrorLog.value },
])

// 限流限速是否启用
const rateLimitEnabled = computed({
  get: () => setting.value.rate_limit !== null,
  set: (value: boolean) => {
    if (value) {
      setting.value.rate_limit = {
        per_server: 0,
        per_ip: 0,
        rate: 0,
      }
    } else {
      setting.value.rate_limit = null
    }
  },
})

// 真实 IP 是否启用
const realIPEnabled = computed({
  get: () => setting.value.real_ip !== null,
  set: (value: boolean) => {
    if (value) {
      setting.value.real_ip = {
        from: [],
        header: 'X-Real-IP',
        recursive: false,
      }
    } else {
      setting.value.real_ip = null
    }
  },
})

// 真实 IP 来源列表，多行文本与数组双向转换，空行与空白在保存时清理
const realIPFrom = computed({
  get: () => setting.value.real_ip?.from?.join('\n') ?? '',
  set: (value: string) => {
    if (setting.value.real_ip) {
      setting.value.real_ip.from = value.split('\n')
    }
  },
})

// ========== 基本认证 ==========
// 是否已存在其他整站规则（整站规则最多一条）
const hasSiteBasicAuth = (self?: any) =>
  setting.value.basic_auth.some((a: any) => a !== self && a._scope === 'site')

// 添加基本认证规则
const addBasicAuth = () => {
  const rule: any = { _id: generateId(), users: {} }
  setBasicAuthScope(rule, hasSiteBasicAuth() ? 'dir' : 'site')
  setting.value.basic_auth.push(rule)
}

// 删除基本认证规则
const removeBasicAuth = (index: number) => {
  setting.value.basic_auth.splice(index, 1)
}

// 切换基本认证规则的生效范围
const setBasicAuthScope = (auth: any, scope: string) => {
  auth._scope = scope
  auth.path = scope === 'site' ? '/' : ''
}
</script>

<template>
  <n-collapse accordion>
    <n-collapse-item :title="$gettext('Website Type')" name="website_type">
      <n-form v-bind="formLayout">
        <n-form-item :label="$gettext('Current Website Type')">
          <n-tag>{{ websiteTypeLabel(setting.type) }}</n-tag>
        </n-form-item>
        <n-form-item :label="$gettext('Target Website Type')">
          <n-select
            v-model:value="targetType"
            :options="websiteTypeOptions"
            :placeholder="$gettext('Select Website Type')"
          />
        </n-form-item>
        <n-form-item v-if="targetType === 'php'" :label="$gettext('PHP Version')">
          <n-select
            v-model:value="targetPHP"
            :options="phpOptions"
            :placeholder="$gettext('Select PHP Version')"
          />
        </n-form-item>
        <n-form-item v-if="targetType === 'proxy'" :label="$gettext('Proxy Target')">
          <n-input
            v-model:value="targetProxy"
            :placeholder="$gettext('For example: http://127.0.0.1:3000')"
          />
        </n-form-item>
        <n-alert type="warning" mb-4>
          {{
            $gettext(
              'After switching, common settings and website files will be preserved. The original type-specific configuration will be deleted and rebuilt for the new type.',
            )
          }}
        </n-alert>
        <app-button
          type="warning"
          :loading="switching"
          :disabled="!canSwitchType || switching || loading"
          @click="handleSwitchType"
        >
          {{ $gettext('Switch Website Type') }}
        </app-button>
      </n-form>
    </n-collapse-item>

    <!-- 访问统计 -->
    <n-collapse-item
      v-if="features.stat"
      :title="$gettext('Access Statistics')"
      name="stat_settings"
    >
      <n-form v-bind="formLayout">
        <n-form-item :label="$gettext('Enable Statistics')">
          <n-switch v-model:value="setting.stat_enabled" />
        </n-form-item>
      </n-form>
    </n-collapse-item>

    <!-- LiteSpeed 页面缓存 -->
    <n-collapse-item v-if="features.lsCache" title="LSCache" name="lscache_settings">
      <n-alert type="info" mb-4>
        {{
          $gettext(
            'Enables the LiteSpeed cache module for this site. Pages are cached only when the application asks for it, e.g. WordPress with the LiteSpeed Cache plugin.',
          )
        }}
      </n-alert>
      <n-form v-bind="formLayout">
        <n-form-item :label="$gettext('Enable LSCache')">
          <n-switch v-model:value="setting.lscache" />
        </n-form-item>
      </n-form>
    </n-collapse-item>

    <!-- 日志设置 -->
    <n-collapse-item :title="$gettext('Log Settings')" name="log_settings">
      <n-form v-bind="formLayout">
        <n-form-item :label="$gettext('Access Log')">
          <n-select
            v-model:value="setting.access_log"
            :options="accessLogOptions"
            :placeholder="defaultAccessLog"
            filterable
            tag
          />
        </n-form-item>
        <n-form-item v-if="features.errorLog" :label="$gettext('Error Log')">
          <n-select
            v-model:value="setting.error_log"
            :options="errorLogOptions"
            :placeholder="defaultErrorLog"
            filterable
            tag
          />
        </n-form-item>
      </n-form>
    </n-collapse-item>

    <!-- 限流限速设置 -->
    <n-collapse-item v-if="features.rateLimit" :title="$gettext('Rate Limiting')" name="rate_limit">
      <n-form v-bind="formLayout">
        <n-form-item :label="$gettext('Enable Rate Limiting')">
          <n-switch v-model:value="rateLimitEnabled" />
        </n-form-item>
        <template v-if="rateLimitEnabled && setting.rate_limit">
          <n-form-item v-if="features.rateLimitConn" :label="$gettext('Concurrent Limit')">
            <n-input-number
              :value="setting.rate_limit.per_server || null"
              :min="0"
              :max="100000"
              w-full
              :placeholder="$gettext('Unlimited')"
              @update:value="(v: number | null) => (setting.rate_limit.per_server = v ?? 0)"
            />
            <template #feedback>
              {{ $gettext('Limit the maximum concurrent connections for this site') }}
            </template>
          </n-form-item>
          <n-form-item v-if="features.rateLimitConn" :label="$gettext('Per IP Limit')">
            <n-input-number
              :value="setting.rate_limit.per_ip || null"
              :min="0"
              :max="10000"
              w-full
              :placeholder="$gettext('Unlimited')"
              @update:value="(v: number | null) => (setting.rate_limit.per_ip = v ?? 0)"
            />
            <template #feedback>
              {{ $gettext('Limit the maximum concurrent connections per IP') }}
            </template>
          </n-form-item>
          <n-form-item :label="$gettext('Rate Limit')">
            <n-input-number
              :value="setting.rate_limit.rate || null"
              :min="0"
              :max="1000000"
              w-full
              :placeholder="$gettext('Unlimited')"
              @update:value="(v: number | null) => (setting.rate_limit.rate = v ?? 0)"
            />
            <template #feedback>
              {{ $gettext('Limit the rate of each request (unit: KB)') }}
            </template>
          </n-form-item>
        </template>
      </n-form>
    </n-collapse-item>

    <!-- 真实 IP 设置 -->
    <n-collapse-item v-if="features.realIP" :title="$gettext('Real IP')" name="real_ip">
      <n-alert type="info" mb-4>
        {{
          $gettext('Configure trusted proxy IPs (e.g., CDN or Frp) to identify real visitor IPs.')
        }}
      </n-alert>
      <n-alert type="warning" mb-4>
        {{
          $gettext(
            'If using Frp, fill in the Frp IP address (e.g., 127.0.0.1). If using CDN, fill in the CDN IP ranges. If unsure, you can fill in 0.0.0.0/0 (ipv4) or ::/0 (ipv6) [insecure].',
          )
        }}
      </n-alert>
      <n-form v-bind="formLayout">
        <n-form-item :label="$gettext('Enable')">
          <n-switch v-model:value="realIPEnabled" />
        </n-form-item>
        <template v-if="realIPEnabled && setting.real_ip">
          <n-form-item :label="$gettext('IP Sources')">
            <n-input
              v-model:value="realIPFrom"
              type="textarea"
              :placeholder="$gettext('One per line, e.g., 127.0.0.1 or 10.0.0.0/8')"
              :autosize="{ minRows: 3, maxRows: 10 }"
            />
          </n-form-item>
          <n-form-item :label="$gettext('IP Header')">
            <n-select
              v-model:value="setting.real_ip.header"
              :options="[
                { label: 'X-Real-IP', value: 'X-Real-IP' },
                { label: 'X-Forwarded-For', value: 'X-Forwarded-For' },
                { label: 'CF-Connecting-IP', value: 'CF-Connecting-IP' },
                { label: 'True-Client-IP', value: 'True-Client-IP' },
                { label: 'Ali-Cdn-Real-Ip', value: 'Ali-Cdn-Real-Ip' },
                { label: 'EO-Connecting-IP', value: 'EO-Connecting-IP' },
              ]"
              filterable
              tag
            />
          </n-form-item>
          <n-form-item v-if="features.realIPRecursive" :label="$gettext('Recursive')">
            <n-switch v-model:value="setting.real_ip.recursive" />
            <template #feedback>
              {{ $gettext('Recursively search for real IP in X-Forwarded-For header') }}
            </template>
          </n-form-item>
        </template>
      </n-form>
    </n-collapse-item>

    <!-- 基本认证设置 -->
    <n-collapse-item :title="$gettext('Basic Authentication')" name="basic_auth">
      <n-flex vertical>
        <n-card
          v-for="(auth, index) in setting.basic_auth as any[]"
          :key="auth._id"
          closable
          @close="removeBasicAuth(index)"
        >
          <n-form v-bind="formLayout">
            <n-form-item :label="$gettext('Scope')">
              <n-radio-group
                :value="auth._scope"
                @update:value="(v: string) => setBasicAuthScope(auth, v)"
              >
                <n-radio value="site" :disabled="hasSiteBasicAuth(auth)">
                  {{ $gettext('Entire Website') }}
                </n-radio>
                <n-radio value="dir">{{ $gettext('Specific Directory') }}</n-radio>
              </n-radio-group>
            </n-form-item>
            <n-form-item v-if="auth._scope === 'dir'" :label="$gettext('Directory')">
              <n-input v-model:value="auth.path" placeholder="/admin" />
            </n-form-item>
            <n-form-item :label="$gettext('User Credentials')">
              <key-value-editor
                v-model="auth.users"
                :key-placeholder="$gettext('Username')"
                :value-placeholder="$gettext('Password')"
                :add-button-text="$gettext('Add User')"
                default-key-prefix="user"
                value-type="password"
                :show-password-toggle="true"
              />
            </n-form-item>
          </n-form>
        </n-card>
        <n-button type="primary" dashed @click="addBasicAuth">
          {{ $gettext('Add Rule') }}
        </n-button>
        <n-alert v-if="setting.basic_auth.length > 0" type="info">
          {{
            $gettext(
              'Visitors will need to enter a username and password to access the protected directories.',
            )
          }}
        </n-alert>
      </n-flex>
    </n-collapse-item>
  </n-collapse>
</template>
