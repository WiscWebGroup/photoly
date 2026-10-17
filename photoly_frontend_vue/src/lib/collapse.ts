import { animate, type AnimationPlaybackControls } from 'motion'
import type { Directive } from 'vue'

/** `v-collapse` on a <details>: animates its height when the <summary> toggles it. */
export const vCollapse: Directive<HTMLDetailsElement> = {
  mounted(el) {
    let anim: AnimationPlaybackControls | undefined
    el.querySelector(':scope > summary')?.addEventListener('click', (e) => {
      e.preventDefault()
      if (window.matchMedia?.('(prefers-reduced-motion: reduce)').matches || !el.offsetHeight) {
        el.open = !el.open
        return
      }
      anim?.stop()
      const opening = !el.open
      const start = el.offsetHeight
      const summary = el.querySelector<HTMLElement>(':scope > summary')!
      const borders = el.offsetHeight - el.clientHeight
      if (opening) el.open = true
      const end = opening ? el.scrollHeight + borders : summary.offsetHeight + borders
      el.style.overflow = 'hidden'
      const run = (anim = animate(
        el,
        { height: [start, end] },
        { duration: 0.3, ease: [0.165, 0.84, 0.44, 1] },
      ))
      void run.finished.then(() => {
        if (anim !== run) return // superseded by another click
        if (!opening) el.open = false
        el.style.removeProperty('height')
        el.style.removeProperty('overflow')
      })
    })
  },
}
