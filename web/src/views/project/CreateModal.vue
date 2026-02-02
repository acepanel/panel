<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

import home from '@/api/panel/home'
import project from '@/api/panel/project'
import website from '@/api/panel/website'
import PathSelector from '@/components/common/PathSelector.vue'

const show = defineModel<boolean>('show', { type: Boolean, required: true })
const type = defineModel<string>('type', { type: String, required: true })

const { $gettext } = useGettext()

// Go 运行模式
const goModes = [
  { label: $gettext('Source Code'), value: 'source' },
  { label: $gettext('Binary'), value: 'binary' }
]

// Java 框架预设
const javaFrameworks = [
  { label: $gettext('Custom'), value: 'custom', command: '' },
  { label: 'Spring Boot (JAR)', value: 'spring-boot-jar', command: '-jar app.jar' },
  { label: 'Spring Boot (WAR)', value: 'spring-boot-war', command: '-jar app.war' },
  { label: 'Quarkus', value: 'quarkus', command: '-jar quarkus-run.jar' },
  { label: 'Micronaut', value: 'micronaut', command: '-jar app.jar' },
  { label: 'Vert.x', value: 'vertx', command: '-jar app.jar' },
  { label: 'Dropwizard', value: 'dropwizard', command: 'server config.yml' }
]

// Node.js 框架预设
const nodejsFrameworks = [
  { label: $gettext('Custom'), value: 'custom', command: '' },
  { label: 'Express', value: 'express', command: 'app.js' },
  { label: 'Koa', value: 'koa', command: 'app.js' },
  { label: 'Fastify', value: 'fastify', command: 'app.js' },
  { label: 'NestJS', value: 'nestjs', command: 'dist/main.js' },
  { label: 'Next.js', value: 'nextjs', command: 'node_modules/.bin/next start' },
  { label: 'Nuxt.js', value: 'nuxtjs', command: 'node_modules/.bin/nuxt start' },
  { label: 'Hapi', value: 'hapi', command: 'server.js' },
  { label: 'AdonisJS', value: 'adonisjs', command: 'server.js' }
]

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

// Python 框架预设
const pythonFrameworks = [
  { label: $gettext('Custom'), value: 'custom', command: '' },
  { label: 'Django', value: 'django', command: 'manage.py runserver 0.0.0.0:8000' },
  { label: 'Flask', value: 'flask', command: '-m flask run --host=0.0.0.0' },
  { label: 'FastAPI (Uvicorn)', value: 'fastapi', command: '-m uvicorn main:app --host 0.0.0.0' },
  { label: 'Tornado', value: 'tornado', command: 'app.py' },
  { label: 'Sanic', value: 'sanic', command: '-m sanic server.app --host=0.0.0.0' },
  { label: 'aiohttp', value: 'aiohttp', command: 'app.py' },
  { label: 'Gunicorn', value: 'gunicorn', command: '-m gunicorn -w 4 app:app' }
]

const createModel = ref({
  name: '',
  type: '',
  root_dir: '',
  working_dir: '',
  exec_start: '',
  user: 'www'
})

// 反向代理相关
const proxyOptions = ref({
  enabled: false,
  domains: [] as string[],
  port: null as number | null
})

// Go 特有字段
const goOptions = ref({
  mode: 'source' as string,
  version: '' as string,
  entryFile: 'main.go' as string
})

// Java 特有字段
const javaOptions = ref({
  version: '' as string,
  framework: 'custom'
})

// Node.js 特有字段
const nodejsOptions = ref({
  version: '' as string,
  framework: 'custom'
})

// PHP 特有字段
const phpOptions = ref({
  version: null as number | null,
  framework: 'custom'
})

// Python 特有字段
const pythonOptions = ref({
  version: '' as string,
  framework: 'custom'
})

const showPathSelector = ref(false)
const pathSelectorPath = ref('/opt/ace/projects')

const { data: installedEnvironment } = useRequest(home.installedEnvironment, {
  initialData: {
    go: [],
    java: [],
    nodejs: [],
    php: [],
    python: []
  }
})

// Go 版本选项
const goVersionOptions = computed(() => {
  return installedEnvironment.value?.go || []
})

// Java 版本选项
const javaVersionOptions = computed(() => {
  return installedEnvironment.value?.java || []
})

// Node.js 版本选项
const nodejsVersionOptions = computed(() => {
  return installedEnvironment.value?.nodejs || []
})

// PHP 版本选项
const phpVersionOptions = computed(() => {
  return installedEnvironment.value?.php || []
})

// Python 版本选项
const pythonVersionOptions = computed(() => {
  return installedEnvironment.value?.python || []
})

