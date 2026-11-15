<script setup lang="ts">
defineOptions({
  name: 'elasticsearch-config-tune',
})

import { useGettext } from 'vue3-gettext'

import { toOptions, type TuneGroup } from '@/utils/tune'

const props = defineProps<{
  api: { configTune: () => any; saveConfigTune: (data: any) => any }
  // elasticsearch 或 opensearch
  service: string
}>()

const { $gettext } = useGettext()

const groups: TuneGroup[] = [
  {
    name: 'cluster',
    tab: $gettext('Cluster'),
    tip:
      props.service === 'opensearch'
        ? $gettext('OpenSearch cluster and network settings.')
        : $gettext('ElasticSearch cluster and network settings.'),
    fields: [
      {
        key: 'cluster_name',
        label: $gettext('Cluster Name (cluster.name)'),
        type: 'text',
        placeholder: props.service,
      },
      {
        key: 'node_name',
        label: $gettext('Node Name (node.name)'),
        type: 'text',
        placeholder: 'node-1',
      },
      {
        key: 'network_host',
        label: $gettext('Network Host (network.host)'),
        type: 'text',
        placeholder: '127.0.0.1',
      },
      {
        key: 'http_port',
        label: $gettext('HTTP Port (http.port)'),
        type: 'text',
        placeholder: '9200',
      },
      {
        key: 'discovery_type',
        label: $gettext('Discovery Type (discovery.type)'),
        type: 'select',
        options: toOptions(['single-node', 'multi-node']),
      },
    ],
  },
  {
    name: 'paths-jvm',
    tab: $gettext('Paths & JVM'),
    tip: $gettext(
      'Data paths and JVM heap memory settings. Heap size is recommended to be 50% of system RAM, max 31g.',
    ),
    fields: [
      {
        key: 'path_data',
        label: $gettext('Data Path (path.data)'),
        type: 'text',
        placeholder: `/var/lib/${props.service}`,
      },
      {
        key: 'path_logs',
        label: $gettext('Logs Path (path.logs)'),
        type: 'text',
        placeholder: `/var/log/${props.service}`,
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
  <config-tune-form :fetch="api.configTune" :save="api.saveConfigTune" :groups="groups" />
</template>
