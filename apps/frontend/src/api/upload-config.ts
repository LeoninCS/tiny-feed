// HTTPS 分离部署时填写独立上传入口；整站 HTTP 直连构建使用 /api，同源上传。
export const UPLOAD_API_BASE = (import.meta.env.VITE_UPLOAD_API_BASE as string | undefined)?.replace(/\/+$/, '') || undefined
export const MAX_VIDEO_SIZE_MB = UPLOAD_API_BASE ? 300 : 95
