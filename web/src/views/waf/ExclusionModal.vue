<script setup lang="ts">
import { NButton } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import waf from '@/api/panel/waf'

const { $gettext } = useGettext()
const show = defineModel<boolean>('show', { type: Boolean, required: true })
const props = defineProps<{ event: any }>()

const loading = ref(false)
const policies = ref<any[]>([])
const policyId = ref<number | null>(null)

const defaultModel = () => ({
  enabled: true,
  detector: '',
  target: '',
  path_prefix: '',
  remark: '',
})

const model = ref(defaultModel())

const policyOptions = computed(() =>
  policies.value.map((p: any) => ({ label: `${p.name} (#${p.id})`, value: p.id })),
)

const detectorOptions = computed(() => [
  { label: $gettext('Path Traversal'), value: 'path_traversal' },
  { label: $gettext('SQL Injection'), value: 'sqli' },
  { label: $gettext('Command Injection'), value: 'command_injection' },
  { label: $gettext('Cross-Site Scripting'), value: 'xss' },
  { label: $gettext('Server-Side Request Forgery'), value: 'ssrf' },
  { label: $gettext('Code Injection'), value: 'code_injection' },
  { label: $gettext('Unsafe Deserialization'), value: 'unsafe_deserialization' },
  { label: $gettext('XML Injection'), value: 'xml_injection' },
  { label: $gettext('NoSQL Injection'), value: 'nosql_injection' },
  { label: $gettext('Protocol Injection'), value: 'protocol_injection' },
  { label: $gettext('Response Data Leakage'), value: 'response_leakage' },
])

const targetOptions = computed(() => [
  { label: $gettext('All Inputs'), value: '' },
  { label: $gettext('Path'), value: 'path' },
  { label: $gettext('Query'), value: 'query' },
  { label: $gettext('Request Body'), value: 'body' },
  { label: $gettext('Request Header'), value: 'header' },
  { label: $gettext('Response Body'), value: 'response' },
])

watch(show, (v) => {
  if (v) {
    model.value = defaultModel()
    policyId.value = null
    // 预填事件信息
    if (props.event) {
      model.value.detector = String(props.event.rule || '').split(':')[0] ?? ''
      model.value.path_prefix = String(props.event.uri || '').split('?')[0] ?? ''
      model.value.remark = $gettext('From event: %{ rule }', {
        rule: String(props.event.rule || ''),
      })
    }
    // 加载策略供选择
    useRequest(waf.policies()).onSuccess(({ data }: any) => {
      policies.value = Array.isArray(data) ? data : data?.items || []
      // 优先选中事件所属策略
      const evPolicy = props.event?.policy_id
      if (evPolicy && policies.value.some((p: any) => p.id === evPolicy)) {
        policyId.value = evPolicy
      } else if (policies.value.length > 0) {
        policyId.value = policies.value[0].id
      }
    })
  }
})

const handleSubmit = () => {
  if (!policyId.value) {
    window.$message.error($gettext('Please select a policy'))
    return
  }
  if (!model.value.detector) {
    window.$message.error($gettext('Please select a detector'))
    return
  }
  loading.value = true
  useRequest(waf.createExclusion(policyId.value, model.value))
    .onSuccess(() => {
      show.value = false
      window.$message.success($gettext('Added to allowlist successfully'))
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
    :title="$gettext('Add False Positive to Allowlist')"
    style="width: 50vw; max-width: 600px"
    size="huge"
    :bordered="false"
    :segmented="false"
    @close="show = false"
  >
    <n-form label-placement="top">
      <n-form-item :label="$gettext('Target Policy')">
        <n-select
          v-model:value="policyId"
          :options="policyOptions"
          :placeholder="$gettext('Select a policy')"
        />
      </n-form-item>
      <n-grid :cols="24" :x-gap="16">
        <n-form-item-gi :span="24" :label="$gettext('Detector')">
          <n-select v-model:value="model.detector" :options="detectorOptions" />
        </n-form-item-gi>
        <n-form-item-gi :span="12" :label="$gettext('Input Target (optional)')">
          <n-select v-model:value="model.target" :options="targetOptions" />
        </n-form-item-gi>
        <n-form-item-gi :span="12" :label="$gettext('Path Prefix (optional)')">
          <n-input v-model:value="model.path_prefix" placeholder="/path" />
        </n-form-item-gi>
        <n-form-item-gi :span="24" :label="$gettext('Remark')">
          <n-input v-model:value="model.remark" type="textarea" :autosize="{ minRows: 2 }" />
        </n-form-item-gi>
      </n-grid>
    </n-form>
    <n-button type="info" block :loading="loading" :disabled="loading" @click="handleSubmit">
      {{ $gettext('Submit') }}
    </n-button>
  </n-modal>
</template>

<style scoped lang="scss"></style>
