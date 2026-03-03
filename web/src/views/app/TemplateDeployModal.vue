<script setup lang="ts">
import containerApi from '@/api/panel/container'
import templateApi from '@/api/panel/template'
import DiffEditor from '@/components/common/DiffEditor.vue'
import PtyTerminalModal from '@/components/common/PtyTerminalModal.vue'
import type { FormInst, FormItemRule, FormRules } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import type { Template, TemplateEnvironment } from './types'

const { $gettext } = useGettext()

const props = defineProps<{
  template: Template | null
}>()

const emit = defineEmits<{
  success: []
}>()

const show = defineModel<boolean>('show', { type: Boolean, required: true })

const doSubmit = ref(false)
const currentStep = ref(1)
const formRef = ref<FormInst | null>(null)

// 部署模式: create 或 update
const deployMode = ref<'create' | 'update'>('create')

// 编排列表
const composeList = ref<{ name: string; path: string }[]>([])
const composeListLoading = ref(false)
const selectedCompose = ref<string | null>(null)
const selectedComposeData = ref<{ compose: string; envs: { key: string; value: string }[] } | null>(
  null
)

// 启动终端
const upModal = ref(false)
const upCommand = ref('')

const deployModel = reactive({
  name: '',
  autoStart: true,
  autoFirewall: false,
  envs: {} as Record<string, any>
})

// 最终编排内容
const finalCompose = ref('')
const finalEnvs = ref<{ key: string; value: string }[]>([])

// 初始化环境变量默认值
const initEnvDefaults = () => {
  if (!props.template?.environments) return
  const envs: Record<string, string> = {}
  props.template.environments.forEach((env: TemplateEnvironment) => {
    envs[env.name] = env.default || ''
  })
  deployModel.envs = envs
}

// 获取 select 选项
const getSelectOptions = (env: TemplateEnvironment) => {
  if (!env.options) return []
  return Object.entries(env.options).map(([label, value]) => ({
    label,
    value
  }))
}

// 动态生成表单校验规则
const formRules = computed<FormRules>(() => {
  const rules: FormRules = {}
  props.template?.environments?.forEach((env) => {
    const isRequired = env.default == null

    if (env.type === 'url') {
      rules[`envs.${env.name}`] = {
        trigger: ['input', 'blur'],
        validator(_rule: FormItemRule, value: string) {
          if (!value && env.default) return true
          if (!value) return new Error($gettext('Please enter URL'))
          try {
            new URL(value)
            return true
          } catch {
            return new Error($gettext('Please enter a valid URL'))
          }
        }
      }
    } else if (isRequired) {
      // 为必填字段添加校验规则
      rules[`envs.${env.name}`] = {
        required: true,
        trigger: ['input', 'blur'],
        validator(_rule: FormItemRule, value: any) {
          if (value === null || value === undefined || value === '') {
            return new Error($gettext('This field is required'))
          }
          return true
        }
      }
    }
  })
  return rules
})

// 加载编排列表
const loadComposeList = () => {
  composeListLoading.value = true
  useRequest(containerApi.composeList(1, 1000))
    .onSuccess(({ data }) => {
      composeList.value = data.items || []
    })
    .onComplete(() => {
      composeListLoading.value = false
    })
}

// 加载选中编排的详情并预填充环境变量
const loadComposeDetailAndFillEnvs = (name: string) => {
  useRequest(containerApi.composeGet(name)).onSuccess(({ data }) => {
    selectedComposeData.value = {
      compose: data.compose,
      envs: data.envs || []
    }
    // 用旧编排的环境变量预填充表单
    const oldEnvs = data.envs || []
    oldEnvs.forEach((env: { key: string; value: string }) => {
      if (env.key in deployModel.envs) {
        // 根据模板中定义的类型进行转换
        const envDef = props.template?.environments?.find((e) => e.name === env.key)
        if (envDef && (envDef.type === 'number' || envDef.type === 'port')) {
          // 数字类型需要转换为 number
          const numValue = Number(env.value)
          deployModel.envs[env.key] = isNaN(numValue) ? null : numValue
        } else {
          deployModel.envs[env.key] = env.value
        }
      }
    })
  })
}

