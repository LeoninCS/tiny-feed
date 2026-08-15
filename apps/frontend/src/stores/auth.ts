import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import { decodeJwtPayload, type JwtPayload } from '../utils/jwt'

const ACCESS_KEY = 'access_token'
const REFRESH_KEY = 'refresh_token'
const AVATAR_KEY = 'avatar_url'

function readStored(key: string): string | null {
  try { return localStorage.getItem(key) } catch { return null }
}

function writeStored(key: string, value: string) {
  try { localStorage.setItem(key, value) } catch {}
}

function removeStored(key: string) {
  try { localStorage.removeItem(key) } catch {}
}

export const useAuthStore = defineStore('auth', () => {
  const token = ref<string | null>(readStored(ACCESS_KEY))
  const refreshToken = ref<string | null>(readStored(REFRESH_KEY))
  // 头像 URL 单独存到 localStorage，JWT claims 里没有这个字段。
  // 登出/清 token 时一并清掉。
  const avatarUrl = ref<string | null>(readStored(AVATAR_KEY))

  const isLoggedIn = computed(() => !!token.value)
  const claims = computed<JwtPayload | null>(() => (token.value ? decodeJwtPayload(token.value) : null))

  function setToken(newToken: string) {
    token.value = newToken
    writeStored(ACCESS_KEY, newToken)
  }

  function setTokens(access: string, refresh: string) {
    token.value = access
    refreshToken.value = refresh
    writeStored(ACCESS_KEY, access)
    writeStored(REFRESH_KEY, refresh)
  }

  // 修改头像后调用，把新的 URL 持久化。
  function setAvatarUrl(url: string) {
    avatarUrl.value = url
    writeStored(AVATAR_KEY, url)
  }

  function clearTokens() {
    token.value = null
    refreshToken.value = null
    avatarUrl.value = null
    removeStored(ACCESS_KEY)
    removeStored(REFRESH_KEY)
    removeStored(AVATAR_KEY)
  }

  return { token, refreshToken, avatarUrl, isLoggedIn, claims, setToken, setTokens, setAvatarUrl, clearTokens }
})
