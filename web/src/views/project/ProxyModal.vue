<script setup lang="ts">
import type { SelectOption } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import website from '@/api/panel/website'
import ListInput from '@/components/common/ListInput.vue'
import { useFormLayout } from '@/components/system/composables/useFormLayout'
import { useThemeStore } from '@/stores'
import WebsiteEditModal from '@/views/website/EditModal.vue'

import type { ProxyWebsiteForm } from './composables/useProxyWebsite'
import { useProxyWebsite } from './composables/useProxyWebsite'
import ProxyForm from './ProxyForm.vue'

const show = defineModel<boolean>('show', { type: Boolean, required: true })
const props = defineProps<{ project: { id: number; name: string } }>()

const { $gettext } = useGettext()
const themeStore = useThemeStore()
const formLayout = useFormLayout(100)
const { validateProxyWebsite, createProxyWebsite } = useProxyWebsite()

const collapsed = ref(false)
// 右侧面板：查看已绑定的网站、新建或绑定已有
const mode = ref<'detail' | 'create' | 'bind'>('detail')
const selectedId = ref<number>()

// 不限类型，绑定后切换过类型的网站也要能看到
const {
  data: websites,
  loading,
  send: loadWebsites,
} = useRequest(() => website.list('all', 1, 100, '', props.project.id), {
  immediate: false,
  initialData: { items: [] },
})
const list = computed<any[]>(() => websites.value?.items ?? [])
const current = computed(() => list.value.find((item) => item.id === selectedId.value))

// 正在查看的网站不在了（刚打开、解绑或被删除）就换到第一个，一个都没有直接进入新建；
// 新建和绑定填到一半时不打断
const refresh = async () => {
  await loadWebsites()
  if (mode.value !== 'detail' || current.value) return
  selectedId.value = list.value[0]?.id
  if (!selectedId.value) openCreate()
}

watch(show, (value) => {
  if (!value) return
  mode.value = 'detail'
  selectedId.value = undefined
  collapsed.value = themeStore.isMobile
  refresh()
})

const handleSelect = (id: number) => {
  selectedId.value = id
  mode.value = 'detail'
  // 手机上展开的列表会挤掉大半详情区，选中后收起
  if (themeStore.isMobile) collapsed.value = true
}

const switching = reactive(new Set<number>())

const handleStatus = (item: any, status: boolean) => {
  switching.add(item.id)
  useRequest(website.status(item.id, status))
    .onSuccess(() => {
      item.status = status
      window.$message.success(
        status ? $gettext('Started successfully') : $gettext('Stopped successfully'),
      )
    })
    .onComplete(() => switching.delete(item.id))
}

// ==================== 快捷设置 ====================
const setting = ref<any>(null)
const quick = ref({ domains: [] as string[], port: null as number | null })
const detailLoading = ref(false)
const saving = ref(false)

// 只有根路径代理到本机端口时才能快捷改端口，其余写法交给完整设置
const LOCAL_PASS = /^(https?:\/\/(?:127\.0\.0\.1|localhost|\[::1\])):(\d+)\/?$/
const rootProxy = computed(() =>
  setting.value?.proxies?.find((p: any) => p.location.replace(/^(\^~|=)\s*/, '') === '/'),
)
const localPass = computed(() => rootProxy.value?.pass.match(LOCAL_PASS))

const loadSetting = (id: number) => {
  detailLoading.value = true
  useRequest(website.config(id))
    .onSuccess(({ data }) => {
      // 切换得快时丢弃上一个网站晚到的结果
      if (id !== selectedId.value) return
      setting.value = data
      quick.value = {
        domains: [...(data.domains || [])],
        port: localPass.value ? Number(localPass.value[2]) : null,
      }
    })
    .onComplete(() => {
      detailLoading.value = false
    })
}

watch(selectedId, (id) => {
  setting.value = null
  if (id) loadSetting(id)
})

const handleSave = () => {
  const domains = quick.value.domains.filter((d) => d.trim() !== '')
  if (domains.length === 0) {
    window.$message.warning($gettext('Please enter at least one domain'))
    return
  }
  setting.value.domains = domains
  if (localPass.value && quick.value.port) {
    rootProxy.value.pass = `${localPass.value[1]}:${quick.value.port}`
  }

  saving.value = true
  useRequest(website.saveConfig(current.value.id, setting.value))
    .onSuccess(() => {
      window.$message.success($gettext('Saved successfully'))
      window.$bus.emit('website:refresh')
    })
    .onComplete(() => {
      saving.value = false
    })
}

