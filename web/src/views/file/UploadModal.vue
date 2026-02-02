<script setup lang="ts">
import { sha256 } from 'js-sha256'
import type { UploadCustomRequestOptions, UploadFileInfo, UploadInst } from 'naive-ui'
import pLimit from 'p-limit'
import { useGettext } from 'vue3-gettext'

import api from '@/api/panel/file'

const { $gettext } = useGettext()
const show = defineModel<boolean>('show', { type: Boolean, required: true })
const path = defineModel<string>('path', { type: String, required: true })

const props = defineProps<{
  initialFiles?: File[]
}>()

const upload = ref<UploadInst | null>(null)
const fileList = ref<UploadFileInfo[]>([])

// 覆盖上传选项
const forceOverwrite = ref(false)

// 文件数量阈值，超过此数量需要二次确认
const FILE_COUNT_THRESHOLD = 100
// 大文件阈值，超过此大小使用分块上传 (100MB)
const LARGE_FILE_THRESHOLD = 100 * 1024 * 1024
// 分块大小 (5MB)
const CHUNK_SIZE = 5 * 1024 * 1024
// 分块上传重试次数
const CHUNK_RETRY_COUNT = 10
// 并发上传数
const CONCURRENT_UPLOADS = 3

// 上传速度状态（每个文件独立）
interface UploadProgress {
  fileName: string
  speed: string
}
const uploadProgressMap = ref<Map<string, UploadProgress>>(new Map())

// 每个上传任务的状态
interface UploadTask {
  isCancelled: boolean
  activeRequests: { abort: () => void }[]
}

// 以文件唯一标识为 key 存储每个上传任务的状态
const uploadTasks = new Map<string, UploadTask>()

// 获取文件唯一标识
const getFileKey = (file: File) => `${file.name}-${file.size}-${file.lastModified}`

// 取消单个文件的上传
const cancelUpload = (file: File) => {
  const fileKey = getFileKey(file)
  const task = uploadTasks.get(fileKey)
  if (task) {
    task.isCancelled = true
    task.activeRequests.forEach((req) => req.abort())
    uploadTasks.delete(fileKey)
  }
}

// 取消所有上传
const cancelAllUploads = () => {
  uploadTasks.forEach((task) => {
    task.isCancelled = true
    task.activeRequests.forEach((req) => req.abort())
  })
  uploadTasks.clear()
}

// 计算文件标识符（快速，用于断点续传识别）
// 使用文件元数据 + 首尾采样计算，避免读取整个大文件
const calculateFileIdentifier = async (file: File): Promise<string> => {
  const sampleSize = 1024 * 1024 // 1MB samples

  // 读取首部
  const headChunk = file.slice(0, Math.min(sampleSize, file.size))
  const headBuffer = await headChunk.arrayBuffer()

  // 读取尾部（如果文件足够大）
  let tailBuffer: ArrayBuffer
  if (file.size > sampleSize * 2) {
    const tailChunk = file.slice(file.size - sampleSize, file.size)
    tailBuffer = await tailChunk.arrayBuffer()
  } else {
    tailBuffer = headBuffer
  }

  // 组合元数据
  const metadata = `${file.name}|${file.size}|${file.lastModified}`
  const metaBuffer = new TextEncoder().encode(metadata)

  // 合并所有数据计算hash
  const combined = new Uint8Array(
    metaBuffer.byteLength + headBuffer.byteLength + tailBuffer.byteLength
  )
  combined.set(new Uint8Array(metaBuffer), 0)
  combined.set(new Uint8Array(headBuffer), metaBuffer.byteLength)
  combined.set(new Uint8Array(tailBuffer), metaBuffer.byteLength + headBuffer.byteLength)

  return sha256(combined)
}

// 计算分块SHA256
const calculateChunkHash = async (chunk: Blob): Promise<string> => {
  const buffer = await chunk.arrayBuffer()
  return sha256(new Uint8Array(buffer))
}

