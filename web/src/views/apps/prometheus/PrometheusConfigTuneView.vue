<script setup lang="ts">
defineOptions({
  name: 'prometheus-config-tune'
})

import { useGettext } from 'vue3-gettext'

import prometheus from '@/api/apps/prometheus'

const { $gettext } = useGettext()

const scrapeInterval = ref('')
const evaluationInterval = ref('')
const scrapeTimeout = ref('')

const saveLoading = ref(false)

useRequest(prometheus.configTune()).onSuccess(({ data }: any) => {
  scrapeInterval.value = data.scrape_interval ?? ''
  evaluationInterval.value = data.evaluation_interval ?? ''
  scrapeTimeout.value = data.scrape_timeout ?? ''
})

const getConfigData = () => ({
  scrape_interval: scrapeInterval.value,
  evaluation_interval: evaluationInterval.value,
  scrape_timeout: scrapeTimeout.value
})

const handleSave = () => {
  saveLoading.value = true
  useRequest(prometheus.saveConfigTune(getConfigData()))
    .onSuccess(() => {
      window.$message.success($gettext('Saved successfully'))
    })
    .onComplete(() => {
      saveLoading.value = false
    })
}
</script>

<template>
  <n-flex vertical>
    <n-alert type="info">
      {{ $gettext('Prometheus global scrape and evaluation settings.') }}
    </n-alert>
    <n-form>
      <n-form-item :label="$gettext('Scrape Interval (scrape_interval)')">
        <n-input v-model:value="scrapeInterval" :placeholder="$gettext('e.g. 15s')" />
      </n-form-item>
      <n-form-item :label="$gettext('Evaluation Interval (evaluation_interval)')">
        <n-input v-model:value="evaluationInterval" :placeholder="$gettext('e.g. 15s')" />
      </n-form-item>
      <n-form-item :label="$gettext('Scrape Timeout (scrape_timeout)')">
        <n-input v-model:value="scrapeTimeout" :placeholder="$gettext('e.g. 10s')" />
      </n-form-item>
    </n-form>
    <n-flex>
      <n-button type="primary" :loading="saveLoading" :disabled="saveLoading" @click="handleSave">
        {{ $gettext('Save') }}
      </n-button>
    </n-flex>
  </n-flex>
</template>