const handleUnbind = () => {
  useRequest(website.updateProject(current.value.id, 0)).onSuccess(() => {
    window.$message.success($gettext('Unbound successfully'))
    window.$bus.emit('website:refresh')
  })
}

// 网站编辑弹窗挂载时会拉证书等数据，首次打开时才挂载
const editModal = ref(false)
const editId = ref(0)
const openFullSettings = async () => {
  editId.value = current.value.id
  await nextTick()
  editModal.value = true
}

// 自己保存、完整设置里保存或网站页改动后都同步刷新
const handleWebsiteRefresh = () => {
  if (!show.value) return
  refresh()
  if (selectedId.value && mode.value === 'detail') loadSetting(selectedId.value)
}
onMounted(() => window.$bus.on('website:refresh', handleWebsiteRefresh))
onUnmounted(() => window.$bus.off('website:refresh', handleWebsiteRefresh))

// ==================== 新建 ====================
const createForm = ref<ProxyWebsiteForm>({ name: '', domains: [], port: null })
const creating = ref(false)

function openCreate() {
  // 网站名不能重复，第一个沿用项目名，之后加序号
  createForm.value = {
    name: list.value.length ? `${props.project.name}-${list.value.length + 1}` : props.project.name,
    domains: [],
    port: null,
  }
  mode.value = 'create'
}

const handleCreate = async () => {
  if (!validateProxyWebsite(createForm.value)) return
  creating.value = true
  if (await createProxyWebsite(props.project, createForm.value)) {
    await loadWebsites()
    const created = list.value.find((item) => item.name === createForm.value.name)
    if (created) handleSelect(created.id)
  }
  creating.value = false
}

// ==================== 绑定已有 ====================
const bindId = ref<number | null>(null)
const binding = ref(false)
const {
  data: candidates,
  loading: candidatesLoading,
  send: searchCandidates,
} = useRequest((keyword: string) => website.list('proxy', 1, 20, keyword), {
  immediate: false,
  initialData: { items: [] },
})
const candidateOptions = computed(() =>
  (candidates.value?.items ?? [])
    .filter((item: any) => item.project_id !== props.project.id)
    .map((item: any) => ({
      label: item.name,
      value: item.id,
      disabled: item.project_id !== 0,
      description: item.project_id
        ? $gettext('Bound to another project')
        : (item.domains || []).join(', '),
    })),
)
const renderCandidate = (option: SelectOption) =>
  h('div', { class: 'flex items-center gap-3' }, [
    h('span', option.label as string),
    h('span', { class: 'text-xs text-gray-400 truncate' }, option.description as string),
  ])

const openBind = () => {
  bindId.value = null
  mode.value = 'bind'
  searchCandidates('')
}

const handleBind = () => {
  if (!bindId.value) return
  const id = bindId.value
  binding.value = true
  useRequest(website.updateProject(id, props.project.id))
    .onSuccess(async () => {
      window.$message.success($gettext('Bound successfully'))
      await loadWebsites()
      handleSelect(id)
    })
    .onComplete(() => {
      binding.value = false
    })
}
</script>