// 格式化速度显示
const formatSpeed = (bytesPerSecond: number): string => {
  if (bytesPerSecond < 1024) {
    return `${bytesPerSecond.toFixed(0)} B/s`
  } else if (bytesPerSecond < 1024 * 1024) {
    return `${(bytesPerSecond / 1024).toFixed(1)} KB/s`
  } else if (bytesPerSecond < 1024 * 1024 * 1024) {
    return `${(bytesPerSecond / 1024 / 1024).toFixed(1)} MB/s`
  } else {
    return `${(bytesPerSecond / 1024 / 1024 / 1024).toFixed(2)} GB/s`
  }
}

// 带重试的分块上传
const uploadChunkWithRetry = async (
  formData: FormData,
  chunkIndex: number,
  chunkSize: number,
  onChunkComplete: (size: number) => void,
  task: UploadTask
): Promise<void> => {
  let lastError: Error | null = null
  for (let attempt = 1; attempt <= CHUNK_RETRY_COUNT; attempt++) {
    // 检查是否已取消
    if (task.isCancelled) {
      throw new DOMException('Upload cancelled', 'AbortError')
    }
    try {
      const method = api.chunkUpload(formData)
      task.activeRequests.push(method)
      try {
        await method
        onChunkComplete(chunkSize)
        return
      } finally {
        // 从活跃请求列表中移除
        const index = task.activeRequests.indexOf(method)
        if (index > -1) {
          task.activeRequests.splice(index, 1)
        }
      }
    } catch (error) {
      // 如果是取消错误，直接抛出
      if (task.isCancelled || (error as Error).message?.includes('abort')) {
        throw new DOMException('Upload cancelled', 'AbortError')
      }
      lastError = error as Error
      console.warn(
        `Chunk ${chunkIndex} upload failed (attempt ${attempt}/${CHUNK_RETRY_COUNT}):`,
        error
      )
      if (attempt < CHUNK_RETRY_COUNT) {
        // 等待一段时间后重试，指数退避
        await new Promise((resolve) =>
          setTimeout(resolve, Math.min(1000 * Math.pow(2, attempt - 1), 10000))
        )
      }
    }
  }
  throw new Error(
    `Chunk ${chunkIndex} upload failed after ${CHUNK_RETRY_COUNT} attempts: ${lastError?.message}`
  )
}

