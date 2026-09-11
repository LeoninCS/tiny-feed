import { onBeforeUnmount, onMounted } from 'vue'

// VisualViewport also shrinks for the software keyboard, unlike dvh on iOS.
export function useViewport() {
  let frame = 0
  const viewport = window.visualViewport

  function update() {
    if (frame) return
    frame = requestAnimationFrame(() => {
      frame = 0
      // Keep browser pinch-to-zoom available without resizing the layout beneath it.
      if (viewport && Math.abs(viewport.scale - 1) > 0.01) return
      const root = document.documentElement
      root.style.setProperty('--app-height', `${viewport?.height ?? window.innerHeight}px`)
      root.style.setProperty('--viewport-top', `${viewport?.offsetTop ?? 0}px`)
    })
  }

  onMounted(() => {
    update()
    window.addEventListener('resize', update)
    viewport?.addEventListener('resize', update)
    viewport?.addEventListener('scroll', update)
  })

  onBeforeUnmount(() => {
    cancelAnimationFrame(frame)
    window.removeEventListener('resize', update)
    viewport?.removeEventListener('resize', update)
    viewport?.removeEventListener('scroll', update)
    document.documentElement.style.removeProperty('--app-height')
    document.documentElement.style.removeProperty('--viewport-top')
  })
}
