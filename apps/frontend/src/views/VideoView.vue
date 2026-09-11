<script setup lang="ts">
import { onUnmounted, reactive, ref, watch } from 'vue'
import { RouterLink, useRouter } from 'vue-router'

import AppShell from '../components/AppShell.vue'
import { ApiError } from '../api/client'
import * as videoApi from '../api/video'
import { MAX_VIDEO_SIZE_MB } from '../api/upload-config'
import type { Video } from '../api/types'
import { useAuthStore } from '../stores/auth'
import { useToastStore } from '../stores/toast'

const router = useRouter()
const auth = useAuthStore()
const toast = useToastStore()

const busy = ref(false)
const stage = ref('')
const uploadPercent = ref(0)
const published = ref<Video | null>(null)
const uploadError = ref('')
const needsLogin = ref(false)
// 重试封面或发布时，复用当前页面已成功上传的文件，避免重复传大视频。
let uploadedVideo: { file: File; url: string } | null = null
let uploadedCover: { file: File; url: string } | null = null

function showError(message: string) {
  uploadError.value = message
  toast.error(message)
}

const videoInput = ref<HTMLInputElement | null>(null)
const coverInput = ref<HTMLInputElement | null>(null)

const publishForm = reactive({
  title: '',
  description: '',
  video: null as File | null,
  cover: null as File | null,
})

const preview = reactive({
  videoUrl: '',
  coverUrl: '',
})

function setPreviewVideo(file: File | null) {
  if (preview.videoUrl) URL.revokeObjectURL(preview.videoUrl)
  preview.videoUrl = file ? URL.createObjectURL(file) : ''
}

function setPreviewCover(file: File | null) {
  if (preview.coverUrl) URL.revokeObjectURL(preview.coverUrl)
  preview.coverUrl = file ? URL.createObjectURL(file) : ''
}

watch(() => publishForm.video, (f) => { setPreviewVideo(f); uploadedVideo = null })
watch(() => publishForm.cover, (f) => { setPreviewCover(f); uploadedCover = null })

onUnmounted(() => {
  setPreviewVideo(null)
  setPreviewCover(null)
})

function pickVideo(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  if (!/\.(mp4|mov|webm|mkv)$/i.test(file.name)) {
    showError('请选择 MP4、MOV、WebM 或 MKV 视频')
    input.value = ''
    return
  }
  if (file.size > MAX_VIDEO_SIZE_MB * 1024 * 1024) {
    showError(`视频超过 ${MAX_VIDEO_SIZE_MB} MB，请压缩后上传`)
    input.value = ''
    return
  }
  publishForm.video = file
  uploadError.value = ''
}

function pickCover(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  if (!/\.(jpe?g|png|webp)$/i.test(file.name)) {
    showError('请选择 JPG、PNG 或 WebP 图片；HEIC 照片请先转换为 JPG')
    input.value = ''
    return
  }
  if (file.size > 10 * 1024 * 1024) {
    showError('封面超过 10 MB，请压缩后上传')
    input.value = ''
    return
  }
  publishForm.cover = file
  uploadError.value = ''
}

function openVideoPicker() {
  videoInput.value?.click()
}

function openCoverPicker() {
  coverInput.value?.click()
}

function clearVideo() {
  publishForm.video = null
  if (videoInput.value) videoInput.value.value = ''
}

function clearCover() {
  publishForm.cover = null
  if (coverInput.value) coverInput.value.value = ''
}

