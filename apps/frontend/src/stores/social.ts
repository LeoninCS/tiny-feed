import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import { ApiError } from '../api/client'
import type { Account } from '../api/types'
import * as socialApi from '../api/social'
import { useAuthStore } from './auth'

export const useSocialStore = defineStore('social', () => {
  const auth = useAuthStore()

  const followers = ref<Account[]>([])
  const vloggers = ref<Account[]>([])

  const followersLoading = ref(false)
  const vloggersLoading = ref(false)

  const followersError = ref('')
  const vloggersError = ref('')

  const followerCount = computed(() => followers.value.length)
  const followingCount = computed(() => vloggers.value.length)

  function clear() {
    followers.value = []
    vloggers.value = []
    followersError.value = ''
    vloggersError.value = ''
    followersLoading.value = false
    vloggersLoading.value = false
  }

  function isFollowing(accountId: number) {
    return vloggers.value.some((a) => a.id === accountId)
  }

  // 当前登录用户自己的 account_id。"我关注的 / 关注我的" 列表都按这个 ID 查。
  function myId(): number {
    return auth.claims?.account_id ?? 0
  }

  async function refreshFollowers(vloggerId: number) {
    if (!auth.isLoggedIn) {
      clear()
      return
    }
    if (!Number.isFinite(vloggerId) || vloggerId <= 0) {
      followersError.value = '无效的用户 id'
      followers.value = []
      return
    }

    followersLoading.value = true
    followersError.value = ''
    try {
      const res = await socialApi.getAllFollowers(vloggerId)
      followers.value = res.followers
    } catch (e) {
      followersError.value = e instanceof ApiError ? e.message : String(e)
      followers.value = []
    } finally {
      followersLoading.value = false
    }
  }

  async function refreshVloggers(followerId: number) {
    if (!auth.isLoggedIn) {
      clear()
      return
    }
    if (!Number.isFinite(followerId) || followerId <= 0) {
      vloggersError.value = '无效的用户 id'
      vloggers.value = []
      return
    }

    vloggersLoading.value = true
    vloggersError.value = ''
    try {
      const res = await socialApi.getAllVloggers(followerId)
      vloggers.value = res.vloggers
    } catch (e) {
      vloggersError.value = e instanceof ApiError ? e.message : String(e)
      vloggers.value = []
    } finally {
      vloggersLoading.value = false
    }
  }

  async function refreshMine() {
    // 刷新"我自己的"粉丝 / 关注列表。用当前登录用户 ID 而不是不传参——
    // 不传参会让后端收到空 body 报 "vlogger_id is required"。
    const id = myId()
    if (id <= 0) {
      clear()
      return
    }
    await Promise.all([refreshFollowers(id), refreshVloggers(id)])
  }

  async function follow(vloggerId: number) {
    if (!auth.isLoggedIn) throw new ApiError('需要先登录', 401)
    await socialApi.follow(vloggerId)
    // 关注成功后刷新"我关注的"列表。
    const id = myId()
    if (id > 0) await refreshVloggers(id)
  }

  async function unfollow(vloggerId: number) {
    if (!auth.isLoggedIn) throw new ApiError('需要先登录', 401)
    await socialApi.unfollow(vloggerId)
    const id = myId()
    if (id > 0) await refreshVloggers(id)
  }

  return {
    followers,
    vloggers,
    followerCount,
    followingCount,
    followersLoading,
    vloggersLoading,
    followersError,
    vloggersError,
    clear,
    isFollowing,
    refreshMine,
    refreshFollowers,
    refreshVloggers,
    follow,
    unfollow,
  }
})

