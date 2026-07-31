<script setup lang="ts">
import { NButton } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import waf, { type WafDecisionInput } from '@/api/panel/waf'

const { $gettext } = useGettext()
const show = defineModel<boolean>('show', { type: Boolean, required: true })

const loading = ref(false)

type DecisionForm = {
  type: WafDecisionInput['type']
  value: string
  duration: number
}

const defaultModel = (): DecisionForm => ({
  type: 'ban',
  value: '',
  duration: 0, // 拉黑时长（小时），0=永久；提交时换算为 until
})

const model = ref(defaultModel())

watch(show, (v) => {
  if (v) model.value = defaultModel()
})

const typeOptions = computed(() => [
  { label: $gettext('Ban'), value: 'ban' },
  { label: $gettext('Allow'), value: 'allow' },
  { label: $gettext('Captcha'), value: 'captcha' },
])

const handleSubmit = () => {
  if (!model.value.value) {
    window.$message.error($gettext('Please enter an IP or CIDR'))
    return
  }
  // duration 小时换算为绝对过期时间戳（秒），0 表示永久
  const until =
    model.value.duration > 0 ? Math.floor(Date.now() / 1000) + model.value.duration * 3600 : 0
  loading.value = true
  useRequest(
    waf.createDecision({
      type: model.value.type,
      value: model.value.value,
      until,
    }),
  )
    .onSuccess(() => {
      show.value = false
      window.$message.success($gettext('Added successfully'))
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
    :title="$gettext('Add Allow/Deny Entry')"
    style="width: 50vw; max-width: 560px"
    size="huge"
    :bordered="false"
    :segmented="false"
    @close="show = false"
  >
    <n-form :model="model" label-placement="top">
      <n-grid :cols="24" :x-gap="16">
        <n-form-item-gi :span="24" :label="$gettext('Type')">
          <n-select v-model:value="model.type" :options="typeOptions" />
        </n-form-item-gi>
        <n-form-item-gi :span="24" :label="$gettext('IP / CIDR')">
          <n-input
            v-model:value="model.value"
            :placeholder="$gettext('e.g., 1.2.3.4 or 1.2.3.0/24')"
          />
        </n-form-item-gi>
        <n-form-item-gi
          v-if="model.type !== 'allow'"
          :span="24"
          :label="$gettext('Duration in hours (0 = permanent)')"
        >
          <n-input-number v-model:value="model.duration" :min="0" w-full />
        </n-form-item-gi>
      </n-grid>
    </n-form>
    <n-button type="info" block :loading="loading" :disabled="loading" @click="handleSubmit">
      {{ $gettext('Submit') }}
    </n-button>
  </n-modal>
</template>

<style scoped lang="scss"></style>