async function onPublish() {
  if (busy.value) return
  if (!auth.isLoggedIn) {
    toast.error('请先登录')
    await router.push('/account')
    return
  }

  const title = publishForm.title.trim()
  const description = publishForm.description.trim()
  if (!title) {
    showError('请输入标题')
    return
  }
  if (!publishForm.video) {
    showError('请选择视频文件（MP4、MOV、WebM 或 MKV）')
    return
  }
  if (!publishForm.cover) {
    showError('请选择封面图片（JPG、PNG 或 WebP）')
    return
  }

  busy.value = true
  uploadError.value = ''
  needsLogin.value = false
  published.value = null
  try {
    stage.value = '上传视频'
    uploadPercent.value = 0
    if (uploadedVideo?.file !== publishForm.video) {
      const videoRes = await videoApi.uploadVideo(publishForm.video, (percent) => { uploadPercent.value = percent })
      const url = videoRes.url || videoRes.play_url || ''
      if (!url) throw new Error('视频上传成功但未返回文件地址，请重试')
      uploadedVideo = { file: publishForm.video, url }
    }

    stage.value = '上传封面'
    uploadPercent.value = 0
    if (uploadedCover?.file !== publishForm.cover) {
      const coverRes = await videoApi.uploadCover(publishForm.cover, (percent) => { uploadPercent.value = percent })
      const url = coverRes.url || coverRes.cover_url || ''
      if (!url) throw new Error('封面上传成功但未返回文件地址，请重试')
      uploadedCover = { file: publishForm.cover, url }
    }

    stage.value = '发布视频'
    const res = await videoApi.publishVideo({
      title,
      description,
      play_url: uploadedVideo.url,
      cover_url: uploadedCover.url,
    })

    published.value = res
    toast.success('已发布')

    publishForm.title = ''
    publishForm.description = ''
    clearVideo()
    clearCover()
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e)
    needsLogin.value = e instanceof ApiError && e.status === 401
    showError(`${stage.value}失败：${msg}`)
  } finally {
    busy.value = false
    stage.value = ''
  }
}
</script>

<template>
  <AppShell>
    <div class="publish-wrap">
      <div class="card publish-card">
        <div class="row" style="justify-content: space-between; align-items: baseline">
          <p class="title" style="margin: 0">发布视频</p>
          <div v-if="busy" class="pill">进行中：{{ stage || '…' }}</div>
        </div>

        <div class="form-grid">
          <label>
            <span class="subtle">标题</span>
            <input v-model="publishForm.title" type="text" placeholder="给你的视频起个名字" :disabled="busy" />
          </label>
          <label>
            <span class="subtle">描述</span>
            <textarea
              v-model="publishForm.description"
              rows="3"
              placeholder="可选，写点介绍"
              :disabled="busy"
            />
          </label>

          <div class="grid two">
            <div>
              <input
                ref="videoInput"
                class="file-native"
                type="file"
                accept="video/*,.mp4,.mov,.webm,.mkv"
                :disabled="busy"
                @change="pickVideo"
              />
              <div class="file-box">
                <button type="button" :disabled="busy" @click="openVideoPicker">选择视频</button>
                <div class="file-name" :class="publishForm.video ? '' : 'muted'">
                  {{ publishForm.video ? publishForm.video.name : '未选择文件' }}
                </div>
                <button v-if="publishForm.video" type="button" :disabled="busy" @click="clearVideo">清除</button>
              </div>
              <div v-if="publishForm.video" class="subtle" style="margin-top: 6px">
                {{ (publishForm.video.size / 1024 / 1024).toFixed(1) }} MB
              </div>
              <p class="subtle">最大 {{ MAX_VIDEO_SIZE_MB }} MB；推荐 MP4（H.264），便于 iPhone 和 Android 播放。</p>
            </div>

            <div>
              <input
                ref="coverInput"
                class="file-native"
                type="file"
                accept="image/jpeg,image/png,image/webp"
                :disabled="busy"
                @change="pickCover"
              />
              <div class="file-box">
                <button type="button" :disabled="busy" @click="openCoverPicker">选择封面</button>
                <div class="file-name" :class="publishForm.cover ? '' : 'muted'">
                  {{ publishForm.cover ? publishForm.cover.name : '未选择文件' }}
                </div>
                <button v-if="publishForm.cover" type="button" :disabled="busy" @click="clearCover">清除</button>
              </div>
              <div v-if="publishForm.cover" class="subtle" style="margin-top: 6px">
                {{ (publishForm.cover.size / 1024 / 1024).toFixed(1) }} MB
              </div>
            </div>
          </div>

          <div v-if="preview.coverUrl || preview.videoUrl" class="grid two">
            <div v-if="preview.videoUrl" class="preview-card">
              <div class="subtle">视频预览</div>
              <video class="video" :src="preview.videoUrl" controls playsinline webkit-playsinline preload="metadata" />
            </div>
            <div v-if="preview.coverUrl" class="preview-card">
              <div class="subtle">封面预览</div>
              <img class="cover" :src="preview.coverUrl" alt="cover preview" />
            </div>
          </div>

          <div v-if="uploadError" class="upload-error" role="alert">
            <div>{{ uploadError }}</div>
            <RouterLink v-if="needsLogin" to="/account?redirect=/video">重新登录</RouterLink>
            <div v-else class="subtle">请检查后再次点击发布；当前页面会保留已选文件。</div>
          </div>
          <div v-if="busy" class="upload-status" role="status" aria-live="polite">
            <div>{{ stage }}{{ stage === '发布视频' ? '…' : ` · ${uploadPercent}%` }}</div>
            <progress v-if="stage !== '发布视频'" :value="uploadPercent" max="100" :aria-label="stage" />
            <div class="subtle">{{ uploadPercent === 100 || stage === '发布视频' ? '正在等待服务器确认，请稍候。' : '请保持页面打开，上传完成后会提示。' }}</div>
          </div>
          <div class="row" style="justify-content: flex-end; margin-top: 8px">
            <button class="primary big-btn" type="button" :disabled="busy" @click="onPublish">发布</button>
          </div>
        </div>

        <div v-if="published" class="card" style="margin-top: 14px">
          <p class="title">已发布</p>
          <div class="row" style="justify-content: space-between">
            <div>
              <div class="title" style="margin: 0">{{ published.title }}</div>
              <div class="subtle mono">#{{ published.id }}</div>
            </div>
            <div class="row">
              <RouterLink class="pill" :to="`/video/${published.id}`">去播放</RouterLink>
              <a class="pill mono" :href="published.play_url" target="_blank" rel="noreferrer">play_url</a>
              <a class="pill mono" :href="published.cover_url" target="_blank" rel="noreferrer">cover_url</a>
            </div>
          </div>
        </div>
      </div>
    </div>
  </AppShell>
