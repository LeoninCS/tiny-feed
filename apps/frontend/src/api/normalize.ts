import type { Account, Comment, FeedAuthor, FeedVideoItem, Video } from './types'

export function listOrEmpty<T>(value: T[] | null | undefined): T[] {
  return Array.isArray(value) ? value : []
}

export function normalizeAccount(value: Account | null | undefined): Account {
  // 保留 avatar_url / bio——后端 findByID / getProfile 会带过来，
  // 之前这里只回填 id + username，把头像 URL 丢了，导致所有 UserAvatar
  // 永远只走 fallback（首字母 + 渐变背景），即使后端有真实头像也显示不出来。
  return {
    id: Number(value?.id ?? 0),
    username: value?.username || '匿名用户',
    avatar_url: typeof value?.avatar_url === 'string' ? value.avatar_url : '',
    bio: typeof value?.bio === 'string' ? value.bio : '',
  }
}

function normalizeAuthor(value: FeedAuthor | null | undefined): FeedAuthor {
  return {
    id: Number(value?.id ?? 0),
    username: value?.username || '匿名用户',
    avatar_url: typeof value?.avatar_url === 'string' ? value.avatar_url : '',
  }
}

export function normalizeFeedVideoItem(value: FeedVideoItem): FeedVideoItem {
  return {
    ...value,
    author: normalizeAuthor(value.author),
    title: value.title || '未命名视频',
    description: value.description || '',
    play_url: value.play_url || '',
    cover_url: value.cover_url || '',
    create_time: Number(value.create_time ?? 0),
    likes_count: Number(value.likes_count ?? 0),
    is_liked: Boolean(value.is_liked),
  }
}

export function normalizeFeedVideoList(value: FeedVideoItem[] | null | undefined): FeedVideoItem[] {
  return listOrEmpty(value).map(normalizeFeedVideoItem)
}

export function normalizeVideoList(value: Video[] | null | undefined): Video[] {
  return listOrEmpty(value).map((video) => ({
    ...video,
    username: video.username || '匿名用户',
    title: video.title || '未命名视频',
    description: video.description || '',
    play_url: video.play_url || '',
    cover_url: video.cover_url || '',
    likes_count: Number(video.likes_count ?? 0),
    avatar_url: typeof video.avatar_url === 'string' ? video.avatar_url : '',
  }))
}

export function normalizeCommentList(value: Comment[] | null | undefined): Comment[] {
  return listOrEmpty(value).map((comment) => ({
    ...comment,
    username: comment.username || '匿名用户',
    content: comment.content || '',
  }))
}
