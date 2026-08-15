import { postJson } from './client'
import { normalizeCommentList } from './normalize'
import type { Comment, MessageResponse } from './types'

// 后端 comment_handler.go GetAll 返回 {comments: [...]}，
// 不是裸数组——跟 social/getAllFollowers 的 {followers: [...]} 一个套路。
// 之前直接断言成 Comment[] 会让 normalizeCommentList 拿到 {comments: [...]}，
// Array.isArray = false → 永远返回空数组 → 评论区"暂无"。
export async function listAll(videoId: number) {
  const res = await postJson<{ comments: Comment[] | null }>(
    '/comment/listAll',
    { video_id: videoId },
  )
  return normalizeCommentList(res.comments)
}

export function publish(videoId: number, content: string) {
  // 后端 comment_handler.Publish 返回刚插入的 *Comment 完整对象，不是 MessageResponse。
  // 改类型为 Comment 之后，前端 publishComment 可以直接拿返回值 append 到列表，
  // 不用再发一次 listAll 重拉整个列表——发评论那一下只一次 API 调用。
  return postJson<Comment>('/comment/publish', { video_id: videoId, content }, { authRequired: true })
}

export function remove(commentId: number) {
  return postJson<MessageResponse>('/comment/delete', { comment_id: commentId }, { authRequired: true })
}