</template>

<style scoped>
.upload-error { display: grid; gap: 8px; padding: 12px; border: 1px solid #ef6666; border-radius: 10px; overflow-wrap: anywhere; }
.upload-status { display: grid; gap: 8px; }
.upload-status progress { width: 100%; height: 8px; accent-color: var(--primary); }
.publish-wrap {
  display: flex;
  justify-content: center;
}

.publish-card {
  width: min(980px, 100%);
  padding: 22px;
  box-sizing: border-box;
}

/* 表单整体：上下排列，每个 label 一行 */
.form-grid {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

/* 视频 + 封面一行两列 */
.form-grid .grid.two {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 20px;
}

.form-grid .grid.two > * {
  min-width: 0;
}

.form-grid input[type='file'],
.form-grid textarea,
.form-grid input[type='text'] {
  width: 100%;
  box-sizing: border-box;
}

.file-native {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
}

.file-box {
  display: flex;
  gap: 10px;
  align-items: center;
  flex-wrap: wrap;
  margin-top: 8px;
}

.file-name {
  font-family: var(--font-mono, monospace);
  font-size: 13px;
  color: var(--text, #e7e7ea);
}

.file-name.muted {
  color: var(--text-subtle, #8a8a93);
}

.preview-card {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.preview-card .video,
.preview-card .cover {
  width: 100%;
  max-height: 360px;
  object-fit: contain;
  background: #000;
  border-radius: 8px;
}

.row {
  display: flex;
  gap: 8px;
  align-items: center;
}

button.ghost {
  background: transparent;
  border: 1px solid var(--border, rgba(255, 255, 255, 0.18));
  color: var(--text, #e7e7ea);
  padding: 6px 12px;
  border-radius: 8px;
  cursor: pointer;
}

button.primary {
  background: linear-gradient(135deg, #fe2c55, #ff5a7a);
  color: #fff;
  border: 0;
  padding: 8px 16px;
  border-radius: 8px;
  cursor: pointer;
}

button.primary[disabled] {
  opacity: 0.5;
  cursor: not-allowed;
}

button.big-btn {
  padding: 10px 22px;
  font-weight: 600;
}

.pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  border-radius: 999px;
  border: 1px solid var(--border, rgba(255, 255, 255, 0.18));
  text-decoration: none;
  color: inherit;
  font-size: 13px;
}

.title {
  font-size: 18px;
  font-weight: 600;
}

.subtle {
  color: var(--text-subtle, #8a8a93);
  font-size: 13px;
}

.mono {
  font-family: var(--font-mono, monospace);
}

input[type='text'],
textarea {
  width: 100%;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid var(--border, rgba(255, 255, 255, 0.12));
  color: inherit;
  border-radius: 8px;
  padding: 8px 10px;
  font-size: 14px;
  box-sizing: border-box;
}

input[type='text']:focus,
textarea:focus {
  outline: none;
  border-color: rgba(254, 44, 85, 0.6);
}
</style>
