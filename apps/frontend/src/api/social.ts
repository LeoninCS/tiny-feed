import { postJson } from './client'
import { listOrEmpty, normalizeAccount } from './normalize'
import type { GetAllFollowersResponse, GetAllVloggersResponse, MessageResponse } from './types'

export function follow(vloggerId: number) {
  return postJson<MessageResponse>('/social/follow', { vlogger_id: vloggerId }, { authRequired: true })
}

export function unfollow(vloggerId: number) {
  return postJson<MessageResponse>('/social/unfollow', { vlogger_id: vloggerId }, { authRequired: true })
}

// 后端这两个端点是公开的（见 http_router.go：/social/getAllFollowers
// 和 /social/getAllVloggers 都在 public group），所以不强制要求 token。
// 入参必填，调用方负责传入合法 ID——传 0 / undefined 进来直接抛错，
// 避免出现 `{vlogger_id:0}` 触发后端 "X is required"。
export async function getAllFollowers(vloggerId: number) {
  if (!Number.isFinite(vloggerId) || vloggerId <= 0) {
    throw new Error('getAllFollowers: vloggerId 必须 > 0')
  }
  const res = await postJson<GetAllFollowersResponse>(
    '/social/getAllFollowers',
    { vlogger_id: vloggerId },
  )
  return { ...res, followers: listOrEmpty(res.followers).map(normalizeAccount) }
}

export async function getAllVloggers(followerId: number) {
  if (!Number.isFinite(followerId) || followerId <= 0) {
    throw new Error('getAllVloggers: followerId 必须 > 0')
  }
  const res = await postJson<GetAllVloggersResponse>(
    '/social/getAllVloggers',
    { follower_id: followerId },
  )
  return { ...res, vloggers: listOrEmpty(res.vloggers).map(normalizeAccount) }
}
