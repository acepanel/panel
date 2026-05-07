<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

import dotnetApi from '@/api/panel/environment/dotnet'

const route = useRoute()
const slug = route.params.slug as string

const { $gettext } = useGettext()

const handleSetCli = async () => {
  useRequest(dotnetApi.setCli(slug)).onSuccess(() => {
    window.$message.success($gettext('Set successfully'))
  })
}
</script>

<template>
  <PageContainer :show-footer="true">
    <n-flex vertical>
      <n-card>
        <template #header> .NET {{ slug }} </template>
        <template #header-extra>
          <n-button type="info" @click="handleSetCli">
            {{ $gettext('Set as CLI Default Version') }}
          </n-button>
        </template>
        <n-alert type="info" :show-icon="false">
          {{
            $gettext(
              '.NET is a free, open-source, cross-platform framework for building modern apps and powerful cloud services.',
            )
          }}
        </n-alert>
      </n-card>
    </n-flex>
  </PageContainer>
</template>
