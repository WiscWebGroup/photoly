/** In-memory only: no face crops or person metadata survive the browser session. */
export class AvatarCache {
  private entries = new Map<string, { blob: Blob; expires: number }>()
  private pending = new Map<string, Promise<Blob>>()
  private controllers = new Set<AbortController>()
  private queue: (() => void)[] = []
  private active = 0
  private bytes = 0
  private epoch = 0
  constructor(
    private budget = 8 * 1024 * 1024,
    private ttl = 5 * 60_000,
  ) {}
  clear() {
    this.epoch++
    this.entries.clear()
    this.bytes = 0
    this.pending.clear()
    this.controllers.forEach((c) => c.abort())
  }
  private evict(key: string) {
    const entry = this.entries.get(key)
    if (entry) this.bytes -= entry.blob.size
    this.entries.delete(key)
  }
  get(key: string, fetch: (signal: AbortSignal) => Promise<Blob>): Promise<Blob> {
    for (const [id, entry] of this.entries) if (entry.expires <= Date.now()) this.evict(id)
    const cached = this.entries.get(key)
    if (cached) {
      this.entries.delete(key)
      this.entries.set(key, cached)
      return Promise.resolve(cached.blob)
    }
    const pending = this.pending.get(key)
    if (pending) return pending
    const epoch = this.epoch
    const controller = new AbortController()
    this.controllers.add(controller)
    const result = (async () => {
      if (this.active >= 4) await new Promise<void>((resolve) => this.queue.push(resolve))
      else this.active++
      try {
        if (controller.signal.aborted) throw new DOMException('Cancelled', 'AbortError')
        const blob = await fetch(controller.signal)
        if (epoch !== this.epoch) throw new DOMException('Cancelled', 'AbortError')
        if (blob.size <= this.budget) {
          while (this.bytes + blob.size > this.budget) this.evict(this.entries.keys().next().value!)
          this.entries.set(key, { blob, expires: Date.now() + this.ttl })
          this.bytes += blob.size
        }
        return blob
      } finally {
        const next = this.queue.shift()
        if (next) next()
        else this.active--
        this.controllers.delete(controller)
        if (epoch === this.epoch) this.pending.delete(key)
      }
    })()
    this.pending.set(key, result)
    return result
  }
}
