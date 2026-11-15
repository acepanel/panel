<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

import home from '@/api/panel/home'
import project from '@/api/panel/project'
import setting from '@/api/panel/setting'
import { useFormLayout } from '@/components/system/composables/useFormLayout'
import { joinPath } from '@/utils'

import { buildCommand, runtimes } from './command'
import type { ProxyWebsiteForm } from './composables/useProxyWebsite'
import { useProxyWebsite } from './composables/useProxyWebsite'
import ProxyForm from './ProxyForm.vue'

const show = defineModel<boolean>('show', { type: Boolean, required: true })
const type = defineModel<string>('type', { type: String, required: true })

const { $gettext } = useGettext()
const formLayout = useFormLayout(100)
const { validateProxyWebsite, createProxyWebsite } = useProxyWebsite()

// Go 运行模式
const goModes = [
  { label: $gettext('Source Code'), value: 'source' },
  { label: $gettext('Binary'), value: 'binary' },
]

// 各语言的框架预设，命令接在运行时之后
const custom = { label: $gettext('Custom'), value: 'custom', command: '' }
const frameworks: Record<string, { label: string; value: string; command: string }[]> = {
  java: [
    custom,
    { label: 'Spring Boot (JAR)', value: 'spring-boot-jar', command: '-jar app.jar' },
    { label: 'Spring Boot (WAR)', value: 'spring-boot-war', command: '-jar app.war' },
    { label: 'Quarkus', value: 'quarkus', command: '-jar quarkus-run.jar' },
    { label: 'Micronaut', value: 'micronaut', command: '-jar app.jar' },
    { label: 'Vert.x', value: 'vertx', command: '-jar app.jar' },
    { label: 'Dropwizard', value: 'dropwizard', command: 'server config.yml' },
  ],
  nodejs: [
    custom,
    { label: 'Express', value: 'express', command: 'app.js' },
    { label: 'Koa', value: 'koa', command: 'app.js' },
    { label: 'Fastify', value: 'fastify', command: 'app.js' },
    { label: 'NestJS', value: 'nestjs', command: 'dist/main.js' },
    { label: 'Next.js', value: 'nextjs', command: 'node_modules/.bin/next start' },
    { label: 'Nuxt.js', value: 'nuxtjs', command: 'node_modules/.bin/nuxt start' },
    { label: 'Hapi', value: 'hapi', command: 'server.js' },
    { label: 'AdonisJS', value: 'adonisjs', command: 'server.js' },
  ],
  php: [
    custom,
    { label: 'Laravel Octane', value: 'laravel-octane', command: 'artisan octane:start' },
    { label: 'Laravel (Artisan Serve)', value: 'laravel-serve', command: 'artisan serve' },
    { label: 'ThinkPHP', value: 'thinkphp', command: 'think run' },
    { label: 'Webman', value: 'webman', command: 'start.php start' },
    { label: 'Hyperf', value: 'hyperf', command: 'bin/hyperf.php start' },
    { label: 'Swoole HTTP', value: 'swoole', command: 'server.php' },
    { label: 'RoadRunner', value: 'roadrunner', command: 'vendor/bin/rr serve' },
  ],
  python: [
    custom,
    { label: 'Django', value: 'django', command: 'manage.py runserver 0.0.0.0:8000' },
    { label: 'Flask', value: 'flask', command: '-m flask run --host=0.0.0.0' },
    {
      label: 'FastAPI (Uvicorn)',
      value: 'fastapi',
      command: '-m uvicorn main:app --host 0.0.0.0',
    },
    { label: 'Tornado', value: 'tornado', command: 'app.py' },
    { label: 'Sanic', value: 'sanic', command: '-m sanic server.app --host=0.0.0.0' },
    { label: 'aiohttp', value: 'aiohttp', command: 'app.py' },
    { label: 'Gunicorn', value: 'gunicorn', command: '-m gunicorn -w 4 app:app' },
  ],
  dotnet: [
    custom,
    { label: 'ASP.NET Core Web', value: 'aspnet-web', command: 'run' },
    { label: 'ASP.NET Core API', value: 'aspnet-api', command: 'run' },
    { label: 'Blazor Server', value: 'blazor-server', command: 'run' },
    { label: 'gRPC Service', value: 'grpc', command: 'run' },
    { label: 'Worker Service', value: 'worker', command: 'run' },
  ],
}

const defaultCreateModel = () => ({
  name: '',
  type: '',
  root_dir: '',
  exec_start: '',
  user: 'www',
})

