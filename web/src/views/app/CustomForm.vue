<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

import app from '@/api/panel/app'

const { $gettext } = useGettext()

const props = defineProps({
  slug: {
    type: String,
    required: true,
  },
})

const model = ref({
  pre_script: '',
  args: '',
})

// 回显已持久化的参数，重装/更新无需重填
watch(
  () => props.slug,
  (slug) => {
    if (!slug) return
    useRequest(app.getCustom(slug)).onSuccess(({ data }: any) => {
      model.value = {
        pre_script: data.pre_script,
        args: data.args,
      }
    })
  },
  { immediate: true },
)

// 提交安装前由父组件调用保存
const save = () => app.saveCustom(props.slug, model.value)

defineExpose({ save })
</script>

<template>
  <n-flex vertical>
    <n-alert type="info">
      {{
        $gettext(
          'Settings are saved persistently and applied automatically on every install and update (recompile), no need to re-enter after reinstalling.',
        )
      }}
    </n-alert>
    <n-form :model="model">
      <n-form-item :label="$gettext('Pre-script')">
        <n-flex vertical class="w-full">
          <n-text depth="3">
            {{
              $gettext(
                'Runs before configure, can be used to download and prepare third-party module sources.',
              )
            }}
          </n-text>
          <common-editor v-model:value="model.pre_script" lang="shell" height="220px" />
        </n-flex>
      </n-form-item>
      <n-form-item :label="$gettext('Compile Params')">
        <n-flex vertical class="w-full">
          <n-text depth="3">
            {{
              $gettext(
                'One param per line, appended to the end of configure. Lines starting with # are ignored.',
              )
            }}
          </n-text>
          <n-input
            v-model:value="model.args"
            type="textarea"
            :rows="6"
            class="font-mono"
            placeholder="--with-xxx_module
--add-module=/path/to/module"
          />
        </n-flex>
      </n-form-item>
    </n-form>
  </n-flex>
</template>

<style scoped lang="scss"></style>
