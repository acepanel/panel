<script setup lang="ts">
defineOptions({
  name: 'mongodb-config-tune',
})

import { useGettext } from 'vue3-gettext'

import mongodb from '@/api/apps/mongodb'
import { toOptions, type TuneGroup } from '@/utils/tune'

const { $gettext } = useGettext()

const groups: TuneGroup[] = [
  {
    name: 'storage',
    tab: $gettext('Storage'),
    tip: $gettext('MongoDB storage engine and data path settings.'),
    fields: [
      {
        key: 'db_path',
        label: $gettext('Data Path (storage.dbPath)'),
        type: 'text',
        placeholder: '/data/db',
      },
      {
        key: 'cache_size_gb',
        label: $gettext('WiredTiger Cache Size GB (storage.wiredTiger.engineConfig.cacheSizeGB)'),
        type: 'text',
        placeholder: '0.5',
      },
    ],
  },
  {
    name: 'network',
    tab: $gettext('Network'),
    tip: $gettext('MongoDB network and security settings.'),
    fields: [
      { key: 'port', label: $gettext('Port (net.port)'), type: 'text', placeholder: '27017' },
      {
        key: 'bind_ip',
        label: $gettext('Bind IP (net.bindIp)'),
        type: 'text',
        placeholder: '127.0.0.1',
      },
      {
        key: 'authorization',
        label: $gettext('Authorization (security.authorization)'),
        type: 'select',
        options: toOptions(['enabled', 'disabled']),
      },
    ],
  },
  {
    name: 'logging',
    tab: $gettext('Logging'),
    fields: [
      {
        key: 'system_log_path',
        label: $gettext('System Log Path (systemLog.path)'),
        type: 'text',
        placeholder: '/var/log/mongodb/mongod.log',
      },
    ],
  },
]
</script>

<template>
  <config-tune-form :fetch="mongodb.configTune" :save="mongodb.saveConfigTune" :groups="groups" />
</template>