// 进入步骤3
const goToStep3 = () => {
  finalCompose.value = props.template?.compose || ''
  finalEnvs.value = generateFinalEnvs()
  currentStep.value = 3
}

// 生成最终的环境变量列表
const generateFinalEnvs = () => {
  return Object.entries(deployModel.envs).map(([key, value]) => ({
    key,
    value: String(value)
  }))
}

// 步骤1：选择部署模式
const handleModeSelect = (mode: 'create' | 'update') => {
  deployMode.value = mode
  if (mode === 'update') {
    loadComposeList()
  }
  currentStep.value = 2
}

// 选择编排后加载详情并预填充环境变量
const handleComposeSelect = (name: string) => {
  selectedCompose.value = name
  if (name) {
    loadComposeDetailAndFillEnvs(name)
  }
}

// 步骤2：验证并进入下一步
const handleStep2Next = async () => {
  if (deployMode.value === 'create') {
    // 验证编排名称
    if (!deployModel.name.trim()) {
      window.$message.warning($gettext('Please enter compose name'))
      return
    }

    // 表单校验
    try {
      await formRef.value?.validate()
    } catch {
      return
    }

    goToStep3()
  } else {
    // 更新模式：验证是否选择了编排
    if (!selectedCompose.value) {
      window.$message.warning($gettext('Please select a compose'))
      return
    }

    // 表单校验
    try {
      await formRef.value?.validate()
    } catch {
      return
    }

    goToStep3()
  }
}

// 步骤3：进入确认步骤
const handleStep3Next = () => {
  currentStep.value = 4
}

// 提交部署
const handleSubmit = async () => {
  if (!props.template) return

  doSubmit.value = true

  if (deployMode.value === 'create') {
    // 创建新编排
    useRequest(
      templateApi.create({
        slug: props.template.slug,
        name: deployModel.name,
        compose: finalCompose.value,
        envs: finalEnvs.value,
        auto_firewall: deployModel.autoFirewall
      })
    )
      .onSuccess(({ data }) => {
        window.$message.success($gettext('Created successfully'))
        if (deployModel.autoStart) {
          upCommand.value = `docker compose -f ${data}/docker-compose.yml up -d`
          upModal.value = true
        } else {
          show.value = false
          emit('success')
        }
      })
      .onComplete(() => {
        doSubmit.value = false
      })
  } else {
    // 更新已有编排
    useRequest(
      containerApi.composeUpdate(selectedCompose.value!, {
        compose: finalCompose.value,
        envs: finalEnvs.value
      })
    )
      .onSuccess(() => {
        window.$message.success($gettext('Update successful'))
        const composePath = composeList.value.find((c) => c.name === selectedCompose.value)?.path
        if (deployModel.autoStart && composePath) {
          upCommand.value = `docker compose -f ${composePath}/docker-compose.yml up -d`
          upModal.value = true
        } else {
          show.value = false
          emit('success')
        }
      })
      .onComplete(() => {
        doSubmit.value = false
      })
  }
}

// 启动完成
const handleUpComplete = () => {
  show.value = false
  emit('success')
}

// 返回上一步
const handlePrev = () => {
  if (currentStep.value > 1) {
    currentStep.value--
  }
}

const resetForm = () => {
  deployModel.name = props.template?.slug || ''
  deployModel.autoStart = true
  deployModel.autoFirewall = false
  deployModel.envs = {}
  currentStep.value = 1
  deployMode.value = 'create'
  selectedCompose.value = null
  selectedComposeData.value = null
  finalCompose.value = ''
  finalEnvs.value = []
  initEnvDefaults()
}

watch(show, (val) => {
  if (val) {
    resetForm()
  }
})

watch(
  () => props.template,
  () => {
    if (props.template) {
      initEnvDefaults()
    }
  }
)

// 编排选项
const composeOptions = computed(() => {
  return composeList.value.map((item) => ({
    label: item.name,
    value: item.name
  }))
})
</script>

