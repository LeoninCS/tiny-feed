import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useToastStore } from '../stores/toast'

export function useVideoPlayer(scrollerRef: ReturnType<typeof ref<HTMLDivElement | null>>) {
  const toast = useToastStore()
  const muted = ref(false)
  const activeIndex = ref(0)
  const blockedId = ref<number | null>(null)
  const videoMap = new Map<number, HTMLVideoElement>()
  let currentId: number | undefined
  let observer: ResizeObserver | undefined

  function getScrollerHeight() {
    return scrollerRef.value?.clientHeight ?? 0
  }

  function setVideoRef(id: number, el: HTMLVideoElement | null) {
    if (el) {
      el.muted = muted.value
      videoMap.set(id, el)
    } else {
      videoMap.get(id)?.pause()
      videoMap.delete(id)
    }
  }

  function scrollToIndex(idx: number, totalItems: number) {
    const el = scrollerRef.value
    if (!el) return
    const h = getScrollerHeight()
    if (!h) return
    const next = Math.max(0, Math.min(idx, Math.max(0, totalItems - 1)))
    el.scrollTo({ top: next * h, behavior: matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth' })
  }

  let scrollRaf = 0
  function onScroll() {
    if (!scrollerRef.value) return
    if (scrollRaf) return
    scrollRaf = window.requestAnimationFrame(() => {
      scrollRaf = 0
      const el = scrollerRef.value
      if (!el) return
      const h = el.clientHeight
      if (!h) return
      const idx = Math.round(el.scrollTop / h)
      if (idx !== activeIndex.value) activeIndex.value = idx
    })
  }

  async function playActive(activeItemId: number | undefined) {
    currentId = activeItemId
    blockedId.value = null
    for (const [id, v] of videoMap.entries()) {
      if (id === activeItemId) continue
      v.pause()
    }
    if (!activeItemId || document.hidden) return
    const video = videoMap.get(activeItemId)
    if (!video) return
    video.muted = muted.value
    try {
      await video.play()
      if (currentId !== activeItemId) video.pause()
    } catch {
      if (currentId === activeItemId) blockedId.value = activeItemId
    }
  }

  function toggleMute() {
    muted.value = !muted.value
    for (const v of videoMap.values()) v.muted = muted.value
    toast.info(muted.value ? '已静音' : '已取消静音')
  }

  function togglePlayPause(activeItemId: number | undefined) {
    if (!activeItemId) return
    const video = videoMap.get(activeItemId)
    if (!video) return
    if (video.paused) void playActive(activeItemId)
    else video.pause()
  }

  function onVisibilityChange() {
    if (document.hidden) {
      for (const video of videoMap.values()) video.pause()
      blockedId.value = currentId ?? null
    }
  }

  onMounted(() => {
    if (scrollerRef.value && typeof ResizeObserver !== 'undefined') {
      observer = new ResizeObserver(() => {
        const el = scrollerRef.value
        if (el) el.scrollTo({ top: activeIndex.value * el.clientHeight, behavior: 'instant' })
      })
      observer.observe(scrollerRef.value)
    }
    document.addEventListener('visibilitychange', onVisibilityChange)
  })

  onBeforeUnmount(() => {
    currentId = undefined
    cancelAnimationFrame(scrollRaf)
    observer?.disconnect()
    document.removeEventListener('visibilitychange', onVisibilityChange)
    for (const video of videoMap.values()) video.pause()
    videoMap.clear()
  })

  return { muted, activeIndex, blockedId, videoMap, setVideoRef, scrollToIndex, onScroll, playActive, toggleMute, togglePlayPause }
}
