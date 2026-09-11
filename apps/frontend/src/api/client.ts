import { useAuthStore } from '../stores/auth'
import { reportError } from '../utils/error-reporter'

export class ApiError extends Error {
  status: number
  payload?: unknown

  constructor(message: string, status: number, payload?: unknown) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.payload = payload
  }
}

type ApiErrorBody = { error?: string }

const API_BASE = (import.meta.env.VITE_API_BASE as string | undefined) ?? '/api'

let isRefreshing = false
let refreshPromise: Promise<string | null> | null = null

async function tryRefresh(): Promise<string | null> {
  const auth = useAuthStore()
  if (!auth.refreshToken) return null
  if (isRefreshing) return refreshPromise
  isRefreshing = true
  refreshPromise = (async () => {
    try {
      // 后端 Refresh handler 从 X-Refresh-Token header 读（见 account_handler.go
      // 的 c.GetHeader("X-Refresh-Token")），不是从 body 解析——之前放在 body
      // 里导致永远拿不到 token，refresh 一直失败，token 一过期用户就被清登录、
      // isLiked 之类需要鉴权的查询全部静默 401，UI 看起来"明明点了赞却没显示"。
      const res = await fetch(`${API_BASE}/account/refresh`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-Refresh-Token': auth.refreshToken ?? '',
        },
      })
      if (res.status === 401) { auth.clearTokens(); return null }
      if (!res.ok) throw new ApiError('登录状态验证暂时失败，请稍后重试', res.status)
      const data = await res.json()
      if (!data.token || !data.refresh_token) throw new ApiError('登录续期响应异常，请重新登录', 502)
      // 服务端会同时轮换两个凭证；保留旧 refresh token 会导致下一次续期失败。
      auth.setTokens(data.token, data.refresh_token)
      return data.token as string
    } finally {
      isRefreshing = false
    }
  })()
  return refreshPromise
}

export async function postJson<T>(path: string, body: unknown, options?: { authRequired?: boolean }): Promise<T> {
  const auth = useAuthStore()
  const token = auth.token

  if (options?.authRequired && !token) {
    throw new ApiError('需要先登录（缺少 token）', 401)
  }

  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (token) headers.Authorization = `Bearer ${token}`

  const res = await fetch(`${API_BASE}${path}`, {
    method: 'POST',
    headers,
    body: JSON.stringify(body ?? {}),
  })

  if (res.status === 401 && path !== '/account/refresh') {
    const newToken = await tryRefresh()
    if (newToken) {
      headers.Authorization = `Bearer ${newToken}`
      const retryRes = await fetch(`${API_BASE}${path}`, {
        method: 'POST', headers, body: JSON.stringify(body ?? {}),
      })
      return handleResponse<T>(retryRes, path)
    }
  }

  return handleResponse<T>(res, path)
}

export async function postForm<T>(path: string, body: FormData, options?: { authRequired?: boolean; onProgress?: (percent: number) => void; baseUrl?: string }): Promise<T> {
  const auth = useAuthStore()

  if (options?.authRequired) {
    if (!auth.token) throw new ApiError('登录已失效，请重新登录后上传', 401)
    // 大文件发送前先续期，避免传完整个文件后才发现 access token 已过期。
    const expires = auth.claims?.exp
    if (expires && expires <= Date.now() / 1000 + 60) {
      if (!await tryRefresh()) throw new ApiError('登录已失效，请重新登录后上传', 401)
    }
  }

  const headers: Record<string, string> = {}
  if (auth.token) headers.Authorization = `Bearer ${auth.token}`

  const send = () => new Promise<Response>((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    xhr.open('POST', `${options?.baseUrl ?? API_BASE}${path}`)
    for (const [name, value] of Object.entries(headers)) xhr.setRequestHeader(name, value)
    xhr.upload.onprogress = (event) => {
      if (event.lengthComputable) options?.onProgress?.(Math.round(event.loaded / event.total * 100))
    }
    xhr.onload = () => {
      if (xhr.status < 200) {
        reject(new ApiError('未收到服务器响应，请检查网络后重试', 0))
        return
      }
      resolve(new Response(xhr.responseText || null, { status: xhr.status }))
    }
    xhr.onerror = () => reject(new ApiError('网络连接中断，请检查网络后重试', 0))
    xhr.onabort = () => reject(new ApiError('上传已取消', 0))
    options?.onProgress?.(0)
    xhr.send(body)
  })

  const res = await send()

  if (res.status === 401 && path !== '/account/refresh') {
    const newToken = await tryRefresh()
    if (newToken) {
      headers.Authorization = `Bearer ${newToken}`
      const retryRes = await send()
      return handleResponse<T>(retryRes, path)
    }
  }

  return handleResponse<T>(res, path)
}

async function handleResponse<T>(res: Response, path: string): Promise<T> {
  const auth = useAuthStore()
  const text = await res.text()
  let data: unknown = null
  if (text) {
    try { data = JSON.parse(text) } catch { data = text }
  }

  if (!res.ok) {
    if (res.status === 401) auth.clearTokens()
    const msg = res.status === 401 && path !== '/account/login' ? '登录已失效，请重新登录后重试'
      : res.status === 413 ? '文件超过上传通道限制，请压缩后重试'
      : [408, 504, 524].includes(res.status) ? '上传连接超时，请检查网络后重试'
      : data && typeof data === 'object' && (data as ApiErrorBody).error
      ? String((data as ApiErrorBody).error)
      : `请求失败 (${res.status})`
    const apiErr = new ApiError(msg, res.status, data)
    reportError(apiErr, { path, status: res.status })
    throw apiErr
  }

  return data as T
}
