import { postForm, postJson } from './client'
import { normalizeVideoList } from './normalize'
import type { Video } from './types'

// 发布一条视频（业务数据：title/description + 已上传文件的 URL）。
export function publishVideo(input: { title: string; description: string; play_url: string; cover_url: string }) {
  return postJson<Video>('/video/publish', input, { authRequired: true })
}

// 单文件上传响应：返回 URL 路径（前端拼 /static 用）。
export type UploadResponse = { url: string; play_url?: string; cover_url?: string }

// 上传视频文件，存到后端 .run/uploads/videos/，返回 /static/videos/xxx。
export function uploadVideo(file: File) {
  const fd = new FormData()
  fd.append('file', file)
  return postForm<UploadResponse>('/video/uploadVideo', fd, { authRequired: true })
}

// 上传封面图片，存到后端 .run/uploads/covers/，返回 /static/covers/xxx。
export function uploadCover(file: File) {
  const fd = new FormData()
  fd.append('file', file)
  return postForm<UploadResponse>('/video/uploadCover', fd, { authRequired: true })
}

export async function listByAuthorId(authorId: number) {
  const videos = await postJson<Video[] | null>('/video/listByAuthorID', { author_id: authorId })
  return normalizeVideoList(videos)
}

export function getDetail(id: number) {
  return postJson<Video>('/video/getDetail', { id })
}
