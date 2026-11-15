<script setup lang="ts">
defineOptions({
  name: 'prometheus-config-tune',
})

import { useGettext } from 'vue3-gettext'

import prometheus from '@/api/apps/prometheus'
import type { TuneGroup } from '@/utils/tune'

const { $gettext } = useGettext()

const groups: TuneGroup[] = [
  {
    name: 'general',
    tab: $gettext('General'),
    tip: $gettext('Prometheus global scrape and evaluation settings.'),
    fields: [
      {
        key: 'scrape_interval',
        label: $gettext('Scrape Interval (scrape_interval)'),
        type: 'text',
        placeholder: $gettext('e.g. 15s'),
      },
      {
        key: 'evaluation_interval',
        label: $gettext('Evaluation Interval (evaluation_interval)'),
        type: 'text',
        placeholder: $gettext('e.g. 15s'),
      },
      {
        key: 'scrape_timeout',
        label: $gettext('Scrape Timeout (scrape_timeout)'),
        type: 'text',
        placeholder: $gettext('e.g. 10s'),
      },
    ],
  },
]
</script>

<template>
  <config-tune-form
    :fetch="prometheus.configTune"
    :save="prometheus.saveConfigTune"
    :groups="groups"
  />
</template>
