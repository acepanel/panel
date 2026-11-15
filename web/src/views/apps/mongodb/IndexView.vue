<script setup lang="ts">
defineOptions({
  name: 'apps-mongodb-index',
})

import mongodb from '@/api/apps/mongodb'

import MongoDBConfigTuneView from './MongoDBConfigTuneView.vue'

const currentTab = ref('status')
</script>

<template>
  <PageContainer :show-footer="true">
    <n-tabs v-model:value="currentTab" type="line" animated>
      <n-tab-pane name="status" :tab="$gettext('Running Status')">
        <n-flex vertical>
          <service-status service="mongod" />
          <password-card
            :title="$gettext('Admin Password')"
            :tip="
              $gettext('The MongoDB admin password is used to manage the database. Keep it safe!')
            "
            :fetch="mongodb.adminPassword"
            :save="mongodb.setAdminPassword"
          />
        </n-flex>
      </n-tab-pane>
      <n-tab-pane name="config" :tab="$gettext('Main Configuration')">
        <config-editor-pane
          :fetch="mongodb.config"
          :save="mongodb.saveConfig"
          :tip="
            $gettext(
              'This modifies the MongoDB configuration file. If you do not understand the meaning of each parameter, please do not modify it randomly!',
            )
          "
        />
      </n-tab-pane>
      <n-tab-pane name="config-tune" :tab="$gettext('Parameter Tuning')">
        <mongo-d-b-config-tune-view />
      </n-tab-pane>
      <n-tab-pane name="load" :tab="$gettext('Load Status')">
        <load-status-table :fetch="mongodb.load" />
      </n-tab-pane>
      <n-tab-pane name="run-log" :tab="$gettext('Runtime Logs')">
        <realtime-log service="mongod" />
      </n-tab-pane>
    </n-tabs>
  </PageContainer>
</template>
