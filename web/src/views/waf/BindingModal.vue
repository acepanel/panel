<script setup lang="ts">
import { NButton } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import waf from '@/api/panel/waf'
import website from '@/api/panel/website'

const { $gettext } = useGettext()
const show = defineModel<boolean>('show', { type: Boolean, required: true })

const loading = ref(false)
const websites = ref<any[]>([])
const policies = ref<any[]>([])

const websiteId = ref<number | null>(null)
const policyId = ref<number | null>(null)

const websiteOptions = computed(() =>
  websites.value.map((w: any) => ({ label: w.name, value: w.id }))
)
const policyOptions = computed(() =>
  policies.value.map((p: any) => ({ label: `${p.name} (#${p.id})`, value: p.id }))
)

watch(show, (v) => {
  if (v) {
    websiteId.value = null
    policyId.value = null
    // 加载网站列表
    useRequest(website.list('all', 1, 10000)).onSuccess(({ data }: any) => {
      websites.value = data?.items || []
    })
    // 加载策略列表
    useRequest(waf.policies()).onSuccess(({ data }: any) => {
      policies.value = Array.isArray(data) ? data : data?.items || []
    })
  }
})

const handleSubmit = () => {
  if (!websiteId.value) {
    window.$message.error($gettext('Please select a website'))
    return
  }
  if (!policyId.value) {
    window.$message.error($gettext('Please select a policy'))
    return
  }
  loading.value = true
  useRequest(
    waf.enableWebsite({
      website_id: websiteId.value,
      policy_id: policyId.value,
    })
  )
    .onSuccess(() => {
      show.value = false
      window.$message.success($gettext('Enabled successfully'))
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
    :title="$gettext('Enable WAF for Website')"
    style="width: 50vw; max-width: 560px"
    size="huge"
    :bordered="false"
    :segmented="false"
    @close="show = false"
  >
    <n-form label-placement="top">
      <n-form-item :label="$gettext('Website')">
        <n-select
          v-model:value="websiteId"
          :options="websiteOptions"
          filterable
          :placeholder="$gettext('Select a website')"
        />
      </n-form-item>
      <n-form-item :label="$gettext('Policy')">
        <n-select
          v-model:value="policyId"
          :options="policyOptions"
          :placeholder="$gettext('Select a policy')"
        />
      </n-form-item>
    </n-form>
    <n-alert type="info" :bordered="false" class="mb-3">
      {{
        $gettext(
          'This writes the WAF directive into the website nginx config and reloads the web server.'
        )
      }}
    </n-alert>
    <n-button type="info" block :loading="loading" :disabled="loading" @click="handleSubmit">
      {{ $gettext('Submit') }}
    </n-button>
  </n-modal>
</template>

<style scoped lang="scss"></style>
