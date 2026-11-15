<script setup lang="ts">
defineOptions({
  name: 'memcached-config-tune',
})

import { useGettext } from 'vue3-gettext'

import memcached from '@/api/apps/memcached'
import type { TuneGroup } from '@/utils/tune'

const { $gettext } = useGettext()

const groups: TuneGroup[] = [
  {
    name: 'general',
    tab: $gettext('General'),
    tip: $gettext('Common Memcached settings.'),
    fields: [
      {
        key: 'port',
        label: $gettext('Port (-p)'),
        type: 'number',
        placeholder: $gettext('e.g. 11211'),
        min: 1,
        max: 65535,
      },
      {
        key: 'udp_port',
        label: $gettext('UDP Port (-U, 0 to disable)'),
        type: 'number',
        placeholder: $gettext('e.g. 0'),
        min: 0,
        max: 65535,
      },
      {
        key: 'listen_address',
        label: $gettext('Listen Address (-l)'),
        type: 'text',
        placeholder: $gettext('e.g. 127.0.0.1'),
      },
      {
        key: 'memory',
        label: $gettext('Memory (-m, MB)'),
        type: 'number',
        placeholder: $gettext('e.g. 64'),
        min: 1,
      },
      {
        key: 'max_connections',
        label: $gettext('Max Connections (-c)'),
        type: 'number',
        placeholder: $gettext('e.g. 1024'),
        min: 1,
      },
      {
        key: 'threads',
        label: $gettext('Threads (-t)'),
        type: 'number',
        placeholder: $gettext('e.g. 4'),
        min: 1,
      },
    ],
  },
]
</script>

<template>
  <config-tune-form
    :fetch="memcached.configTune"
    :save="memcached.saveConfigTune"
    :groups="groups"
  />
</template>