const defaultRuntime = () => ({
  version: null as string | number | null,
  framework: 'custom',
  mode: 'source', // Go 运行方式
  entry: '', // 选中的入口文件或可执行文件
})

const createModel = ref(defaultCreateModel())
const runtime = ref(defaultRuntime())
const proxyEnabled = ref(false)
const proxyForm = ref<ProxyWebsiteForm>({ name: '', domains: [], port: null })
const loading = ref(false)

const { data: installedEnvironment } = useRequest(home.installedEnvironment, {
  initialData: {},
})
const { data: settings } = useRequest(setting.list, { initialData: {} })

const versionOptions = computed<{ label: string; value: string | number }[]>(
  () => installedEnvironment.value?.[type.value] || [],
)
const hasRuntime = computed(() => !!runtimes[type.value])
const isGoBinary = computed(() => type.value === 'go' && runtime.value.mode === 'binary')

// 与后端一致：留空时为默认目录/项目名，缺少前导 / 时补上
const defaultDir = computed(() => settings.value?.project_path || '/opt/ace/projects')
const projectDir = computed(() => {
  const dir =
    createModel.value.root_dir.trim() ||
    joinPath(defaultDir.value, createModel.value.name || 'project')
  return dir.startsWith('/') ? dir : '/' + dir
})

// 运行时命令名带版本号，Go 二进制和通用项目没有运行时
const runtimeName = computed(() => {
  if (!hasRuntime.value || isGoBinary.value) return ''
  return `${runtimes[type.value]}${runtime.value.version ?? ''}`
})

// 选了文件按文件生成，否则用框架预设；Go 源码默认运行当前目录的包，二进制默认项目目录下的 main
const generatedCommand = computed(() => {
  const { entry, framework } = runtime.value
  if (entry) return buildCommand(type.value, runtimeName.value, entry, projectDir.value)
  if (isGoBinary.value) return joinPath(projectDir.value, 'main')
  if (type.value === 'go') return `${runtimeName.value} run .`
  const preset = frameworks[type.value]?.find((f) => f.value === framework)
  return preset?.command ? `${runtimeName.value} ${preset.command}` : ''
})

// 只在生成结果变化时覆盖，手改过的命令不会因为改名、改目录被冲掉
watch(generatedCommand, (command) => {
  if (command) createModel.value.exec_start = command
})

// 换框架或 Go 运行方式后，之前选的文件不再适用
watch(
  () => runtime.value.framework,
  (framework) => {
    if (framework !== 'custom') runtime.value.entry = ''
  },
)
watch(
  () => runtime.value.mode,
  () => {
    runtime.value.entry = ''
  },
)

// 默认选中第一个已安装的版本，免得生成不带版本号的命令
const pickDefaultVersion = () => {
  if (runtime.value.version === null) runtime.value.version = versionOptions.value[0]?.value ?? null
}
watch(versionOptions, pickDefaultVersion)

// 弹窗复用同一实例，每次打开时重置表单，避免残留在其他语言标签下生成的启动命令等字段
watch(show, (value) => {
  if (!value) return
  createModel.value = defaultCreateModel()
  runtime.value = defaultRuntime()
  proxyEnabled.value = false
  proxyForm.value = { name: '', domains: [], port: null }
  pickDefaultVersion()
  createModel.value.exec_start = generatedCommand.value
})

// 目录选择器：项目目录或启动命令要用的文件，都从项目目录开始浏览
type PathTarget = 'root_dir' | 'entry'
const showPathSelector = ref(false)
const pathSelectorPath = ref('')
const pathSelectorTarget = ref<PathTarget>('root_dir')

const handleSelectPath = (target: PathTarget) => {
  pathSelectorTarget.value = target
  pathSelectorPath.value = createModel.value.root_dir.trim() ? projectDir.value : defaultDir.value
  showPathSelector.value = true
}

const handlePathSelected = (path: string) => {
  if (pathSelectorTarget.value === 'root_dir') {
    createModel.value.root_dir = path
    return
  }
  runtime.value.framework = 'custom'
  runtime.value.entry = path
}

const handleCreate = () => {
  createModel.value.type = type.value == 'all' ? 'general' : type.value
  proxyForm.value.name = createModel.value.name
  if (proxyEnabled.value && !validateProxyWebsite(proxyForm.value)) return

  loading.value = true
  useRequest(project.create(createModel.value))
    .onSuccess(async ({ data }) => {
      window.$bus.emit('project:refresh')
      window.$message.success($gettext('Project created successfully'))
      // 项目已建好，反代网站创建失败也关闭弹窗，可以到项目的反向代理里重试
      if (proxyEnabled.value) await createProxyWebsite(data, proxyForm.value)
      loading.value = false
      show.value = false
    })
    .onError(() => {
      loading.value = false
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
    python: $gettext('Create Python Project'),
    dotnet: $gettext('Create .NET Project'),
  }
  return titles[type.value] || $gettext('Create Project')
})

