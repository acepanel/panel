<script setup lang="ts">
defineOptions({
  name: 'database-index',
})

import { NButton } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import pgadmin from '@/api/apps/pgadmin'
import phpmyadmin from '@/api/apps/phpmyadmin'
import app from '@/api/panel/app'
import database from '@/api/panel/database'
import { useBus } from '@/components/system/composables/useBus'
import { renderLocalIcon } from '@/utils'
import CreateDatabaseModal from '@/views/database/CreateDatabaseModal.vue'
import CreateUserModal from '@/views/database/CreateUserModal.vue'
import DatabaseList from '@/views/database/DatabaseList.vue'
import ElasticsearchDataView from '@/views/database/ElasticsearchDataView.vue'
import RedisDataView from '@/views/database/RedisDataView.vue'
import ServerList from '@/views/database/ServerList.vue'
import ServerModal from '@/views/database/ServerModal.vue'
import UserList from '@/views/database/UserList.vue'

const { $gettext } = useGettext()
const currentTab = ref('')
const keyword = ref('')

const createDatabaseModalShow = ref(false)
const createUserModalShow = ref(false)
const createServerModalShow = ref(false)

const phpMyAdminIcon = renderLocalIcon('app', 'phpmyadmin', { size: 16 })
const pgAdminIcon = renderLocalIcon('app', 'pgadmin', { size: 16 })

const phpMyAdminInstalled = ref(false)
const phpMyAdminLoading = ref(false)

const pgAdminInstalled = ref(false)

useRequest(app.isInstalled('phpmyadmin')).onSuccess(({ data }: any) => {
  phpMyAdminInstalled.value = data
})
useRequest(app.isInstalled('pgadmin')).onSuccess(({ data }: any) => {
  pgAdminInstalled.value = data
})

// 类型标签页仅展示已添加服务器的数据库类型
const typeTabs = [
  'mysql',
  'postgresql',
  'clickhouse',
  'mongodb',
  'sqlite',
  'elasticsearch',
  'redis',
]
// 带数据库列表的类型标签页
const databaseTabs = ['mysql', 'postgresql', 'clickhouse', 'mongodb', 'sqlite']
const servers = ref<any[]>([])

const availableTypes = computed(() => new Set(servers.value.map((item: any) => item.type)))
// n-tabs 对 v-if 动态增删子 tab 不会重算指示条位置，用 key 强制重挂
const tabsKey = computed(() => typeTabs.filter((t) => availableTypes.value.has(t)).join(','))
const mysqlServers = computed(() => servers.value.filter((item: any) => item.type === 'mysql'))
const postgresqlServers = computed(() =>
  servers.value.filter((item: any) => item.type === 'postgresql'),
)

const refreshServers = () => {
  useRequest(database.serverList(1, 10000)).onSuccess(({ data }: any) => {
    servers.value = data.items || []
    // 未初始化或停留在已消失的类型标签时定位到第一个可用类型
    const first = typeTabs.find((t) => availableTypes.value.has(t))
    if (
      !currentTab.value ||
      (typeTabs.includes(currentTab.value) && !availableTypes.value.has(currentTab.value))
    ) {
      currentTab.value = first ?? 'server'
    }
  })
}

onMounted(() => {
  refreshServers()
})

useBus('database-server:refresh', refreshServers)

const serverOptions = computed(() =>
  mysqlServers.value.map((item: any) => ({ label: item.name, key: item.id })),
)

const handlePhpMyAdmin = (serverID: number) => {
  phpMyAdminLoading.value = true
  useRequest(phpmyadmin.login(serverID))
    .onSuccess(({ data }: any) => {
      window.open(`http://${window.location.hostname}:${data.port}/${data.path}/`, '_blank')
    })
    .onComplete(() => {
      phpMyAdminLoading.value = false
    })
}

const handlePgAdmin = async () => {
  const data = await pgadmin.login()
  window.open(`http://${window.location.hostname}:${data.port}/`, '_blank')
}
</script>

