<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

import javaApi from '@/api/panel/environment/java'

const route = useRoute()
const slug = route.params.slug as string

const { $gettext } = useGettext()

const handleSetCli = async () => {
  useRequest(javaApi.setCli(slug)).onSuccess(() => {
    window.$message.success($gettext('Set successfully'))
  })
}
</script>

<template>
  <PageContainer :show-footer="true">
    <n-flex vertical>
      <n-card>
        <template #header> Java {{ slug }} (Amazon Corretto) </template>
        <template #header-extra>
          <n-button type="info" @click="handleSetCli">
            {{ $gettext('Set as CLI Default Version') }}
          </n-button>
        </template>
        <n-alert type="info" :show-icon="false">
          {{
            $gettext(
              'Amazon Corretto is a no-cost, multiplatform, production-ready distribution of the Open Java Development Kit (OpenJDK).',
            )
          }}
        </n-alert>
      </n-card>
    </n-flex>
  </PageContainer>
</template>
