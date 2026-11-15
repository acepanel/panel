<script setup lang="ts">
defineOptions({
  name: 'clickhouse-config-tune',
})

import { useGettext } from 'vue3-gettext'

import clickhouse from '@/api/apps/clickhouse'
import { toOptions, type TuneGroup } from '@/utils/tune'

const { $gettext } = useGettext()

const groups: TuneGroup[] = [
  {
    name: 'network',
    tab: $gettext('Network'),
    tip: $gettext('ClickHouse network settings.'),
    fields: [
      {
        key: 'listen_host',
        label: $gettext('Listen Host'),
        type: 'text',
        placeholder: '127.0.0.1',
      },
      { key: 'http_port', label: $gettext('HTTP Port'), type: 'text', placeholder: '8123' },
      { key: 'tcp_port', label: $gettext('TCP Port'), type: 'text', placeholder: '9000' },
    ],
  },
  {
    name: 'performance',
    tab: $gettext('Performance'),
    tip: $gettext('ClickHouse performance settings.'),
    fields: [
      {
        key: 'max_memory_usage',
        label: $gettext('Max Memory Usage'),
        type: 'text',
        placeholder: '10000000000',
      },
      { key: 'max_threads', label: $gettext('Max Threads'), type: 'text', placeholder: '0' },
    ],
  },
  {
    name: 'paths',
    tab: $gettext('Paths'),
    fields: [
      {
        key: 'path',
        label: $gettext('Data Path'),
        type: 'text',
        placeholder: '/var/lib/clickhouse/',
      },
      {
        key: 'tmp_path',
        label: $gettext('Temp Path'),
        type: 'text',
        placeholder: '/var/lib/clickhouse/tmp/',
      },
    ],
  },
  {
    name: 'logging',
    tab: $gettext('Logging'),
    fields: [
      {
        key: 'log_level',
        label: $gettext('Log Level (logger.level)'),
        type: 'select',
        options: toOptions(['trace', 'debug', 'information', 'warning', 'error']),
      },
    ],
  },
]
</script>

<template>
  <config-tune-form
    :fetch="clickhouse.configTune"
    :save="clickhouse.saveConfigTune"
    :groups="groups"
  />
</template>
