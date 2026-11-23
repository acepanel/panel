<script setup lang="ts">
import copy2clipboard from '@vavt/copy2clipboard'
import { NButton, NFlex, NInput, NInputGroup, NTag } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import database from '@/api/panel/database'
import AppButton from '@/components/system/AppButton.vue'
import { useBus } from '@/components/system/composables/useBus'
import { useConfirm } from '@/components/system/composables/useConfirm'
import { usePagedTable } from '@/components/system/composables/usePagedTable'
import { formatDateTime } from '@/utils'
import ServerModal from '@/views/database/ServerModal.vue'

const props = defineProps<{
  type?: string
}>()

const { $gettext } = useGettext()
const { confirmDelete, confirmAction } = useConfirm()
const updateModal = ref(false)
const updateID = ref(0)

// 终端弹窗
const terminalModal = ref(false)
const terminalTitle = ref('')
const terminalCommand = ref('')

// 打开数据库终端
const openTerminal = (row: any) => {
  if (row.type === 'mysql') {
    terminalTitle.value = `MySQL - ${row.name}`
    terminalCommand.value = `mysql -u'${row.username}' -p'${row.password}' -h'${row.host}' -P'${row.port}'`
  } else if (row.type === 'postgresql') {
    terminalTitle.value = `PostgreSQL - ${row.name}`
    if (row.password) {
      terminalCommand.value = `PGPASSWORD='${row.password}' psql -U '${row.username}' -h '${row.host}' -p '${row.port}'`
    } else {
      terminalCommand.value = `su - postgres -c 'psql'`
    }
  } else if (row.type === 'redis') {
    terminalTitle.value = `Redis - ${row.name}`
    if (row.password) {
      terminalCommand.value = `redis-cli -h '${row.host}' -p '${row.port}' -a '${row.password}'`
    } else {
      terminalCommand.value = `redis-cli -h '${row.host}' -p '${row.port}'`
    }
  } else if (row.type === 'clickhouse') {
    terminalTitle.value = `ClickHouse - ${row.name}`
    terminalCommand.value = `clickhouse-client --host '${row.host}' --port 9000 --user '${row.username}' --password '${row.password}'`
  } else if (row.type === 'mongodb') {
    terminalTitle.value = `MongoDB - ${row.name}`
    terminalCommand.value = `mongosh mongodb://${row.username}:${row.password}@${row.host}:${row.port}/admin`
  } else if (row.type === 'sqlite') {
    terminalTitle.value = `SQLite - ${row.name}`
    terminalCommand.value = `sqlite3 '${row.host}'`
  } else if (row.type === 'elasticsearch') {
    terminalTitle.value = `Elasticsearch - ${row.name}`
    terminalCommand.value = `curl -s 'http://${row.host}:${row.port}/_cat/indices?v'`
  } else {
    window.$message.error($gettext('Unsupported database type'))
    return
  }
  terminalModal.value = true
}