const versionLabels: Record<string, { label: string; placeholder: string }> = {
  go: { label: $gettext('Go Version'), placeholder: $gettext('Select Go Version') },
  java: { label: $gettext('Java Version'), placeholder: $gettext('Select Java Version') },
  nodejs: { label: $gettext('Node.js Version'), placeholder: $gettext('Select Node.js Version') },
  php: { label: $gettext('PHP Version'), placeholder: $gettext('Select PHP Version') },
  python: { label: $gettext('Python Version'), placeholder: $gettext('Select Python Version') },
  dotnet: { label: $gettext('.NET Version'), placeholder: $gettext('Select .NET Version') },
}
</script>

<template>
  <app-modal v-model:show="show" :title="modalTitle">
    <n-form :model="createModel" v-bind="formLayout">
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
            :placeholder="joinPath(defaultDir, createModel.name || '')"
          />
          <n-button @click="handleSelectPath('root_dir')">
            <template #icon>
              <i-mdi-folder-open />
            </template>
          </n-button>
        </n-input-group>
        <template #feedback>
          <span class="text-gray-400">
            {{ $gettext('Also used as the working directory of the project') }}
          </span>
        </template>
      </n-form-item>

      <n-form-item v-if="type === 'go'" :label="$gettext('Run Mode')">
        <n-radio-group v-model:value="runtime.mode">
          <n-radio-button
            v-for="mode in goModes"
            :key="mode.value"
            :value="mode.value"
            :label="mode.label"
          />
        </n-radio-group>
      </n-form-item>

      <n-grid v-if="hasRuntime && !isGoBinary" cols="1 m:24" :x-gap="24" responsive="screen">
        <n-form-item-gi :span="frameworks[type] ? 12 : 24" :label="versionLabels[type]?.label">
          <n-select
            v-model:value="runtime.version"
            :options="versionOptions"
            :placeholder="versionLabels[type]?.placeholder"
            @keydown.enter.prevent
          />
        </n-form-item-gi>
        <n-form-item-gi v-if="frameworks[type]" :span="12" :label="$gettext('Framework')">
          <n-select
            v-model:value="runtime.framework"
            :options="frameworks[type]"
            :placeholder="$gettext('Select Framework')"
            @keydown.enter.prevent
          />
        </n-form-item-gi>
      </n-grid>

      <n-form-item path="exec_start" :label="$gettext('Start Command')" required>
        <n-input-group>
          <n-input
            v-model:value="createModel.exec_start"
            type="text"
            @keydown.enter.prevent
            :placeholder="$gettext('e.g., php artisan serve, node app.js')"
          />
          <n-button @click="handleSelectPath('entry')">
            <template #icon>
              <i-mdi-file-outline />
            </template>
          </n-button>
        </n-input-group>
        <template #feedback>
          <span class="text-gray-400">
            {{
              hasRuntime && !isGoBinary
                ? $gettext('Select the entry file on the right to generate the command')
                : $gettext('Select the executable file on the right as the command')
            }}
          </span>
        </template>
      </n-form-item>

      <n-form-item path="user" :label="$gettext('Run User')">
        <n-select
          v-model:value="createModel.user"
          :options="[
            { label: 'www', value: 'www' },
            { label: 'root', value: 'root' },
            { label: 'nobody', value: 'nobody' },
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

      <n-form-item :label="$gettext('Reverse Proxy')">
        <n-switch v-model:value="proxyEnabled" />
        <template #feedback>
          <span class="text-gray-400">
            {{ $gettext('Automatically create a reverse proxy website for this project') }}
          </span>
        </template>
      </n-form-item>
      <proxy-form v-if="proxyEnabled" v-model:value="proxyForm" />
    </n-form>

    <n-button
      type="info"
      block
      class="mt-6"
      :loading="loading"
      :disabled="loading"
      @click="handleCreate"
    >
      {{ $gettext('Create') }}
    </n-button>
  </app-modal>

  <path-selector
    v-model:show="showPathSelector"
    v-model:path="pathSelectorPath"
    :dir="pathSelectorTarget === 'root_dir'"
    @select="handlePathSelected"
  />
</template>

<style scoped lang="scss"></style>