<template>
  <n-modal
    v-model:show="show"
    :title="$gettext('Deploy Template') + (template ? ` - ${template.name}` : '')"
    preset="card"
    style="width: 70vw"
    size="huge"
    :bordered="false"
    :segmented="false"
    :mask-closable="!doSubmit"
    :closable="!doSubmit"
  >
    <!-- 步骤指示器 -->
    <n-steps :current="currentStep" size="small" class="mb-24">
      <n-step :title="$gettext('Deploy Mode')" />
      <n-step :title="$gettext('Configuration')" />
      <n-step :title="$gettext('Preview & Edit')" />
      <n-step :title="$gettext('Confirm')" />
    </n-steps>

    <!-- 步骤1：选择部署模式 -->
    <div v-if="currentStep === 1">
      <n-flex justify="center" :size="24" style="padding: 40px 0">
        <n-card hoverable style="width: 280px; cursor: pointer" @click="handleModeSelect('create')">
          <n-flex vertical align="center" :size="16">
            <n-icon size="48" color="#18a058">
              <svg viewBox="0 0 24 24" fill="currentColor">
                <path d="M19 13h-6v6h-2v-6H5v-2h6V5h2v6h6v2z" />
              </svg>
            </n-icon>
            <n-text strong style="font-size: 16px">{{ $gettext('Create New Compose') }}</n-text>
            <n-text depth="3" style="text-align: center">
              {{ $gettext('Create a new compose from this template') }}
            </n-text>
          </n-flex>
        </n-card>

        <n-card hoverable style="width: 280px; cursor: pointer" @click="handleModeSelect('update')">
          <n-flex vertical align="center" :size="16">
            <n-icon size="48" color="#2080f0">
              <svg viewBox="0 0 24 24" fill="currentColor">
                <path
                  d="M21 10.12h-6.78l2.74-2.82c-2.73-2.7-7.15-2.8-9.88-.1-2.73 2.71-2.73 7.08 0 9.79s7.15 2.71 9.88 0C18.32 15.65 19 14.08 19 12.1h2c0 1.98-.88 4.55-2.64 6.29-3.51 3.48-9.21 3.48-12.72 0-3.5-3.47-3.53-9.11-.02-12.58s9.14-3.47 12.65 0L21 3v7.12z"
                />
              </svg>
            </n-icon>
            <n-text strong style="font-size: 16px">{{
              $gettext('Update Existing Compose')
            }}</n-text>
            <n-text depth="3" style="text-align: center">
              {{ $gettext('Update an existing compose with this template') }}
            </n-text>
          </n-flex>
        </n-card>
      </n-flex>
    </div>

    <!-- 步骤2：配置 -->
    <div v-else-if="currentStep === 2">
      <!-- 创建模式 -->
      <template v-if="deployMode === 'create'">
        <n-form
          ref="formRef"
          :model="deployModel"
          :rules="formRules"
          label-placement="left"
          label-width="160"
        >
          <n-form-item path="name" :label="$gettext('Compose Name')" required>
            <n-input
              v-model:value="deployModel.name"
              type="text"
              @keydown.enter.prevent
              :placeholder="$gettext('Enter compose name')"
            />
          </n-form-item>

          <n-divider title-placement="left">{{ $gettext('Deploy Options') }}</n-divider>

          <n-row :gutter="[24, 0]">
            <n-col :span="8">
              <n-form-item path="autoStart" :label="$gettext('Auto Start')">
                <n-switch v-model:value="deployModel.autoStart" />
              </n-form-item>
            </n-col>
            <n-col :span="8">
              <n-form-item path="autoFirewall" :label="$gettext('Auto Firewall')">
                <n-switch v-model:value="deployModel.autoFirewall" />
                <template #feedback>
                  <span>{{ $gettext('Automatically allow ports defined in compose') }}</span>
                </template>
              </n-form-item>
            </n-col>
          </n-row>

          <!-- 环境变量 -->
          <template v-if="template?.environments?.length">
            <n-divider title-placement="left">{{ $gettext('Environment Variables') }}</n-divider>

            <n-form-item
              v-for="env in template.environments"
              :key="env.name"
              :path="`envs.${env.name}`"
              :label="env.description"
              :required="env.default == null"
            >
              <!-- Select 类型 -->
              <n-select
                v-if="env.type === 'select'"
                v-model:value="deployModel.envs[env.name]"
                :options="getSelectOptions(env)"
                :placeholder="$gettext('Select value')"
              />
              <!-- Number/Port 类型 -->
              <n-input-number
                v-else-if="env.type === 'number' || env.type === 'port'"
                v-model:value="deployModel.envs[env.name]"
                :min="env.type === 'port' ? 1 : undefined"
                :max="env.type === 'port' ? 65535 : undefined"
                style="width: 100%"
                :placeholder="env.default != null ? env.default : ''"
              />
              <!-- Password 类型 -->
              <n-input
                v-else-if="env.type === 'password'"
                v-model:value="deployModel.envs[env.name]"
                type="password"
                show-password-on="click"
                :placeholder="env.default != null ? env.default : ''"
              />
              <!-- URL 类型 -->
              <n-input
                v-else-if="env.type === 'url'"
                v-model:value="deployModel.envs[env.name]"
                :placeholder="env.default != null ? env.default : ''"
              />
              <!-- Text 类型 (默认) -->
              <n-input
                v-else
                v-model:value="deployModel.envs[env.name]"
                :placeholder="env.default != null ? env.default : ''"
              />
            </n-form-item>
          </template>
        </n-form>
      </template>

      <!-- 更新模式 -->
      <template v-else>
        <n-form
          ref="formRef"
          :model="deployModel"
          :rules="formRules"
          label-placement="left"
          label-width="160"
        >
          <n-form-item :label="$gettext('Select Compose')" required>
            <n-select
              :value="selectedCompose"
              :options="composeOptions"
              :loading="composeListLoading"
              :placeholder="$gettext('Select a compose to update')"
              filterable
              @update:value="handleComposeSelect"
            />
          </n-form-item>

          <n-divider title-placement="left">{{ $gettext('Deploy Options') }}</n-divider>

          <n-row :gutter="[24, 0]">
            <n-col :span="8">
              <n-form-item path="autoStart" :label="$gettext('Auto Start')">
                <n-switch v-model:value="deployModel.autoStart" />
              </n-form-item>
            </n-col>
          </n-row>

          <!-- 环境变量 -->
          <template v-if="template?.environments?.length">
            <n-divider title-placement="left">{{ $gettext('Environment Variables') }}</n-divider>

            <n-form-item
              v-for="env in template.environments"
              :key="env.name"
              :path="`envs.${env.name}`"
              :label="env.description"
              :required="env.default == null"
            >
              <!-- Select 类型 -->
              <n-select
                v-if="env.type === 'select'"
                v-model:value="deployModel.envs[env.name]"
                :options="getSelectOptions(env)"
                :placeholder="$gettext('Select value')"
              />
              <!-- Number/Port 类型 -->
              <n-input-number
                v-else-if="env.type === 'number' || env.type === 'port'"
                v-model:value="deployModel.envs[env.name]"
                :min="env.type === 'port' ? 1 : undefined"
                :max="env.type === 'port' ? 65535 : undefined"
                style="width: 100%"
                :placeholder="env.default != null ? env.default : ''"
              />
              <!-- Password 类型 -->
              <n-input
                v-else-if="env.type === 'password'"
                v-model:value="deployModel.envs[env.name]"
                type="password"
                show-password-on="click"
                :placeholder="env.default != null ? env.default : ''"
              />
              <!-- URL 类型 -->
              <n-input
                v-else-if="env.type === 'url'"
                v-model:value="deployModel.envs[env.name]"
                :placeholder="env.default != null ? env.default : ''"
              />
              <!-- Text 类型 (默认) -->
              <n-input
                v-else
                v-model:value="deployModel.envs[env.name]"
                :placeholder="env.default != null ? env.default : ''"
              />
            </n-form-item>
          </template>
        </n-form>
      </template>
    </div>

    <!-- 步骤3：预览和编辑 -->
    <div v-else-if="currentStep === 3">
      <n-tabs type="line" animated>
        <n-tab-pane name="compose" :tab="$gettext('Compose File')">
          <!-- 创建模式：普通编辑器 -->
          <template v-if="deployMode === 'create'">
            <common-editor v-model:value="finalCompose" lang="yaml" height="50vh" />
          </template>
          <!-- 更新模式：差异编辑器 -->
          <template v-else>
            <n-alert type="info" style="margin-bottom: 12px">
              {{
                $gettext(
                  'Left side shows the original compose, right side shows the new compose. You can edit the right side.'
                )
              }}
            </n-alert>
            <diff-editor
              v-if="selectedComposeData"
              :original="selectedComposeData.compose"
              v-model:modified="finalCompose"
              lang="yaml"
              height="50vh"
            />
          </template>
        </n-tab-pane>

        <n-tab-pane name="env" :tab="$gettext('Environment Variables')">
          <n-dynamic-input
            v-model:value="finalEnvs"
            preset="pair"
            :key-placeholder="$gettext('Variable Name')"
            :value-placeholder="$gettext('Variable Value')"
          />
        </n-tab-pane>
      </n-tabs>
    </div>

    <!-- 步骤4：确认 -->
    <div v-else-if="currentStep === 4">
      <n-descriptions :column="1" label-placement="left" bordered>
        <n-descriptions-item :label="$gettext('Deploy Mode')">
          <n-tag :type="deployMode === 'create' ? 'success' : 'info'">
            {{ deployMode === 'create' ? $gettext('Create New') : $gettext('Update Existing') }}
          </n-tag>
        </n-descriptions-item>
        <n-descriptions-item :label="$gettext('Compose Name')">
          {{ deployMode === 'create' ? deployModel.name : selectedCompose }}
        </n-descriptions-item>
        <n-descriptions-item :label="$gettext('Auto Start')">
          <n-tag :type="deployModel.autoStart ? 'success' : 'default'">
            {{ deployModel.autoStart ? $gettext('Yes') : $gettext('No') }}
          </n-tag>
        </n-descriptions-item>
        <n-descriptions-item v-if="deployMode === 'create'" :label="$gettext('Auto Firewall')">
          <n-tag :type="deployModel.autoFirewall ? 'success' : 'default'">
            {{ deployModel.autoFirewall ? $gettext('Yes') : $gettext('No') }}
          </n-tag>
        </n-descriptions-item>
        <n-descriptions-item :label="$gettext('Environment Variables')">
          {{ finalEnvs.length }} {{ $gettext('variables') }}
        </n-descriptions-item>
      </n-descriptions>

      <n-divider />

      <n-collapse>
        <n-collapse-item :title="$gettext('Compose Content')" name="compose">
          <common-editor :value="finalCompose" lang="yaml" height="30vh" read-only />
        </n-collapse-item>
      </n-collapse>
    </div>

    <template #footer>
      <n-flex justify="space-between">
        <n-button v-if="currentStep > 1" @click="handlePrev" :disabled="doSubmit">
          {{ $gettext('Previous') }}
        </n-button>
        <div v-else />

        <n-flex>
          <n-button @click="show = false" :disabled="doSubmit">
            {{ $gettext('Cancel') }}
          </n-button>
          <n-button v-if="currentStep === 2" type="primary" @click="handleStep2Next">
            {{ $gettext('Next') }}
          </n-button>
          <n-button v-else-if="currentStep === 3" type="primary" @click="handleStep3Next">
            {{ $gettext('Next') }}
          </n-button>
          <n-button
            v-else-if="currentStep === 4"
            type="primary"
            :loading="doSubmit"
            :disabled="doSubmit"
            @click="handleSubmit"
          >
            {{ deployMode === 'create' ? $gettext('Create') : $gettext('Update') }}
          </n-button>
        </n-flex>
      </n-flex>
    </template>
  </n-modal>

  <pty-terminal-modal
    v-model:show="upModal"
    :title="
      $gettext('Starting Compose') +
      ' - ' +
      (deployMode === 'create' ? deployModel.name : selectedCompose)
    "
    :command="upCommand"
    @complete="handleUpComplete"
  />
</template>