<template>
  <PageContainer :show-footer="true">
    <template #tabs>
      <n-tabs :key="tabsKey" v-model:value="currentTab" animated>
        <n-tab v-if="availableTypes.has('mysql')" name="mysql" tab="MySQL" />
        <n-tab v-if="availableTypes.has('postgresql')" name="postgresql" tab="PostgreSQL" />
        <n-tab v-if="availableTypes.has('clickhouse')" name="clickhouse" tab="ClickHouse" />
        <n-tab v-if="availableTypes.has('mongodb')" name="mongodb" tab="MongoDB" />
        <n-tab v-if="availableTypes.has('sqlite')" name="sqlite" tab="SQLite" />
        <n-tab
          v-if="availableTypes.has('elasticsearch')"
          name="elasticsearch"
          tab="Elasticsearch"
        />
        <n-tab v-if="availableTypes.has('redis')" name="redis" tab="Redis" />
        <n-tab name="user" :tab="$gettext('User')" />
        <n-tab name="server" :tab="$gettext('Server')" />
      </n-tabs>
    </template>
    <n-flex vertical>
      <n-flex v-if="!['redis', 'elasticsearch'].includes(currentTab)" justify="space-between">
        <n-flex>
          <n-button
            v-if="['mysql', 'postgresql', 'clickhouse', 'mongodb'].includes(currentTab)"
            type="primary"
            @click="createDatabaseModalShow = true"
          >
            {{ $gettext('Create Database') }}
          </n-button>
          <template v-if="currentTab === 'mysql' && phpMyAdminInstalled && mysqlServers.length > 0">
            <n-dropdown
              v-if="mysqlServers.length > 1"
              :options="serverOptions"
              trigger="click"
              :disabled="phpMyAdminLoading"
              @select="handlePhpMyAdmin"
            >
              <n-button :loading="phpMyAdminLoading" :disabled="phpMyAdminLoading">
                <template #icon>
                  <component :is="phpMyAdminIcon" />
                </template>
                phpMyAdmin
              </n-button>
            </n-dropdown>
            <n-button
              v-else
              :loading="phpMyAdminLoading"
              :disabled="phpMyAdminLoading"
              @click="handlePhpMyAdmin(mysqlServers[0].id)"
            >
              <template #icon>
                <component :is="phpMyAdminIcon" />
              </template>
              phpMyAdmin
            </n-button>
          </template>
          <app-button
            v-if="currentTab === 'postgresql' && pgAdminInstalled && postgresqlServers.length > 0"
            @click="handlePgAdmin"
          >
            <template #icon>
              <component :is="pgAdminIcon" />
            </template>
            pgAdmin
          </app-button>
          <n-button v-if="currentTab === 'user'" type="primary" @click="createUserModalShow = true">
            {{ $gettext('Create User') }}
          </n-button>
          <n-button
            v-if="currentTab === 'server'"
            type="primary"
            @click="createServerModalShow = true"
          >
            {{ $gettext('Add Server') }}
          </n-button>
        </n-flex>
        <n-input
          v-if="databaseTabs.includes(currentTab)"
          v-model:value="keyword"
          :placeholder="$gettext('Search')"
          clearable
          class="md:!w-60"
        />
      </n-flex>
      <database-list
        v-if="databaseTabs.includes(currentTab)"
        :key="currentTab"
        :type="currentTab"
        :keyword="keyword"
      />
      <elasticsearch-data-view v-if="currentTab === 'elasticsearch'" type="elasticsearch" />
      <redis-data-view v-if="currentTab === 'redis'" />
      <user-list v-if="currentTab === 'user'" />
      <server-list v-if="currentTab === 'server'" />
    </n-flex>
  </PageContainer>
  <create-database-modal v-model:show="createDatabaseModalShow" :type="currentTab" />
  <create-user-modal v-model:show="createUserModalShow" />
  <server-modal v-model:show="createServerModalShow" />
</template>
