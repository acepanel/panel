<script setup lang="ts">
defineOptions({
  name: 'pureftpd-config-tune',
})

import { useGettext } from 'vue3-gettext'

import pureftpd from '@/api/apps/pureftpd'
import { toOptions, type TuneGroup } from '@/utils/tune'

const { $gettext } = useGettext()

const yesNoOptions = toOptions(['yes', 'no'])

const groups: TuneGroup[] = [
  {
    name: 'general',
    tab: $gettext('General'),
    tip: $gettext('Common Pure-FTPd settings.'),
    fields: [
      {
        key: 'max_clients_number',
        label: $gettext('MaxClientsNumber'),
        type: 'number',
        placeholder: $gettext('e.g. 50'),
        min: 1,
      },
      {
        key: 'max_clients_per_ip',
        label: $gettext('MaxClientsPerIP'),
        type: 'number',
        placeholder: $gettext('e.g. 8'),
        min: 1,
      },
      {
        key: 'max_idle_time',
        label: $gettext('MaxIdleTime (minutes)'),
        type: 'number',
        placeholder: $gettext('e.g. 15'),
        min: 0,
      },
      {
        key: 'max_load',
        label: $gettext('MaxLoad'),
        type: 'number',
        placeholder: $gettext('e.g. 4'),
        min: 1,
      },
      {
        key: 'passive_port_range',
        label: $gettext('PassivePortRange (start end)'),
        type: 'text',
        placeholder: $gettext('e.g. 39000 40000'),
      },
      {
        key: 'anonymous_only',
        label: $gettext('AnonymousOnly'),
        type: 'select',
        options: yesNoOptions,
      },
      {
        key: 'no_anonymous',
        label: $gettext('NoAnonymous'),
        type: 'select',
        options: yesNoOptions,
      },
      {
        key: 'max_disk_usage',
        label: $gettext('MaxDiskUsage (%)'),
        type: 'number',
        placeholder: $gettext('e.g. 99'),
        min: 1,
        max: 100,
      },
    ],
  },
]
</script>

<template>
  <config-tune-form :fetch="pureftpd.configTune" :save="pureftpd.saveConfigTune" :groups="groups" />
</template>