const columns: any = [
  {
    title: $gettext('Name'),
    key: 'name',
    minWidth: 100,
    resizable: true,
    ellipsis: { tooltip: true },
  },
  {
    title: $gettext('Username'),
    key: 'username',
    width: 150,
    ellipsis: { tooltip: true },
    render(row: any) {
      return row.username || $gettext('None')
    },
  },
  {
    title: $gettext('Password'),
    key: 'password',
    width: 250,
    render(row: any) {
      return h(NInputGroup, null, {
        default: () => [
          h(NInput, {
            value: row.password,
            type: 'password',
            showPasswordOn: 'click',
            readonly: true,
            placeholder: $gettext('None'),
          }),
          h(
            NButton,
            {
              type: 'primary',
              ghost: true,
              onClick: () => {
                copy2clipboard(row.password).then(() => {
                  window.$message.success($gettext('Copied successfully'))
                })
              },
            },
            { default: () => $gettext('Copy') },
          ),
        ],
      })
    },
  },
  {
    title: $gettext('Host'),
    key: 'host',
    width: 150,
    render(row: any) {
      return h(NTag, null, {
        default: () => `${row.host}:${row.port}`,
      })
    },
  },
  {
    title: $gettext('Comment'),
    key: 'remark',
    minWidth: 250,
    resizable: true,
    render(row: any) {
      return h(NInput, {
        size: 'small',
        class: 'w-full',
        value: row.remark,
        disabled: saving.has(row.id),
        loading: saving.has(row.id) || undefined,
        onBlur: () => handleRemark(row),
        onUpdateValue(v) {
          row.remark = v
        },
      })
    },
  },
  {
    title: $gettext('Status'),
    key: 'status',
    width: 100,
    render(row: any) {
      return h(
        NTag,
        { type: row.status === 'valid' ? 'success' : 'error' },
        { default: () => (row.status === 'valid' ? $gettext('Valid') : $gettext('Invalid')) },
      )
    },
  },
  {
    title: $gettext('Update Date'),
    key: 'updated_at',
    width: 200,
    ellipsis: { tooltip: true },
    render(row: any) {
      return formatDateTime(row.updated_at)
    },
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
    width: 350,
    hideInExcel: true,
    render(row: any) {
      const items = [
        h(
          NButton,
          {
            size: 'small',
            type: 'info',
            onClick: () => openTerminal(row),
          },
          { default: () => $gettext('Terminal') },
        ),
      ]
      if (!['redis', 'mongodb', 'sqlite', 'elasticsearch'].includes(row.type)) {
        items.push(
          h(
            AppButton,
            {
              size: 'small',
              type: 'success',
              onClick: async () => {
                const ok = await confirmAction({
                  type: 'warning',
                  title: $gettext('Confirm Sync'),
                  content: $gettext(
                    'Are you sure you want to synchronize database users (excluding password) to the panel?',
                  ),
                })
                if (!ok) return
                await database.serverSync(row.id)
                refresh()
                window.$message.success($gettext('Synchronized successfully'))
              },
            },
            { default: () => $gettext('Sync') },
          ),
        )
      }
      // 内置服务器的凭据由面板随应用设置同步，不提供修改
      if (!row.built_in) {
        items.push(
          h(
            NButton,
            {
              size: 'small',
              type: 'primary',
              onClick: () => {
                updateID.value = row.id
                updateModal.value = true
              },
            },
            { default: () => $gettext('Modify') },
          ),
        )
      }
      items.push(
        h(
          AppButton,
          {
            size: 'small',
            type: 'error',
            // 内置服务器直接提示，不返回 Promise，否则按钮会闪一下加载图标
            onClick: () => {
              if (row.built_in) {
                window.$message.error(
                  $gettext(
                    'Built-in servers cannot be deleted. If you need to delete them, please uninstall the corresponding app',
                  ),
                )
                return
              }
              return handleDelete(row.id)
            },
          },
          { default: () => $gettext('Delete') },
        ),
      )
      return h(NFlex, { size: 'small', align: 'center' }, () => items)
    },
  },
]

const { loading, data, page, pageSize, refresh, pagination } = usePagedTable((page, pageSize) =>
  database.serverList(page, pageSize, props.type),
)

const handleDelete = async (id: number) => {
  const ok = await confirmDelete({
    content: $gettext('Are you sure you want to delete the server?'),
    countdown: 5,
  })
  if (!ok) return
  await database.serverDelete(id)
  // 广播刷新,数据库页需要同步更新类型标签页
  window.$bus.emit('database-server:refresh')
  window.$message.success($gettext('Deleted successfully'))
}

const saving = reactive(new Set<number>())

const handleRemark = async (row: any) => {
  if (saving.has(row.id)) return
  saving.add(row.id)
  try {
    await database.serverRemark(row.id, row.remark)
    window.$message.success($gettext('Modified successfully'))
  } finally {
    saving.delete(row.id)
  }
}

useBus('database-server:refresh', refresh)
</script>

<template>
  <n-data-table
    v-model:page="page"
    v-model:pageSize="pageSize"
    striped
    remote
    :scroll-x="1700"
    :loading="loading"
    :columns="columns"
    :data="data"
    :row-key="(row: any) => row.name"
    :pagination="pagination"
  />
  <server-modal v-model:show="updateModal" :edit-id="updateID" />
  <!-- 终端弹窗 -->
  <pty-terminal-modal
    v-model:show="terminalModal"
    :title="terminalTitle"
    :command="terminalCommand"
  />
</template>

<style scoped lang="scss"></style>
