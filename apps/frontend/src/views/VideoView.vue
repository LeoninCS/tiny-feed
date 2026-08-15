<script setup lang="ts">
import { onUnmounted, reactive, ref, watch } from 'vue'
import { RouterLink, useRouter } from 'vue-router'

import AppShell from '../components/AppShell.vue'
import { ApiError } from '../api/client'
import * as videoApi from '../api/video'
import type { Video } from '../api/types'
import { useAuthStore } from '../stores/auth'
import { useToastStore } from '../stores/toast'

const router = useRouter()
const auth = useAuthStore()
const toast = useToastStore()

const busy = ref(false)
const stage = ref('')
const published = ref<Video | null>(null)

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

watch(() => publishForm.video, (f) => setPreviewVideo(f))
watch(() => publishForm.cover, (f) => setPreviewCover(f))

onUnmounted(() => {
  setPreviewVideo(null)
  setPreviewCover(null)
})

function pickVideo(e: Event) {
  const input = e.target as HTMLInputElement
  publishForm.video = input.files?.[0] ?? null
}

function pickCover(e: Event) {
  const input = e.target as HTMLInputElement
  publishForm.cover = input.files?.[0] ?? null
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
    toast.error('请输入 title')
    return
  }
  if (!publishForm.video) {
    toast.error('请选择视频文件（.mp4）')
    return
  }
  if (!publishForm.cover) {
    toast.error('请选择封面图片（jpg/png/webp）')
    return
  }

  busy.value = true
  published.value = null
  try {
    stage.value = '上传视频'
    const videoRes = await videoApi.uploadVideo(publishForm.video)

    stage.value = '上传封面'
    const coverRes = await videoApi.uploadCover(publishForm.cover)

    const playUrl = videoRes.url || videoRes.play_url || ''
    const coverUrl = coverRes.url || coverRes.cover_url || ''
    if (!coverUrl || !playUrl) {
      toast.error('上传成功但缺少 url')
      return
    }

    stage.value = '发布视频'
    const res = await videoApi.publishVideo({
      title,
      description,
      play_url: playUrl,
      cover_url: coverUrl,
    })

    published.value = res
    toast.success('已发布')

    publishForm.title = ''
    publishForm.description = ''
    clearVideo()
    clearCover()
  } catch (e) {
    const msg = e instanceof ApiError ? e.message : String(e)
    toast.error(msg)
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
                accept="video/mp4,video/*"
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
                已选择：{{ publishForm.video.name }}
              </div>
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
                已选择：{{ publishForm.cover.name }}
              </div>
            </div>
          </div>

          <div v-if="preview.coverUrl || preview.videoUrl" class="grid two">
            <div v-if="preview.videoUrl" class="preview-card">
              <div class="subtle">视频预览</div>
              <video class="video" :src="preview.videoUrl" controls playsinline preload="metadata" />
            </div>
            <div v-if="preview.coverUrl" class="preview-card">
              <div class="subtle">封面预览</div>
              <img class="cover" :src="preview.coverUrl" alt="cover preview" />
            </div>
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
