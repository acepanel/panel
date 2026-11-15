<script setup lang="ts">
defineOptions({
  name: 'apache-config-tune',
})

import { useGettext } from 'vue3-gettext'

import apache from '@/api/apps/apache'
import { toOptions, type TuneGroup, type TunePreset } from '@/utils/tune'
import { apachePreset } from '@/utils/tunepreset'

const { $gettext } = useGettext()

const groups: TuneGroup[] = [
  {
    name: 'mpm',
    tab: $gettext('MPM Event'),
    tip: $gettext('Worker thread pool settings for the event MPM.'),
    fields: [
      {
        key: 'start_servers',
        label: $gettext('Start Servers (StartServers)'),
        type: 'number',
        placeholder: $gettext('e.g. 3'),
        min: 1,
      },
      {
        key: 'min_spare_threads',
        label: $gettext('Min Spare Threads (MinSpareThreads)'),
        type: 'number',
        placeholder: $gettext('e.g. 75'),
        min: 1,
      },
      {
        key: 'max_spare_threads',
        label: $gettext('Max Spare Threads (MaxSpareThreads)'),
        type: 'number',
        placeholder: $gettext('e.g. 250'),
        min: 1,
      },
      {
        key: 'threads_per_child',
        label: $gettext('Threads Per Child (ThreadsPerChild)'),
        type: 'number',
        placeholder: $gettext('e.g. 25'),
        min: 1,
      },
      {
        key: 'max_request_workers',
        label: $gettext('Max Request Workers (MaxRequestWorkers)'),
        type: 'number',
        placeholder: $gettext('e.g. 400'),
        min: 1,
      },
      {
        key: 'max_connections_per_child',
        label: $gettext('Max Connections Per Child (MaxConnectionsPerChild)'),
        type: 'number',
        placeholder: $gettext('0 means unlimited'),
        min: 0,
      },
    ],
  },
  {
    name: 'connection',
    tab: $gettext('Connection'),
    tip: $gettext('Connection and keep-alive settings.'),
    fields: [
      {
        key: 'timeout',
        label: $gettext('Timeout (Timeout)'),
        type: 'number',
        placeholder: $gettext('e.g. 60'),
        min: 1,
      },
      {
        key: 'keep_alive',
        label: $gettext('Keep Alive (KeepAlive)'),
        type: 'select',
        options: toOptions(['On', 'Off']),
      },
      {
        key: 'max_keep_alive_requests',
        label: $gettext('Max Keep Alive Requests (MaxKeepAliveRequests)'),
        type: 'number',
        placeholder: $gettext('0 means unlimited'),
        min: 0,
      },
      {
        key: 'keep_alive_timeout',
        label: $gettext('Keep Alive Timeout (KeepAliveTimeout)'),
        type: 'number',
        placeholder: $gettext('e.g. 5'),
        min: 1,
      },
    ],
  },
]

const preset: TunePreset = {
  fields: ['memory', 'cpu'],
  memoryRatio: 0.5,
  apply: (profile) => {
    const r = apachePreset(profile)
    return {
      start_servers: r.startServers,
      threads_per_child: r.threadsPerChild,
      max_request_workers: r.maxRequestWorkers,
      min_spare_threads: r.minSpareThreads,
      max_spare_threads: r.maxSpareThreads,
      max_connections_per_child: r.maxConnectionsPerChild,
      timeout: r.timeout,
      keep_alive: r.keepAlive,
      max_keep_alive_requests: r.maxKeepAliveRequests,
      keep_alive_timeout: r.keepAliveTimeout,
    }
  },
}
</script>

<template>
  <config-tune-form
    :fetch="apache.configTune"
    :save="apache.saveConfigTune"
    :groups="groups"
    :preset="preset"
  />
</template>
