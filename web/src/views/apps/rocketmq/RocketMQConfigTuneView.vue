<script setup lang="ts">
defineOptions({
  name: 'rocketmq-config-tune',
})

import { useGettext } from 'vue3-gettext'

import rocketmq from '@/api/apps/rocketmq'
import { toOptions, type TuneGroup } from '@/utils/tune'

const { $gettext } = useGettext()

const groups: TuneGroup[] = [
  {
    name: 'broker',
    tab: $gettext('Broker'),
    tip: $gettext('RocketMQ broker basic settings.'),
    fields: [
      {
        key: 'broker_name',
        label: $gettext('Broker Name (brokerName)'),
        type: 'text',
        placeholder: 'broker-a',
      },
      {
        key: 'listen_port',
        label: $gettext('Listen Port (listenPort)'),
        type: 'text',
        placeholder: '10911',
      },
      {
        key: 'namesrv_addr',
        label: $gettext('NameServer Address (namesrvAddr)'),
        type: 'text',
        placeholder: '127.0.0.1:9876',
      },
      {
        key: 'broker_role',
        label: $gettext('Broker Role (brokerRole)'),
        type: 'select',
        options: toOptions(['ASYNC_MASTER', 'SYNC_MASTER', 'SLAVE']),
      },
      {
        key: 'flush_disk_type',
        label: $gettext('Flush Disk Type (flushDiskType)'),
        type: 'select',
        options: toOptions(['ASYNC_FLUSH', 'SYNC_FLUSH']),
      },
    ],
  },
  {
    name: 'storage',
    tab: $gettext('Storage'),
    tip: $gettext('RocketMQ storage path and message size settings.'),
    fields: [
      {
        key: 'store_path_root_dir',
        label: $gettext('Store Root Dir (storePathRootDir)'),
        type: 'text',
        placeholder: '/data/rocketmq/store',
      },
      {
        key: 'store_path_commit_log',
        label: $gettext('CommitLog Path (storePathCommitLog)'),
        type: 'text',
        placeholder: '/data/rocketmq/store/commitlog',
      },
      {
        key: 'max_message_size',
        label: $gettext('Max Message Size (maxMessageSize)'),
        type: 'text',
        placeholder: '4194304',
      },
    ],
  },
  {
    name: 'jvm',
    tab: $gettext('JVM'),
    tip: $gettext('JVM heap memory settings for NameServer and Broker.'),
    fields: [
      {
        key: 'namesrv_heap_init_size',
        label: $gettext('Initial Heap Size (-Xms)'),
        type: 'text',
        placeholder: '512m',
        heading: 'NameServer',
      },
      {
        key: 'namesrv_heap_max_size',
        label: $gettext('Max Heap Size (-Xmx)'),
        type: 'text',
        placeholder: '512m',
      },
      {
        key: 'broker_heap_init_size',
        label: $gettext('Initial Heap Size (-Xms)'),
        type: 'text',
        placeholder: '1g',
        heading: 'Broker',
      },
      {
        key: 'broker_heap_max_size',
        label: $gettext('Max Heap Size (-Xmx)'),
        type: 'text',
        placeholder: '1g',
      },
    ],
  },
]
</script>

<template>
  <config-tune-form :fetch="rocketmq.configTune" :save="rocketmq.saveConfigTune" :groups="groups" />
</template>
