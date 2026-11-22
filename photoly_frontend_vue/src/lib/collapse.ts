import { animate, type AnimationPlaybackControls } from 'motion'
import type { Directive } from 'vue'

const ease = [0.165, 0.84, 0.44, 1] as const
const still = () => !!window.matchMedia?.('(prefers-reduced-motion: reduce)').matches
const running = new WeakMap<Element, AnimationPlaybackControls>()

function slide(el: Element, opening: boolean, done: () => void) {
  running.get(el)?.stop()
  if (still()) return done()
  const box = el as HTMLElement
  const full = box.scrollHeight
  box.style.overflow = 'hidden'
  const run = animate(box, { height: opening ? [0, full] : [full, 0] }, { duration: 0.3, ease })
  running.set(el, run)
  void run.finished.then(() => {
    if (running.get(el) !== run) return // superseded by a quicker toggle
    box.style.removeProperty('height')
    box.style.removeProperty('overflow')
    done()
  })
}

/** For a `v-show` panel outside a <details>: `<Transition v-bind="collapseTransition">`. Same motion as v-collapse. */
export const collapseTransition = {
  css: false,
  onEnter: (el: Element, done: () => void) => slide(el, true, done),
  onLeave: (el: Element, done: () => void) => slide(el, false, done),
}

/** `v-collapse` on a <details>: animates its height when the <summary> toggles it. */
export const vCollapse: Directive<HTMLDetailsElement> = {
  mounted(el) {
    let anim: AnimationPlaybackControls | undefined
    el.querySelector(':scope > summary')?.addEventListener('click', (e) => {
      e.preventDefault()
      if (still() || !el.offsetHeight) {
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
      const run = (anim = animate(el, { height: [start, end] }, { duration: 0.3, ease }))
      void run.finished.then(() => {
        if (anim !== run) return // superseded by another click
        if (!opening) el.open = false
        el.style.removeProperty('height')
        el.style.removeProperty('overflow')
      })
    })
  },
}
