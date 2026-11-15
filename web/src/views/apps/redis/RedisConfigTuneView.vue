<script setup lang="ts">
defineOptions({
  name: 'redis-config-tune',
})

import { useGettext } from 'vue3-gettext'

import { composeSize, toOptions, type TuneGroup, type TunePreset } from '@/utils/tune'
import { mbToPair, redisPreset } from '@/utils/tunepreset'

const props = defineProps<{
  api: { configTune: () => any; saveConfigTune: (data: any) => any }
  // Redis 或 Valkey
  name: string
}>()

const { $gettext } = useGettext()
const isValkey = props.name === 'Valkey'

const groups: TuneGroup[] = [
  {
    name: 'general',
    tab: $gettext('General'),
    tip: isValkey
      ? $gettext('Common Valkey general settings.')
      : $gettext('Common Redis general settings.'),
    fields: [
      {
        key: 'bind',
        label: $gettext('Bind (bind)'),
        type: 'text',
        placeholder: $gettext('e.g. 127.0.0.1'),
      },
      {
        key: 'port',
        label: $gettext('Port (port)'),
        type: 'number',
        placeholder: $gettext('e.g. 6379'),
        min: 1,
        max: 65535,
      },
      {
        key: 'databases',
        label: $gettext('Databases (databases)'),
        type: 'number',
        placeholder: $gettext('e.g. 16'),
        min: 1,
      },
      {
        key: 'requirepass',
        label: $gettext('Password (requirepass)'),
        type: 'text',
        placeholder: $gettext('Leave empty for no password'),
        password: true,
      },
      {
        key: 'timeout',
        label: $gettext('Timeout (timeout)'),
        type: 'number',
        placeholder: $gettext('e.g. 0 (disabled) or seconds'),
        min: 0,
      },
      {
        key: 'tcp_keepalive',
        label: $gettext('TCP Keepalive (tcp-keepalive)'),
        type: 'number',
        placeholder: $gettext('e.g. 300'),
        min: 0,
      },
    ],
  },
  {
    name: 'memory',
    tab: $gettext('Memory'),
    tip: isValkey
      ? $gettext('Valkey memory management settings.')
      : $gettext('Redis memory management settings.'),
    fields: [
      {
        key: 'maxmemory',
        label: $gettext('Max Memory (maxmemory)'),
        type: 'size',
        placeholder: $gettext('e.g. 256'),
        min: 0,
        units: ['kb', 'mb', 'gb'],
        defaultUnit: 'mb',
      },
      {
        key: 'maxmemory_policy',
        label: $gettext('Maxmemory Policy (maxmemory-policy)'),
        type: 'select',
        options: toOptions([
          'noeviction',
          'allkeys-lru',
          'allkeys-lfu',
          'allkeys-random',
          'volatile-lru',
          'volatile-lfu',
          'volatile-random',
          'volatile-ttl',
        ]),
      },
    ],
  },
  {
    name: 'persistence',
    tab: $gettext('Persistence'),
    tip: isValkey
      ? $gettext('Valkey AOF persistence settings.')
      : $gettext('Redis AOF persistence settings.'),
    fields: [
      {
        key: 'appendonly',
        label: $gettext('Append Only (appendonly)'),
        type: 'select',
        options: toOptions(['yes', 'no']),
      },
      {
        key: 'appendfsync',
        label: $gettext('Append Fsync (appendfsync)'),
        type: 'select',
        options: toOptions(['always', 'everysec', 'no']),
      },
    ],
  },
]

const preset: TunePreset = {
  fields: ['memory'],
  scenarioOptions: [
    { label: $gettext('Cache only'), value: 'cache' },
    { label: $gettext('Cache and storage mixed'), value: 'mixed' },
    { label: $gettext('Persistent storage'), value: 'persistent' },
  ],
  memoryRatio: 0.5,
  apply: (profile) => {
    const r = redisPreset(profile)
    const maxmemory = mbToPair(r.maxmemoryMB, ['mb', 'gb'])
    return {
      maxmemory: composeSize(maxmemory.num, maxmemory.unit),
      maxmemory_policy: r.maxmemoryPolicy,
      appendonly: r.appendonly,
      appendfsync: r.appendfsync,
    }
  },
}
</script>

<template>
  <config-tune-form
    :fetch="api.configTune"
    :save="api.saveConfigTune"
    :groups="groups"
    :preset="preset"
  />
</template>
