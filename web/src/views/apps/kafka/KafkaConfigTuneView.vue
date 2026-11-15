<script setup lang="ts">
defineOptions({
  name: 'kafka-config-tune',
})

import { useGettext } from 'vue3-gettext'

import kafka from '@/api/apps/kafka'
import type { TuneGroup } from '@/utils/tune'

const { $gettext } = useGettext()

const groups: TuneGroup[] = [
  {
    name: 'broker',
    tab: $gettext('Broker'),
    tip: $gettext('Kafka broker and network settings.'),
    fields: [
      { key: 'node_id', label: $gettext('Node ID (node.id)'), type: 'text', placeholder: '1' },
      {
        key: 'listeners',
        label: $gettext('Listeners (listeners)'),
        type: 'text',
        placeholder: 'PLAINTEXT://:9092',
      },
      {
        key: 'log_dirs',
        label: $gettext('Log Dirs (log.dirs)'),
        type: 'text',
        placeholder: '/data/kafka-logs',
      },
      {
        key: 'num_partitions',
        label: $gettext('Num Partitions (num.partitions)'),
        type: 'text',
        placeholder: '1',
      },
    ],
  },
  {
    name: 'storage-jvm',
    tab: $gettext('Storage & JVM'),
    tip: $gettext('Log retention and JVM heap memory settings.'),
    fields: [
      {
        key: 'retention_hours',
        label: $gettext('Log Retention Hours (log.retention.hours)'),
        type: 'text',
        placeholder: '168',
      },
      {
        key: 'log_segment_bytes',
        label: $gettext('Log Segment Bytes (log.segment.bytes)'),
        type: 'text',
        placeholder: '1073741824',
      },
      {
        key: 'heap_init_size',
        label: $gettext('Initial Heap Size (-Xms)'),
        type: 'text',
        placeholder: '1g',
      },
      {
        key: 'heap_max_size',
        label: $gettext('Max Heap Size (-Xmx)'),
        type: 'text',
        placeholder: '1g',
      },
    ],
  },
]
</script>

<template>
  <config-tune-form :fetch="kafka.configTune" :save="kafka.saveConfigTune" :groups="groups" />
</template>
