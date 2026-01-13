<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

import home from '@/api/panel/home'
import project from '@/api/panel/project'
import PathSelector from '@/components/common/PathSelector.vue'

const show = defineModel<boolean>('show', { type: Boolean, required: true })
const type = defineModel<string>('type', { type: String, required: true })

const { $gettext } = useGettext()

// PHP 框架预设
const phpFrameworks = [
  { label: $gettext('Custom'), value: 'custom', command: '' },
  { label: 'Laravel Octane', value: 'laravel-octane', command: 'artisan octane:start' },
  { label: 'Laravel (Artisan Serve)', value: 'laravel-serve', command: 'artisan serve' },
  { label: 'ThinkPHP', value: 'thinkphp', command: 'think run' },
  { label: 'Webman', value: 'webman', command: 'start.php start' },
  { label: 'Hyperf', value: 'hyperf', command: 'bin/hyperf.php start' },
  { label: 'Swoole HTTP', value: 'swoole', command: 'server.php' },
  { label: 'RoadRunner', value: 'roadrunner', command: 'vendor/bin/rr serve' }
]

const createModel = ref({
  name: '',
  type: '',
  root_dir: '',
  working_dir: '',
  exec_start: '',
  user: 'www'
})

// PHP 特有字段
const phpOptions = ref({
  version: null as number | null,
  framework: 'custom'
})

const showPathSelector = ref(false)
const pathSelectorPath = ref('/opt/ace/projects')

const { data: installedEnvironment } = useRequest(home.installedEnvironment, {
  initialData: {
    php: []
  }
})

// PHP 版本选项
const phpVersionOptions = computed(() => {
  return installedEnvironment.value?.php || []
})

// 根据 PHP 版本和框架生成启动命令
const generateCommand = () => {
  if (type.value !== 'php' || !phpOptions.value.version) {
    return
  }

  const framework = phpFrameworks.find((f) => f.value === phpOptions.value.framework)
  if (!framework || framework.value === 'custom') {
    return
  }

  const phpBin = `php${phpOptions.value.version}`
  createModel.value.exec_start = `${phpBin} ${framework.command}`
}

// 监听 PHP 版本和框架变化
watch(
  () => [phpOptions.value.version, phpOptions.value.framework],
  () => {
    generateCommand()
  }
)

// 处理目录选择
const handleSelectPath = () => {
  pathSelectorPath.value = createModel.value.root_dir || '/opt/ace/projects'
  showPathSelector.value = true
}

// 目录选择完成
watch(showPathSelector, (val) => {
  if (!val && pathSelectorPath.value) {
    createModel.value.root_dir = pathSelectorPath.value
  }
})

const handleCreate = async () => {
  createModel.value.type = type.value == 'all' ? 'general' : type.value

  useRequest(project.create(createModel.value)).onSuccess(() => {
    window.$bus.emit('project:refresh')
    window.$message.success($gettext('Project created successfully'))
    show.value = false
    // 重置表单
    createModel.value = {
      name: '',
      type: '',
      root_dir: '',
      working_dir: '',
      exec_start: '',
      user: 'www'
    }
    phpOptions.value = {
      version: null,
      framework: 'custom'
    }
  })
}

// 根据类型获取标题
const modalTitle = computed(() => {
  const titles: Record<string, string> = {
    general: $gettext('Create General Project'),
    php: $gettext('Create PHP Project')
  }
  return titles[type.value] || $gettext('Create Project')
})
</script>

<template>
  <n-modal
    v-model:show="show"
    :title="modalTitle"
    preset="card"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
    @close="show = false"
  >
    <n-form :model="createModel" label-placement="left" label-width="100">
      <n-form-item path="name" :label="$gettext('Project Name')">
        <n-input
          v-model:value="createModel.name"
          type="text"
          @keydown.enter.prevent
          :placeholder="$gettext('Project name, used as service identifier')"
        />
      </n-form-item>

      <n-form-item path="root_dir" :label="$gettext('Project Directory')">
        <n-input-group>
          <n-input
            v-model:value="createModel.root_dir"
            type="text"
            @keydown.enter.prevent
            :placeholder="
              $gettext(
                'Project root directory (if left empty, defaults to project directory/project name)'
              )
            "
          />
          <n-button @click="handleSelectPath">
            <template #icon>
              <i-mdi-folder-open />
            </template>
          </n-button>
        </n-input-group>
      </n-form-item>

      <!-- PHP 类型特有字段 -->
      <template v-if="type === 'php'">
        <n-row :gutter="[24, 0]">
          <n-col :span="12">
            <n-form-item :label="$gettext('PHP Version')">
              <n-select
                v-model:value="phpOptions.version"
                :options="phpVersionOptions"
                :placeholder="$gettext('Select PHP Version')"
                @keydown.enter.prevent
              />
            </n-form-item>
          </n-col>
          <n-col :span="12">
            <n-form-item :label="$gettext('Framework')">
              <n-select
                v-model:value="phpOptions.framework"
                :options="phpFrameworks"
                :placeholder="$gettext('Select Framework')"
                @keydown.enter.prevent
              />
            </n-form-item>
          </n-col>
        </n-row>
      </template>

      <n-form-item path="user" :label="$gettext('Run User')">
        <n-select
          v-model:value="createModel.user"
          :options="[
            { label: 'www', value: 'www' },
            { label: 'root', value: 'root' },
            { label: 'nobody', value: 'nobody' }
          ]"
          :placeholder="$gettext('Select User')"
          @keydown.enter.prevent
        />
        <template #feedback>
          <span class="text-gray-400">
            {{ $gettext('Select www user if no special requirements') }}
          </span>
        </template>
      </n-form-item>

      <n-form-item path="exec_start" :label="$gettext('Start Command')" required>
        <n-input
          v-model:value="createModel.exec_start"
          type="text"
          @keydown.enter.prevent
          :placeholder="$gettext('e.g., php artisan serve, node app.js')"
        />
      </n-form-item>
    </n-form>

    <n-button type="info" block @click="handleCreate">
      {{ $gettext('Create') }}
    </n-button>
  </n-modal>

  <!-- 目录选择器 -->
  <path-selector v-model:show="showPathSelector" v-model:path="pathSelectorPath" :dir="true" />
</template>

<style scoped lang="scss"></style>