// 分块上传
const chunkedUpload = async (
  file: File,
  onProgress: (e: { percent: number }) => void,
  onFinish: () => void,
  onError: () => void
) => {
  // 创建此文件的上传任务
  const fileKey = getFileKey(file)
  const task: UploadTask = { isCancelled: false, activeRequests: [] }
  uploadTasks.set(fileKey, task)

  // 初始化进度显示
  uploadProgressMap.value.set(fileKey, { fileName: file.name, speed: '' })

  try {
    // 计算文件标识符（快速）
    onProgress({ percent: 0 })
    const fileHash = await calculateFileIdentifier(file)

    // 检查是否已取消
    if (task.isCancelled) {
      throw new DOMException('Upload cancelled', 'AbortError')
    }

    // 计算分块数量
    const chunkCount = Math.ceil(file.size / CHUNK_SIZE)

    // 开始分块上传（查询已上传的分块）
    const startMethod = api.chunkStart({
      path: path.value,
      file_name: file.name,
      file_hash: fileHash,
      chunk_count: chunkCount,
      force: forceOverwrite.value
    })
    task.activeRequests.push(startMethod)
    const startRes = await startMethod
    task.activeRequests = task.activeRequests.filter((r) => r !== startMethod)

    const uploadedChunks: Set<number> = new Set(startRes.uploaded_chunks)

    // 速度计算相关
    let uploadedBytes = uploadedChunks.size * CHUNK_SIZE
    let lastTime = Date.now()
    let lastBytes = uploadedBytes

    // 更新进度和速度
    const updateProgress = () => {
      const now = Date.now()
      const timeDiff = (now - lastTime) / 1000 // 秒
      if (timeDiff >= 0.5) {
        // 每0.5秒更新一次速度
        const bytesDiff = uploadedBytes - lastBytes
        const speed = bytesDiff / timeDiff
        uploadProgressMap.value.set(fileKey, { fileName: file.name, speed: formatSpeed(speed) })
        lastTime = now
        lastBytes = uploadedBytes
      }
      const percent = Math.ceil((uploadedBytes / file.size) * 100)
      onProgress({ percent: Math.min(percent, 99) }) // 最多99%，留1%给finish
    }

    // 分块完成回调
    const onChunkComplete = (size: number) => {
      uploadedBytes += size
      updateProgress()
    }

    // 构建待上传分块列表
    const pendingChunks: number[] = []
    for (let i = 0; i < chunkCount; i++) {
      if (!uploadedChunks.has(i)) {
        pendingChunks.push(i)
      }
    }

    // 并发上传单个分块
    const uploadChunk = async (chunkIndex: number) => {
      // 检查是否已取消
      if (task.isCancelled) {
        throw new DOMException('Upload cancelled', 'AbortError')
      }

      const start = chunkIndex * CHUNK_SIZE
      const end = Math.min(start + CHUNK_SIZE, file.size)
      const chunk = file.slice(start, end)
      const chunkSize = end - start

      // 计算分块hash
      const chunkHash = await calculateChunkHash(chunk)

      // 上传分块
      const formData = new FormData()
      formData.append('path', path.value)
      formData.append('file_name', file.name)
      formData.append('file_hash', fileHash)
      formData.append('chunk_index', chunkIndex.toString())
      formData.append('chunk_hash', chunkHash)
      formData.append('file', chunk)

      await uploadChunkWithRetry(formData, chunkIndex, chunkSize, onChunkComplete, task)
    }

    // 控制并发
    const limit = pLimit(CONCURRENT_UPLOADS)
    await Promise.all(pendingChunks.map((chunkIndex) => limit(() => uploadChunk(chunkIndex))))

    // 检查是否已取消
    if (task.isCancelled) {
      throw new DOMException('Upload cancelled', 'AbortError')
    }

    // 完成分块上传（合并）
    const finishMethod = api.chunkFinish({
      path: path.value,
      file_name: file.name,
      file_hash: fileHash,
      chunk_count: chunkCount,
      force: forceOverwrite.value
    })
    task.activeRequests.push(finishMethod)
    await finishMethod
    task.activeRequests = task.activeRequests.filter((r) => r !== finishMethod)

    onProgress({ percent: 100 })
    uploadProgressMap.value.delete(fileKey)
    uploadTasks.delete(fileKey)
    if (!task.isCancelled) {
      onFinish()
      window.$message.success($gettext('Upload %{ fileName } successful', { fileName: file.name }))
    }
    window.$bus.emit('file:refresh')
  } catch (error) {
    uploadProgressMap.value.delete(fileKey)
    uploadTasks.delete(fileKey)

    // 如果是取消错误，静默处理
    if ((error as Error).name === 'AbortError' || task.isCancelled) {
      console.log('Upload cancelled by user')
      return
    }

    console.error('Chunked upload error:', error)
    if (!task.isCancelled) {
      onError()
    }
  }
}

// 监听预拖入的文件
watch(
  () => props.initialFiles,
  async (files) => {
    if (files && files.length > 0) {
      // 如果文件数量超过阈值，弹窗确认
      if (files.length > FILE_COUNT_THRESHOLD) {
        window.$dialog.warning({
          title: $gettext('Confirm Upload'),
          content: $gettext(
            'You are about to upload %{count} files. This may take a while. Do you want to continue?',
            { count: files.length }
          ),
          positiveText: $gettext('Continue'),
          negativeText: $gettext('Cancel'),
          onPositiveClick: () => {
            addFilesToList(files, true)
          },
          onNegativeClick: () => {
            show.value = false
          }
        })
      } else {
        addFilesToList(files, true)
      }
    }
  },
  { immediate: true }
)

// 将文件添加到上传列表
const addFilesToList = (files: File[], autoUpload: boolean = false) => {
  fileList.value = files.map((file, index) => ({
    id: `dropped-${Date.now()}-${index}`,
    name: file.name,
    status: 'pending' as const,
    file: file
  }))

  // 自动开始上传
  if (autoUpload) {
    nextTick(() => {
      upload.value?.submit()
    })
  }
}

// 监听弹窗关闭，清空文件列表并取消上传
watch(show, (val) => {
  if (!val) {
    cancelAllUploads()
    fileList.value = []
  }
})