<template>
  <app-modal
    v-model:show="show"
    size="lg"
    :title="$gettext('Reverse Proxy - %{ name }', { name: project.name })"
  >
    <n-layout has-sider class="h-120 !bg-transparent">
      <n-layout-sider
        v-model:collapsed="collapsed"
        :width="200"
        :collapsed-width="16"
        collapse-mode="width"
        show-trigger="arrow-circle"
        bordered
        :native-scrollbar="false"
        content-class="pr-3"
        class="!bg-transparent"
      >
        <n-flex v-show="!collapsed" vertical :size="4">
          <n-flex :size="8" :wrap="false" class="mb-2">
            <n-button size="small" type="primary" class="flex-1" @click="openCreate">
              {{ $gettext('Create') }}
            </n-button>
            <n-button size="small" class="flex-1" @click="openBind">
              {{ $gettext('Bind') }}
            </n-button>
          </n-flex>
          <div
            v-for="item in list"
            :key="item.id"
            class="hover-bg px-3 py-2 rounded cursor-pointer"
            :class="{ 'bg-bg-subtle': mode === 'detail' && item.id === selectedId }"
            @click="handleSelect(item.id)"
          >
            <n-ellipsis class="w-full">{{ item.name }}</n-ellipsis>
            <n-flex align="center" justify="space-between" :wrap="false" class="mt-1">
              <n-ellipsis class="text-xs text-gray-400 min-w-0">
                {{ item.domains?.[0] || '-' }}
              </n-ellipsis>
              <n-switch
                size="small"
                :value="item.status"
                :loading="switching.has(item.id)"
                @click.stop
                @update:value="(value: boolean) => handleStatus(item, value)"
              />
            </n-flex>
          </div>
          <n-empty
            v-if="!loading && !list.length"
            size="small"
            class="py-6"
            :description="$gettext('No reverse proxy website bound')"
          />
        </n-flex>
      </n-layout-sider>
      <n-layout-content class="!bg-transparent" content-class="h-full pl-4">
        <!-- 新建 -->
        <n-form v-if="mode === 'create'" v-bind="formLayout">
          <n-form-item :label="$gettext('Name')">
            <n-input v-model:value="createForm.name" @keydown.enter.prevent />
          </n-form-item>
          <proxy-form v-model:value="createForm" />
          <n-flex justify="end">
            <n-button type="primary" :loading="creating" :disabled="creating" @click="handleCreate">
              {{ $gettext('Create') }}
            </n-button>
          </n-flex>
        </n-form>

        <!-- 绑定已有 -->
        <n-form v-else-if="mode === 'bind'" v-bind="formLayout">
          <n-form-item :label="$gettext('Website')">
            <n-select
              v-model:value="bindId"
              :options="candidateOptions"
              :loading="candidatesLoading"
              :render-label="renderCandidate"
              :placeholder="$gettext('Search by name or remark')"
              filterable
              remote
              @search="searchCandidates"
            />
            <template #feedback>
              <span class="text-gray-400">
                {{
                  $gettext('Only reverse proxy websites not bound to other projects can be bound')
                }}
              </span>
            </template>
          </n-form-item>
          <n-flex justify="end">
            <n-button
              type="primary"
              :loading="binding"
              :disabled="!bindId || binding"
              @click="handleBind"
            >
              {{ $gettext('Bind') }}
            </n-button>
          </n-flex>
        </n-form>

        <!-- 快捷设置 -->
        <n-spin v-else-if="current" :show="detailLoading">
          <n-form v-bind="formLayout">
            <n-form-item :label="$gettext('Domain')">
              <ListInput v-model:value="quick.domains" placeholder="example.com" :min="1" />
            </n-form-item>
            <n-form-item :label="$gettext('Project Port')">
              <n-input-number
                v-model:value="quick.port"
                :min="1"
                :max="65535"
                :disabled="!localPass"
                class="w-full md:!w-60"
                :placeholder="$gettext('e.g., 3000')"
              />
              <template v-if="setting && !localPass" #feedback>
                <span class="text-gray-400">
                  {{
                    $gettext(
                      'The root path does not proxy to a local port, please modify it in full settings',
                    )
                  }}
                </span>
              </template>
            </n-form-item>
          </n-form>
          <n-flex justify="end">
            <n-popconfirm @positive-click="handleUnbind">
              <template #trigger>
                <n-button>{{ $gettext('Unbind') }}</n-button>
              </template>
              {{
                $gettext('Unbind website %{ name } from this project? The website will be kept.', {
                  name: current.name,
                })
              }}
            </n-popconfirm>
            <n-button @click="openFullSettings">{{ $gettext('Full Settings') }}</n-button>
            <n-button
              type="primary"
              :loading="saving"
              :disabled="saving || !setting"
              @click="handleSave"
            >
              {{ $gettext('Save') }}
            </n-button>
          </n-flex>
        </n-spin>
      </n-layout-content>
    </n-layout>
  </app-modal>

  <website-edit-modal v-if="editId" v-model:show="editModal" v-model:edit-id="editId" />
</template>
