<script setup lang="ts">
import { computed } from 'vue'
import { useGettext } from 'vue3-gettext'

const { $gettext } = useGettext()

const model = defineModel<any>('model', { type: Object, required: true })

// HTTPS 模式：off / acme / custom
const httpsMode = computed({
  get: () => {
    if (!model.value.https) return 'off'
    return model.value.acme ? 'acme' : 'custom'
  },
  set: (value: string) => {
    switch (value) {
      case 'off':
        model.value.https = false
        model.value.acme = false
        break
      case 'acme':
        model.value.https = true
        model.value.acme = true
        break
      case 'custom':
        model.value.https = true
        model.value.acme = false
        break
    }
  }
})
</script>

<template>
  <n-space vertical>
    <n-form>
      <n-form-item>
        <template #label>
          <n-tooltip>
            <template #trigger>
              <div class="flex items-center">
                {{ $gettext('Login Timeout') }}
                <the-icon :size="16" icon="mdi:help-circle-outline" class="ml-1" />
              </div>
            </template>
            {{
              $gettext(
                'Set the maximum lifetime (in minutes) of the login state, after which you need to log in again'
              )
            }}
          </n-tooltip>
        </template>
        <n-input-number
          v-model:value="model.lifetime"
          :placeholder="$gettext('120')"
          :min="10"
          :max="43200"
          w-full
        >
          <template #suffix>
            {{ $gettext('minutes') }}
          </template>
        </n-input-number>
      </n-form-item>
      <n-form-item>
        <template #label>
          <n-tooltip>
            <template #trigger>
              <div class="flex items-center">
                {{ $gettext('Access Entrance') }}
                <the-icon :size="16" icon="mdi:help-circle-outline" class="ml-1" />
              </div>
            </template>
            {{
              $gettext(
                'Set the access entrance of the panel (e.g. /mypanel) to prevent some malicious access. Leave blank to disable (not recommended)'
              )
            }}
          </n-tooltip>
        </template>
        <n-input v-model:value="model.entrance" />
      </n-form-item>
      <n-form-item>
        <template #label>
          <n-tooltip>
            <template #trigger>
              <div class="flex items-center">
                {{ $gettext('Entrance Error Page') }}
                <the-icon :size="16" icon="mdi:help-circle-outline" class="ml-1" />
              </div>
            </template>
            {{
              $gettext(
                'Set the error page to display when accessing with wrong entrance. 418 shows teapot page, Nginx 404 shows nginx style 404 page, Close Connection will close the connection immediately'
              )
            }}
          </n-tooltip>
        </template>
        <n-select
          v-model:value="model.entrance_error"
          :options="[
            { label: $gettext(`418 I'm a teapot`), value: '418' },
            { label: $gettext('Nginx 404'), value: 'nginx' },
            { label: $gettext('Close Connection'), value: 'close' }
          ]"
          :placeholder="$gettext(`418 I'm a teapot`)"
        />
      </n-form-item>
      <n-form-item>
        <template #label>
          <n-tooltip>
            <template #trigger>
              <div class="flex items-center">
                {{ $gettext('Login Captcha') }}
                <the-icon :size="16" icon="mdi:help-circle-outline" class="ml-1" />
              </div>
            </template>
            {{
              $gettext(
                'When enabled, a captcha will be required after 3 failed login attempts to prevent brute force attacks'
              )
            }}
          </n-tooltip>
        </template>
        <n-switch v-model:value="model.login_captcha" />
      </n-form-item>
      <n-form-item>
        <template #label>
          <n-tooltip>
            <template #trigger>
              <div class="flex items-center">
                {{ $gettext('Request IP Header') }}
                <the-icon :size="16" icon="mdi:help-circle-outline" class="ml-1" />
              </div>
            </template>
            {{
              $gettext(
                'Set the header that carries the real IP of the client, useful when using CDN or reverse proxy. Leave blank to use the client IP directly'
              )
            }}
          </n-tooltip>
        </template>
        <n-input v-model:value="model.ip_header" :placeholder="$gettext('X-Real-IP')" />
      </n-form-item>
      <n-form-item>
        <template #label>
          <n-tooltip>
            <template #trigger>
              <div class="flex items-center">
                {{ $gettext('Bind Domain') }}
                <the-icon :size="16" icon="mdi:help-circle-outline" class="ml-1" />
              </div>
            </template>
            {{
              $gettext(
                'Restrict panel access to the specified domain names. Leave blank to allow access from any domain'
              )
            }}
          </n-tooltip>
        </template>
        <n-dynamic-input
          v-model:value="model.bind_domain"
          placeholder="example.com"
          show-sort-button
        />
      </n-form-item>
      <n-form-item>
        <template #label>
          <n-tooltip>
            <template #trigger>
              <div class="flex items-center">
                {{ $gettext('Bind IP') }}
                <the-icon :size="16" icon="mdi:help-circle-outline" class="ml-1" />
              </div>
            </template>
            {{
              $gettext(
                'Restrict panel access to the specified IP addresses. Leave blank to allow access from any IP'
              )
            }}
          </n-tooltip>
        </template>
        <n-dynamic-input v-model:value="model.bind_ip" placeholder="127.0.0.1" show-sort-button />
      </n-form-item>
      <n-form-item>
        <template #label>
          <n-tooltip>
            <template #trigger>
              <div class="flex items-center">
                {{ $gettext('Bind UA') }}
                <the-icon :size="16" icon="mdi:help-circle-outline" class="ml-1" />
              </div>
            </template>
            {{
              $gettext(
                'Restrict panel access to the specified User-Agent strings. Leave blank to allow access from any User-Agent'
              )
            }}
          </n-tooltip>
        </template>
        <n-dynamic-input
          v-model:value="model.bind_ua"
          placeholder="Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Safari/537.36"
          show-sort-button
        />
      </n-form-item>
      <n-form-item>
        <template #label>
          <n-tooltip>
            <template #trigger>
              <div class="flex items-center">
                {{ $gettext('Offline Mode') }}
                <the-icon :size="16" icon="mdi:help-circle-outline" class="ml-1" />
              </div>
            </template>
            {{
              $gettext(
                'When enabled, the panel will not attempt to connect to external services for updates or other features. This may limit some functionalities'
              )
            }}
          </n-tooltip>
        </template>
        <n-switch v-model:value="model.offline_mode" />
      </n-form-item>
      <n-form-item>
        <template #label>
          <n-tooltip>
            <template #trigger>
              <div class="flex items-center">
                {{ $gettext('Auto Update') }}
                <the-icon :size="16" icon="mdi:help-circle-outline" class="ml-1" />
              </div>
            </template>
            {{
              $gettext(
                'When enabled, the panel will automatically check for and install updates when they are available. It is recommended to keep this enabled to ensure you have the latest features and security patches'
              )
            }}
          </n-tooltip>
        </template>
        <n-switch v-model:value="model.auto_update" />
      </n-form-item>
      <n-form-item>
        <template #label>
          <n-tooltip>
            <template #trigger>
              <div class="flex items-center">
                {{ $gettext('Panel HTTPS') }}
                <the-icon :size="16" icon="mdi:help-circle-outline" class="ml-1" />
              </div>
            </template>
            {{
              $gettext(
                'Enable HTTPS for the panel. ACME will automatically obtain and renew certificates (requires panel accessible via public IP). Custom allows you to provide your own certificate'
              )
            }}
          </n-tooltip>
        </template>
        <n-radio-group v-model:value="httpsMode">
          <n-radio-button
            v-for="option in [
              { label: $gettext('Disabled'), value: 'off' },
              { label: $gettext('ACME (Auto)'), value: 'acme' },
              { label: $gettext('Custom Certificate'), value: 'custom' }
            ]"
            :key="option.value"
            :value="option.value"
            :label="option.label"
          />
        </n-radio-group>
      </n-form-item>
      <n-form-item v-if="httpsMode === 'acme'" :label="$gettext('Panel Public IP')">
        <template #label>
          <n-tooltip>
            <template #trigger>
              <div class="flex items-center">
                {{ $gettext('Panel Public IP') }}
                <the-icon :size="16" icon="mdi:help-circle-outline" class="ml-1" />
              </div>
            </template>
            {{
              $gettext(
                'Panel public IP is used to issue HTTPS certificates using ACME. Ensure that the entered IP address is accessible from the public network.'
              )
            }}
          </n-tooltip>
        </template>
        <n-dynamic-input v-model:value="model.public_ip" placeholder="127.0.0.1" show-sort-button />
      </n-form-item>
      <n-form-item v-if="httpsMode === 'custom'" :label="$gettext('Certificate')">
        <n-input
          v-model:value="model.cert"
          type="textarea"
          :autosize="{ minRows: 10, maxRows: 15 }"
        />
      </n-form-item>
      <n-form-item v-if="httpsMode === 'custom'" :label="$gettext('Private Key')">
        <n-input
          v-model:value="model.key"
          type="textarea"
          :autosize="{ minRows: 10, maxRows: 15 }"
        />
      </n-form-item>
    </n-form>
  </n-space>
</template>

<style scoped lang="scss"></style>
