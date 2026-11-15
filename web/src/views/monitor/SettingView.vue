<script setup lang="ts">
import { NButton, NFlex, NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import monitor from '@/api/panel/monitor'
import notify from '@/api/panel/notify'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { useFormLayout } from '@/components/system/composables/useFormLayout'
import { usePagedTable } from '@/components/system/composables/usePagedTable'
import ChannelModal from '@/views/monitor/ChannelModal.vue'
import { useNotifyEvents } from '@/views/monitor/metrics'

const { $gettext } = useGettext()
const { confirmAction, confirmDelete } = useConfirm()
const events = useNotifyEvents()
const formLayout = useFormLayout(160)

// 监控设置
const setting = ref({
  enabled: false,
  days: 30,
  interval: 1,
  alert_days: 30,
})
const settingLoading = ref(false)

const loadSetting = () => {
  useRequest(monitor.setting()).onSuccess(({ data }: any) => {
    setting.value = data
  })
}
loadSetting()

const handleSaveSetting = () => {
  settingLoading.value = true
  useRequest(monitor.updateSetting(setting.value))
    .onSuccess(() => {
      window.$message.success($gettext('Saved successfully'))
    })
    .onComplete(() => {
      settingLoading.value = false
    })
}

const handleClearMonitor = async () => {
  const ok = await confirmAction({
    title: $gettext('Clear Monitoring Records'),
    content: $gettext('Are you sure you want to clear?'),
  })
  if (!ok) return
  useRequest(monitor.clear()).onSuccess(() => {
    window.$message.success($gettext('Cleared successfully'))
  })
}

// 通知渠道
const channelModalShow = ref(false)
const editingChannel = ref<any>(null)
const testing = ref(0)

const {
  loading: channelsLoading,
  data: channels,
  page: channelPage,
  pageSize: channelPageSize,
  refresh: refreshChannels,
  pagination: channelPagination,
} = usePagedTable((page, pageSize) => notify.channels(page, pageSize), {
  pageSizes: [20, 50, 100],
})

const handleAddChannel = () => {
  editingChannel.value = null
  channelModalShow.value = true
}

const handleEditChannel = (row: any) => {
  editingChannel.value = row
  channelModalShow.value = true
}

const handleDeleteChannel = async (row: any) => {
  const ok = await confirmDelete({
    content: $gettext('Are you sure to delete this channel?'),
  })
  if (!ok) return
  useRequest(notify.deleteChannel(row.id)).onSuccess(() => {
    window.$message.success($gettext('Deleted successfully'))
    refreshChannels()
    loadNotifySetting()
  })
}

const handleTestChannel = (row: any) => {
  testing.value = row.id
  useRequest(notify.testChannel(row.id))
    .onSuccess(() => {
      window.$message.success($gettext('Test notification sent'))
    })
    .onComplete(() => {
      testing.value = 0
    })
}

const channelColumns: any = [
  { title: $gettext('Name'), key: 'name', width: 180, ellipsis: { tooltip: true } },
  {
    title: $gettext('Type'),
    key: 'type',
    width: 120,
    render: () => $gettext('SMTP Email'),
  },
  {
    title: $gettext('Recipients'),
    key: 'config',
    ellipsis: { tooltip: true },
    render: (row: any) => (row.config?.to || []).join(', '),
  },
  {
    title: $gettext('Status'),
    key: 'enabled',
    width: 90,
    render(row: any) {
      return h(NTag, { size: 'small', type: row.enabled ? 'success' : 'default' }, () =>
        row.enabled ? $gettext('Enabled') : $gettext('Disabled'),
      )
    },
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 200,
    align: 'center',
    render(row: any) {
      return h(NFlex, { justify: 'center', size: 8 }, () => [
        h(
          NButton,
          {
            size: 'small',
            secondary: true,
            loading: testing.value === row.id,
            onClick: () => handleTestChannel(row),
          },
          () => $gettext('Test'),
        ),
        h(NButton, { size: 'small', secondary: true, onClick: () => handleEditChannel(row) }, () =>
          $gettext('Edit'),
        ),
        h(
          NButton,
          {
            size: 'small',
            type: 'error',
            secondary: true,
            onClick: () => handleDeleteChannel(row),
          },
          () => $gettext('Delete'),
        ),
      ])
    },
  },
]

// 系统事件通知
const notifySetting = ref({
  events: [] as string[],
  channels: [] as number[],
})
const notifyLoading = ref(false)
const channelOptions = ref<{ label: string; value: number }[]>([])

const loadNotifySetting = () => {
  useRequest(notify.setting()).onSuccess(({ data }: any) => {
    notifySetting.value = { events: data.events || [], channels: data.channels || [] }
  })
  useRequest(notify.allChannels()).onSuccess(({ data }: any) => {
    channelOptions.value = (data || []).map((item: any) => ({ label: item.name, value: item.id }))
  })
}
loadNotifySetting()

const handleSaveNotifySetting = () => {
  notifyLoading.value = true
  useRequest(notify.updateSetting(notifySetting.value))
    .onSuccess(() => {
      window.$message.success($gettext('Saved successfully'))
    })
    .onComplete(() => {
      notifyLoading.value = false
    })
}

const handleChannelSaved = () => {
  refreshChannels()
  loadNotifySetting()
}
</script>

<template>
  <n-flex vertical :size="16">
    <n-card :title="$gettext('Monitoring')" size="small">
      <n-form v-bind="formLayout">
        <n-form-item :label="$gettext('Enable Monitoring')">
          <n-switch v-model:value="setting.enabled" />
        </n-form-item>
        <n-form-item :label="$gettext('Save Days')">
          <n-input-number v-model:value="setting.days" :min="1" :max="365" class="w-full md:!w-50">
            <template #suffix>{{ $gettext('days') }}</template>
          </n-input-number>
        </n-form-item>
        <n-form-item :label="$gettext('Collection Interval')">
          <n-input-number
            v-model:value="setting.interval"
            :min="1"
            :max="120"
            class="w-full md:!w-50"
          >
            <template #suffix>{{ $gettext('minutes') }}</template>
          </n-input-number>
        </n-form-item>
        <n-form-item :label="$gettext('Alert Record Retention')">
          <n-input-number
            v-model:value="setting.alert_days"
            :min="1"
            :max="365"
            class="w-full md:!w-50"
          >
            <template #suffix>{{ $gettext('days') }}</template>
          </n-input-number>
        </n-form-item>
        <n-form-item>
          <n-flex>
            <n-button
              type="primary"
              :loading="settingLoading"
              :disabled="settingLoading"
              @click="handleSaveSetting"
            >
              {{ $gettext('Save') }}
            </n-button>
            <n-button type="error" secondary @click="handleClearMonitor">
              {{ $gettext('Clear Monitoring Records') }}
            </n-button>
          </n-flex>
        </n-form-item>
      </n-form>
    </n-card>

    <n-card :title="$gettext('Notify Channels')" size="small">
      <template #header-extra>
        <n-button type="primary" size="small" @click="handleAddChannel">
          <template #icon>
            <i-mdi-plus />
          </template>
          {{ $gettext('Add Channel') }}
        </n-button>
      </template>
      <n-data-table
        v-model:page="channelPage"
        v-model:pageSize="channelPageSize"
        remote
        striped
        :scroll-x="800"
        :loading="channelsLoading"
        :columns="channelColumns"
        :data="channels"
        :row-key="(row: any) => row.id"
        :pagination="channelPagination"
      />
    </n-card>

    <n-card :title="$gettext('System Event Notifications')" size="small">
      <n-form v-bind="formLayout">
        <n-form-item :label="$gettext('Events')">
          <n-checkbox-group v-model:value="notifySetting.events">
            <n-grid :cols="2" :x-gap="16" :y-gap="8" item-responsive responsive="screen">
              <n-gi v-for="event in events" :key="event.value" span="2 m:1">
                <n-checkbox :value="event.value" :label="event.label" />
              </n-gi>
            </n-grid>
          </n-checkbox-group>
        </n-form-item>
        <n-form-item :label="$gettext('Notify Channels')">
          <n-select
            v-model:value="notifySetting.channels"
            multiple
            clearable
            :options="channelOptions"
            :placeholder="$gettext('Select channels to receive event notifications')"
            class="max-w-120"
          />
        </n-form-item>
        <n-form-item>
          <n-button
            type="primary"
            :loading="notifyLoading"
            :disabled="notifyLoading"
            @click="handleSaveNotifySetting"
          >
            {{ $gettext('Save') }}
          </n-button>
        </n-form-item>
      </n-form>
    </n-card>
  </n-flex>

  <channel-modal
    v-model:show="channelModalShow"
    :channel="editingChannel"
    @saved="handleChannelSaved"
  />
</template>
