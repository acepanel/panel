<script setup lang="ts">
defineOptions({
  name: 'nginx-config-tune',
})

import { useGettext } from 'vue3-gettext'

import { toOptions, type TuneField, type TuneGroup, type TunePreset } from '@/utils/tune'
import { nginxPreset } from '@/utils/tunepreset'

defineProps<{
  api: any
}>()

const { $gettext } = useGettext()

const onOffOptions = toOptions(['on', 'off'])
const onOffAlwaysOptions = toOptions(['on', 'off', 'always'])
// Nginx 的容量单位是小写
const size = {
  type: 'size',
  min: 0,
  units: ['k', 'm', 'g'],
  defaultUnit: 'k',
} satisfies Partial<TuneField>

const groups: TuneGroup[] = [
  {
    name: 'general',
    tab: $gettext('General'),
    tip: $gettext('Common Nginx general settings.'),
    fields: [
      {
        key: 'worker_processes',
        label: $gettext('Worker Processes (worker_processes)'),
        type: 'text',
        placeholder: $gettext('e.g. auto or number'),
      },
      {
        key: 'worker_connections',
        label: $gettext('Worker Connections (worker_connections)'),
        type: 'number',
        placeholder: $gettext('e.g. 65535'),
        min: 1,
      },
      {
        key: 'keepalive_timeout',
        label: $gettext('Keepalive Timeout (keepalive_timeout)'),
        type: 'number',
        placeholder: $gettext('e.g. 60'),
        min: 0,
      },
      {
        key: 'client_max_body_size',
        label: $gettext('Client Max Body Size (client_max_body_size)'),
        placeholder: $gettext('e.g. 200'),
        ...size,
      },
      {
        key: 'client_body_buffer_size',
        label: $gettext('Client Body Buffer Size (client_body_buffer_size)'),
        placeholder: $gettext('e.g. 10'),
        ...size,
      },
      {
        key: 'client_header_buffer_size',
        label: $gettext('Client Header Buffer Size (client_header_buffer_size)'),
        placeholder: $gettext('e.g. 32'),
        ...size,
      },
      {
        key: 'server_names_hash_bucket_size',
        label: $gettext('Server Names Hash Bucket Size (server_names_hash_bucket_size)'),
        type: 'number',
        placeholder: $gettext('e.g. 512'),
        min: 1,
      },
      {
        key: 'server_tokens',
        label: $gettext('Server Tokens (server_tokens)'),
        type: 'select',
        options: onOffOptions,
      },
      { key: 'reuseport', label: $gettext('TCP Port Reuse (reuseport)'), type: 'switch' },
    ],
  },
  {
    name: 'gzip',
    tab: 'Gzip',
    tip: $gettext(
      'Gzip compression settings. Gzip is the most widely supported compression method.',
    ),
    fields: [
      { key: 'gzip', label: $gettext('Gzip (gzip)'), type: 'select', options: onOffOptions },
      {
        key: 'gzip_min_length',
        label: $gettext('Min Length (gzip_min_length)'),
        placeholder: $gettext('e.g. 1'),
        ...size,
      },
      {
        key: 'gzip_comp_level',
        label: $gettext('Compression Level (gzip_comp_level)'),
        type: 'number',
        min: 1,
        max: 9,
      },
      {
        key: 'gzip_types',
        label: $gettext('Types (gzip_types)'),
        type: 'text',
        placeholder: $gettext('e.g. *'),
      },
      {
        key: 'gzip_vary',
        label: $gettext('Vary (gzip_vary)'),
        type: 'select',
        options: onOffOptions,
      },
      {
        key: 'gzip_proxied',
        label: $gettext('Proxied (gzip_proxied)'),
        type: 'text',
        placeholder: $gettext('e.g. any'),
      },
    ],
  },
  {
    name: 'brotli',
    tab: 'Brotli',
    tip: $gettext(
      'Brotli compression settings. Brotli provides better compression ratio than Gzip.',
    ),
    fields: [
      { key: 'brotli', label: $gettext('Brotli (brotli)'), type: 'select', options: onOffOptions },
      {
        key: 'brotli_min_length',
        label: $gettext('Min Length (brotli_min_length)'),
        placeholder: $gettext('e.g. 1'),
        ...size,
      },
      {
        key: 'brotli_comp_level',
        label: $gettext('Compression Level (brotli_comp_level)'),
        type: 'number',
        min: 0,
        max: 11,
      },
      {
        key: 'brotli_types',
        label: $gettext('Types (brotli_types)'),
        type: 'text',
        placeholder: $gettext('e.g. *'),
      },
      {
        key: 'brotli_static',
        label: $gettext('Static (brotli_static)'),
        type: 'select',
        options: onOffAlwaysOptions,
      },
    ],
  },
  {
    name: 'zstd',
    tab: 'Zstd',
    tip: $gettext('Zstd compression settings. Zstd provides fast compression with high ratio.'),
    fields: [
      { key: 'zstd', label: $gettext('Zstd (zstd)'), type: 'select', options: onOffOptions },
      {
        key: 'zstd_min_length',
        label: $gettext('Min Length (zstd_min_length)'),
        placeholder: $gettext('e.g. 1'),
        ...size,
      },
      {
        key: 'zstd_comp_level',
        label: $gettext('Compression Level (zstd_comp_level)'),
        type: 'number',
        min: 1,
        max: 22,
      },
      {
        key: 'zstd_types',
        label: $gettext('Types (zstd_types)'),
        type: 'text',
        placeholder: $gettext('e.g. *'),
      },
      {
        key: 'zstd_static',
        label: $gettext('Static (zstd_static)'),
        type: 'select',
        options: onOffAlwaysOptions,
      },
    ],
  },
]

const preset: TunePreset = {
  fields: ['memory', 'cpu'],
  memoryRatio: 1,
  apply: (profile) => {
    const r = nginxPreset(profile)
    return {
      worker_processes: r.workerProcesses,
      worker_connections: r.workerConnections,
      keepalive_timeout: r.keepaliveTimeout,
      gzip: r.gzip,
      gzip_comp_level: r.gzipCompLevel,
      brotli: r.brotli,
      brotli_comp_level: r.brotliCompLevel,
      brotli_static: r.brotliStatic,
      zstd: r.zstd,
      zstd_comp_level: r.zstdCompLevel,
      zstd_static: r.zstdStatic,
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
