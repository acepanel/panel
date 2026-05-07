<script setup lang="ts">
import { useThemeVars } from 'naive-ui'
import { useGettext } from 'vue3-gettext'

import file from '@/api/panel/file'
import { useEditorStore } from '@/stores'
import { decodeBase64 } from '@/utils'

const { $gettext } = useGettext()
const editorStore = useEditorStore()
const themeVars = useThemeVars()

const emit = defineEmits<{
  (e: 'search'): void
  (e: 'replace'): void
  (e: 'goto'): void
  (e: 'settings'): void
}>()

const saving = ref(false)
const savingAll = ref(false)

// 保存当前文件
function handleSave() {
  const tab = editorStore.activeTab
  if (!tab) {
    window.$message.warning($gettext('No file to save'))
    return
  }

  if (!tab.modified) {
    window.$message.info($gettext('No changes to save'))
    return
  }

  saving.value = true
  useRequest(file.save(tab.path, tab.content))
    .onSuccess(() => {
      editorStore.markSaved(tab.path)
      window.$message.success($gettext('Saved successfully'))
    })
    .onComplete(() => {
      saving.value = false
    })
}

// 保存所有文件
async function handleSaveAll() {
  const unsavedTabs = editorStore.unsavedTabs
  if (unsavedTabs.length === 0) {
    window.$message.info($gettext('No changes to save'))
    return
  }

  savingAll.value = true
  let successCount = 0
  let failCount = 0

  for (const tab of unsavedTabs) {
    try {
      await new Promise<void>((resolve, reject) => {
        useRequest(file.save(tab.path, tab.content))
          .onSuccess(() => {
            editorStore.markSaved(tab.path)
            successCount++
            resolve()
          })
          .onError(() => {
            failCount++
            reject()
          })
      })
    } catch {
      // 继续处理下一个文件
    }
  }

  savingAll.value = false

  if (failCount === 0) {
    window.$message.success($gettext('All files saved successfully'))
  } else {
    window.$message.warning(
      $gettext('Saved %{ success } files, %{ fail } failed', {
        success: successCount,
        fail: failCount,
      }),
    )
  }
}

// 刷新当前文件
function handleRefresh() {
  const tab = editorStore.activeTab
  if (!tab) return

  if (tab.modified) {
    window.$dialog.warning({
      title: $gettext('Unsaved Changes'),
      content: $gettext('This file has unsaved changes. Refreshing will discard them. Continue?'),
      positiveText: $gettext('Refresh'),
      negativeText: $gettext('Cancel'),
      onPositiveClick: () => {
        doRefresh(tab.path)
      },
    })
  } else {
    doRefresh(tab.path)
  }
}

function doRefresh(path: string) {
  editorStore.setLoading(path, true)
  useRequest(file.content(encodeURIComponent(path)))
    .onSuccess(({ data }) => {
      const content = decodeBase64(data.content)
      editorStore.reloadFile(path, content)
      window.$message.success($gettext('Refreshed successfully'))
    })
    .onComplete(() => {
      editorStore.setLoading(path, false)
    })
}

// 搜索
function handleSearch() {
  emit('search')
}

// 替换
function handleReplace() {
  emit('replace')
}

// 跳转行
function handleGoto() {
  emit('goto')
}

// 设置
function handleSettings() {
  emit('settings')
}

// 字体大小调整
function handleFontSizeChange(value: number | null) {
  if (value !== null) {
    editorStore.updateSettings({ fontSize: value })
  }
}

// 切换小地图
function handleToggleMinimap() {
  editorStore.updateSettings({ minimap: !editorStore.settings.minimap })
}

// 切换自动换行
function handleToggleWordWrap() {
  const current = editorStore.settings.wordWrap
  editorStore.updateSettings({ wordWrap: current === 'on' ? 'off' : 'on' })
}

// 暴露方法供外部调用
defineExpose({
  save: handleSave,
  saveAll: handleSaveAll,
  refresh: handleRefresh,
})
</script>

<template>
  <div class="editor-toolbar">
    <n-flex align="center" :wrap="false">
      <!-- 文件操作 -->
      <n-button-group size="small">
        <n-button
          @click="handleSave"
          :disabled="!editorStore.activeTab?.modified"
          :loading="saving"
          :title="$gettext('Save (Ctrl+S)')"
        >
          <template #icon>
            <i-mdi-content-save />
          </template>
          {{ $gettext('Save') }}
        </n-button>
        <n-button
          @click="handleSaveAll"
          :disabled="!editorStore.hasUnsavedFiles"
          :loading="savingAll"
          :title="$gettext('Save All (Ctrl+Shift+S)')"
        >
          <template #icon>
            <i-mdi-content-save-all />
          </template>
          {{ $gettext('Save All') }}
        </n-button>
        <n-button
          @click="handleRefresh"
          :disabled="!editorStore.activeTab"
          :title="$gettext('Refresh')"
        >
          <template #icon>
            <i-mdi-refresh />
          </template>
          {{ $gettext('Refresh') }}
        </n-button>
      </n-button-group>

      <n-divider vertical />

      <!-- 编辑操作 -->
      <n-button-group size="small">
        <n-button
          @click="handleSearch"
          :disabled="!editorStore.activeTab"
          :title="$gettext('Search (Ctrl+F)')"
        >
          <template #icon>
            <i-mdi-magnify />
          </template>
          {{ $gettext('Search') }}
        </n-button>
        <n-button
          @click="handleReplace"
          :disabled="!editorStore.activeTab"
          :title="$gettext('Replace (Ctrl+H)')"
        >
          <template #icon>
            <i-mdi-find-replace />
          </template>
          {{ $gettext('Replace') }}
        </n-button>
        <n-button
          @click="handleGoto"
          :disabled="!editorStore.activeTab"
          :title="$gettext('Go to Line (Ctrl+G)')"
        >
          <template #icon>
            <i-mdi-arrow-right-bold />
          </template>
          {{ $gettext('Go to') }}
        </n-button>
      </n-button-group>

      <n-divider vertical />

      <!-- 视图操作 -->
      <n-input-number
        :value="editorStore.settings.fontSize"
        @update:value="handleFontSizeChange"
        size="small"
        button-placement="both"
        :min="10"
        :max="24"
        :show-button="true"
        class="w-20"
      >
        <template #minus-icon>
          <i-mdi-format-font-size-decrease />
        </template>
        <template #add-icon>
          <i-mdi-format-font-size-increase />
        </template>
      </n-input-number>

      <n-button
        size="small"
        :type="editorStore.settings.wordWrap === 'on' ? 'primary' : 'default'"
        @click="handleToggleWordWrap"
        :title="$gettext('Toggle Word Wrap')"
      >
        <template #icon>
          <i-mdi-wrap />
        </template>
      </n-button>

      <n-button
        size="small"
        :type="editorStore.settings.minimap ? 'primary' : 'default'"
        @click="handleToggleMinimap"
        :title="$gettext('Toggle Minimap')"
      >
        <template #icon>
          <i-mdi-map-outline />
        </template>
      </n-button>

      <div class="spacer" />

      <!-- 设置 -->
      <n-button size="small" quaternary @click="handleSettings" :title="$gettext('Settings')">
        <template #icon>
          <i-mdi-cog />
        </template>
      </n-button>
    </n-flex>
  </div>
</template>

<style scoped lang="scss">
.editor-toolbar {
  padding: 8px 12px;
  border-bottom: 1px solid v-bind('themeVars.borderColor');
  background: v-bind('themeVars.cardColor');
  flex-shrink: 0;
}

.spacer {
  flex: 1;
}
</style>
