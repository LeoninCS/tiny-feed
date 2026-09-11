import { postForm, postJson } from './client'
import { normalizeVideoList } from './normalize'
import type { Video } from './types'
import { UPLOAD_API_BASE } from './upload-config'

// 发布一条视频（业务数据：title/description + 已上传文件的 URL）。
export function publishVideo(input: { title: string; description: string; play_url: string; cover_url: string }) {
  return postJson<Video>('/video/publish', input, { authRequired: true })
}

// 单文件上传响应：返回 URL 路径（前端拼 /static 用）。
export type UploadResponse = { url: string; play_url?: string; cover_url?: string }

function uploadForm(file: File): FormData {
  const types: Record<string, string> = {
    mp4: 'video/mp4', mov: 'video/quicktime', webm: 'video/webm', mkv: 'video/x-matroska',
    jpg: 'image/jpeg', jpeg: 'image/jpeg', png: 'image/png', webp: 'image/webp',
  }
  const extension = file.name.split('.').pop()?.toLowerCase() ?? ''
  // 部分手机文件提供方不提供 MIME，或统一返回 octet-stream。
  // 仅为后端已支持的扩展名补全类型；保留文件内容和原有的明确 MIME。
  const inferredType = types[extension]
  const genericType = !file.type || file.type.toLowerCase() === 'application/octet-stream'
  const payload = genericType && inferredType ? file.slice(0, file.size, inferredType) : file
  const fd = new FormData()
  fd.append('file', payload, file.name)
  return fd
}

// 上传视频文件，存到后端 .run/uploads/videos/，返回 /static/videos/xxx。
export function uploadVideo(file: File, onProgress?: (percent: number) => void) {
  return postForm<UploadResponse>('/video/uploadVideo', uploadForm(file), { authRequired: true, onProgress, baseUrl: UPLOAD_API_BASE })
}

// 上传封面图片，存到后端 .run/uploads/covers/，返回 /static/covers/xxx。
export function uploadCover(file: File, onProgress?: (percent: number) => void) {
  return postForm<UploadResponse>('/video/uploadCover', uploadForm(file), { authRequired: true, onProgress, baseUrl: UPLOAD_API_BASE })
}

export async function listByAuthorId(authorId: number) {
  const videos = await postJson<Video[] | null>('/video/listByAuthorID', { author_id: authorId })
  return normalizeVideoList(videos)
}

export function getDetail(id: number) {
  return postJson<Video>('/video/getDetail', { id })
}