// 根据语言版本和框架生成启动命令
const generateCommand = () => {
  switch (type.value) {
    case 'go': {
      if (goOptions.value.mode === 'source') {
        // 源码模式
        if (!goOptions.value.version || !goOptions.value.entryFile) return
        const goBin = `go${goOptions.value.version}`
        createModel.value.exec_start = `${goBin} run ${goOptions.value.entryFile}`
      } else {
        // 二进制模式
        const rootDir =
          createModel.value.root_dir || `/opt/ace/projects/${createModel.value.name || 'project'}`
        createModel.value.exec_start = `${rootDir}/main`
      }
      break
    }
    case 'java': {
      if (!javaOptions.value.version) return
      const framework = javaFrameworks.find((f) => f.value === javaOptions.value.framework)
      if (!framework || framework.value === 'custom') return
      const javaBin = `java${javaOptions.value.version}`
      createModel.value.exec_start = `${javaBin} ${framework.command}`
      break
    }
    case 'nodejs': {
      if (!nodejsOptions.value.version) return
      const framework = nodejsFrameworks.find((f) => f.value === nodejsOptions.value.framework)
      if (!framework || framework.value === 'custom') return
      const nodeBin = `node${nodejsOptions.value.version}`
      createModel.value.exec_start = `${nodeBin} ${framework.command}`
      break
    }
    case 'php': {
      if (!phpOptions.value.version) return
      const framework = phpFrameworks.find((f) => f.value === phpOptions.value.framework)
      if (!framework || framework.value === 'custom') return
      const phpBin = `php${phpOptions.value.version}`
      createModel.value.exec_start = `${phpBin} ${framework.command}`
      break
    }
    case 'python': {
      if (!pythonOptions.value.version) return
      const framework = pythonFrameworks.find((f) => f.value === pythonOptions.value.framework)
      if (!framework || framework.value === 'custom') return
      const pythonBin = `python${pythonOptions.value.version}`
      createModel.value.exec_start = `${pythonBin} ${framework.command}`
      break
    }
  }
}

// 监听 Go 选项变化
watch(
  () => [
    goOptions.value.mode,
    goOptions.value.version,
    goOptions.value.entryFile,
    createModel.value.root_dir,
    createModel.value.name
  ],
  () => {
    if (type.value === 'go') generateCommand()
  }
)

// 监听 Java 版本和框架变化
watch(
  () => [javaOptions.value.version, javaOptions.value.framework],
  () => {
    if (type.value === 'java') generateCommand()
  }
)

// 监听 Node.js 版本和框架变化
watch(
  () => [nodejsOptions.value.version, nodejsOptions.value.framework],
  () => {
    if (type.value === 'nodejs') generateCommand()
  }
)

// 监听 PHP 版本和框架变化
watch(
  () => [phpOptions.value.version, phpOptions.value.framework],
  () => {
    if (type.value === 'php') generateCommand()
  }
)

