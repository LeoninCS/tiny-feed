// 独立上传入口只用于视频和封面的文件请求，登录、发布和播放仍使用站点原有地址。
export const UPLOAD_API_BASE = (import.meta.env.VITE_UPLOAD_API_BASE as string | undefined)?.replace(/\/+$/, '') || undefined
export const MAX_VIDEO_SIZE_MB = UPLOAD_API_BASE ? 300 : 95