const uploadRequest = ({ file, onFinish, onError, onProgress }: UploadCustomRequestOptions) => {
  const fileObj = file.file as File

  // 大文件使用分块上传
  if (fileObj.size > LARGE_FILE_THRESHOLD) {
    chunkedUpload(fileObj, onProgress, onFinish, onError)
    return
  }

  // 小文件使用普通上传
  const fileKey = getFileKey(fileObj)
  const task: UploadTask = { isCancelled: false, activeRequests: [] }
  uploadTasks.set(fileKey, task)

  const formData = new FormData()
  formData.append('path', `${path.value}/${file.name}`)
  formData.append('file', fileObj)
  formData.append('force', forceOverwrite.value.toString())

  const method = api.upload(formData)
  task.activeRequests.push(method)

  const { uploading } = useRequest(method)
    .onSuccess(() => {
      uploadTasks.delete(fileKey)
      if (!task.isCancelled) {
        onFinish()
        window.$bus.emit('file:refresh')
        window.$message.success($gettext('Upload %{ fileName } successful', { fileName: file.name }))
      }
    })
    .onError(() => {
      uploadTasks.delete(fileKey)
      if (!task.isCancelled) {
        onError()
      }
    })
    .onComplete(() => {
      stopWatch()
    })
  const stopWatch = watch(uploading, (progress) => {
    if (!task.isCancelled) {
      onProgress({ percent: Math.ceil((progress.loaded / progress.total) * 100) })
    }
  })
}

// 处理文件移除（取消上传）
const handleRemove = ({ file }: { file: UploadFileInfo }) => {
  if (file.file) {
    cancelUpload(file.file)
  }
}

// 处理文件选择变化（用于文件数量确认）
const handleChange = (data: { fileList: UploadFileInfo[] }) => {
  const newFiles = data.fileList.filter(
    (f) => !fileList.value.some((existing) => existing.id === f.id)
  )

  // 如果新增文件数量超过阈值，弹窗确认
  if (newFiles.length > FILE_COUNT_THRESHOLD) {
    window.$dialog.warning({
      title: $gettext('Confirm Upload'),
      content: $gettext(
        'You are about to upload %{count} files. This may take a while. Do you want to continue?',
        { count: newFiles.length }
      ),
      positiveText: $gettext('Continue'),
      negativeText: $gettext('Cancel'),
      onPositiveClick: () => {
        fileList.value = data.fileList
      }
    })
  } else {
    fileList.value = data.fileList
  }
}
</script>

<template>
  <n-modal
    v-model:show="show"
    preset="card"
    :title="$gettext('Upload')"
    style="width: 60vw"
    size="huge"
    :bordered="false"
    :segmented="false"
  >
    <n-flex vertical>
      <!-- 覆盖上传选项 -->
      <n-checkbox v-model:checked="forceOverwrite">
        {{ $gettext('Overwrite existing files') }}
      </n-checkbox>
      <!-- 上传速度显示 -->
      <n-flex
        v-for="[key, progress] in uploadProgressMap"
        :key="key"
        justify="space-between"
        align="center"
        class="upload-speed-bar"
      >
        <NText>{{ progress.fileName }}</NText>
        <NText type="success">{{ progress.speed || $gettext('Preparing...') }}</NText>
      </n-flex>
      <n-upload
        ref="upload"
        v-model:file-list="fileList"
        multiple
        directory-dnd
        :custom-request="uploadRequest"
        @change="handleChange"
        @remove="handleRemove"
      >
        <n-upload-dragger>
          <div style="margin-bottom: 12px">
            <the-icon :size="60" icon="mdi:arrow-up-bold-box-outline" />
          </div>
          <NText text-18> {{ $gettext('Click or drag files to this area to upload') }}</NText>
          <NP depth="3" m-10>
            {{
              $gettext('For large files, it is recommended to use SFTP and other methods to upload')
            }}
          </NP>
        </n-upload-dragger>
      </n-upload>
    </n-flex>
  </n-modal>
</template>

<style scoped lang="scss">
.upload-speed-bar {
  padding: 8px 12px;
  background: var(--n-color-embedded);
  border-radius: 4px;
  margin-bottom: 12px;
}
</style>