// 监听 Python 版本和框架变化
watch(
  () => [pythonOptions.value.version, pythonOptions.value.framework],
  () => {
    if (type.value === 'python') generateCommand()
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

  // 如果启用了反向代理，先创建网站
  if (proxyOptions.value.enabled) {
    // 验证域名和端口
    const domains = proxyOptions.value.domains.filter((d) => d.trim() !== '')
    if (domains.length === 0) {
      window.$message.warning($gettext('Please enter at least one domain'))
      return
    }
    if (!proxyOptions.value.port) {
      window.$message.warning($gettext('Please enter the project port'))
      return
    }

    // 先创建反向代理网站
    const websiteData = {
      type: 'proxy',
      name: createModel.value.name,
      domains: domains,
      listens: ['80'],
      proxy: `http://127.0.0.1:${proxyOptions.value.port}`,
      remark: $gettext('Auto-created for project: %{ name }', { name: createModel.value.name })
    }

    try {
      await new Promise<void>((resolve, reject) => {
        useRequest(website.create(websiteData))
          .onSuccess(() => resolve())
          .onError((err) => reject(err))
      })
    } catch {
      // 网站创建失败，不继续创建项目
      return
    }
  }

  useRequest(project.create(createModel.value)).onSuccess(() => {
    window.$bus.emit('project:refresh')
    if (proxyOptions.value.enabled) {
      window.$bus.emit('website:refresh')
    }
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
    proxyOptions.value = {
      enabled: false,
      domains: [],
      port: null
    }
    goOptions.value = {
      mode: 'source',
      version: '',
      entryFile: 'main.go'
    }
    javaOptions.value = {
      version: '',
      framework: 'custom'
    }
    nodejsOptions.value = {
      version: '',
      framework: 'custom'
    }
    phpOptions.value = {
      version: null,
      framework: 'custom'
    }
    pythonOptions.value = {
      version: '',
      framework: 'custom'
    }
  })
}

// 根据类型获取标题
const modalTitle = computed(() => {
  const titles: Record<string, string> = {
    general: $gettext('Create General Project'),
    go: $gettext('Create Go Project'),
    java: $gettext('Create Java Project'),
    nodejs: $gettext('Create Node.js Project'),
    php: $gettext('Create PHP Project'),
    python: $gettext('Create Python Project')
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

      <!-- Go 类型特有字段 -->
      <template v-if="type === 'go'">
        <n-form-item :label="$gettext('Run Mode')">
          <n-radio-group v-model:value="goOptions.mode">
            <n-radio-button
              v-for="mode in goModes"
              :key="mode.value"
              :value="mode.value"
              :label="mode.label"
            />
          </n-radio-group>
        </n-form-item>

        <!-- 源码模式 -->
        <template v-if="goOptions.mode === 'source'">
          <n-row :gutter="[24, 0]">
            <n-col :span="12">
              <n-form-item :label="$gettext('Go Version')">
                <n-select
                  v-model:value="goOptions.version"
                  :options="goVersionOptions"
                  :placeholder="$gettext('Select Go Version')"
                  @keydown.enter.prevent
                />
              </n-form-item>
            </n-col>
            <n-col :span="12">
              <n-form-item :label="$gettext('Entry File')">
                <n-input
                  v-model:value="goOptions.entryFile"
                  type="text"
                  @keydown.enter.prevent
                  :placeholder="$gettext('e.g., main.go, cmd/server/main.go')"
                />
              </n-form-item>
            </n-col>
          </n-row>
        </template>
      </template>

      <!-- Java 类型特有字段 -->
      <template v-if="type === 'java'">
        <n-row :gutter="[24, 0]">
          <n-col :span="12">
            <n-form-item :label="$gettext('Java Version')">
              <n-select
                v-model:value="javaOptions.version"
                :options="javaVersionOptions"
                :placeholder="$gettext('Select Java Version')"
                @keydown.enter.prevent
              />
            </n-form-item>
          </n-col>
          <n-col :span="12">
            <n-form-item :label="$gettext('Framework')">
              <n-select
                v-model:value="javaOptions.framework"
                :options="javaFrameworks"
                :placeholder="$gettext('Select Framework')"
                @keydown.enter.prevent
              />
            </n-form-item>
          </n-col>
        </n-row>
      </template>

      <!-- Node.js 类型特有字段 -->
      <template v-if="type === 'nodejs'">
        <n-row :gutter="[24, 0]">
          <n-col :span="12">
            <n-form-item :label="$gettext('Node.js Version')">
              <n-select
                v-model:value="nodejsOptions.version"
                :options="nodejsVersionOptions"
                :placeholder="$gettext('Select Node.js Version')"
                @keydown.enter.prevent
              />
            </n-form-item>
          </n-col>
          <n-col :span="12">
            <n-form-item :label="$gettext('Framework')">
              <n-select
                v-model:value="nodejsOptions.framework"
                :options="nodejsFrameworks"
                :placeholder="$gettext('Select Framework')"
                @keydown.enter.prevent
              />
            </n-form-item>
          </n-col>
        </n-row>
      </template>

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

      <!-- Python 类型特有字段 -->
      <template v-if="type === 'python'">
        <n-row :gutter="[24, 0]">
          <n-col :span="12">
            <n-form-item :label="$gettext('Python Version')">
              <n-select
                v-model:value="pythonOptions.version"
                :options="pythonVersionOptions"
                :placeholder="$gettext('Select Python Version')"
                @keydown.enter.prevent
              />
            </n-form-item>
          </n-col>
          <n-col :span="12">
            <n-form-item :label="$gettext('Framework')">
              <n-select
                v-model:value="pythonOptions.framework"
                :options="pythonFrameworks"
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
          :placeholder="$gettext('Select or enter user')"
          filterable
          tag
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
      <n-form-item :label="$gettext('Reverse Proxy')">
        <n-switch v-model:value="proxyOptions.enabled" />
        <template #feedback>
          <span class="text-gray-400">
            {{ $gettext('Automatically create a reverse proxy website for this project') }}
          </span>
        </template>
      </n-form-item>

      <template v-if="proxyOptions.enabled">
        <n-row :gutter="[24, 0]">
          <n-col :span="16">
            <n-form-item :label="$gettext('Domain')">
              <n-dynamic-input
                v-model:value="proxyOptions.domains"
                placeholder="example.com"
                :min="1"
                show-sort-button
              />
            </n-form-item>
          </n-col>
          <n-col :span="8">
            <n-form-item :label="$gettext('Project Port')">
              <n-input-number
                v-model:value="proxyOptions.port"
                :min="1"
                :max="65535"
                style="width: 100%"
                :placeholder="$gettext('e.g., 3000')"
              />
            </n-form-item>
          </n-col>
        </n-row>
      </template>
    </n-form>

    <n-button type="info" block class="mt-24" @click="handleCreate">
      {{ $gettext('Create') }}
    </n-button>
  </n-modal>

  <!-- 目录选择器 -->
  <path-selector v-model:show="showPathSelector" v-model:path="pathSelectorPath" :dir="true" />
</template>

<style scoped lang="scss"></style>
