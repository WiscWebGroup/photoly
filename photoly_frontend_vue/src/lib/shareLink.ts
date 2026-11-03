/** Resolve the same API address used by this browser, including reverse-proxy prefixes. */
export function shareBase(base: string | undefined, origin: string): string {
  return new URL(base || '/api', origin).href.replace(/\/$/, '')
}

export async function copyText(text: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text)
    return true
  } catch {
    const previous = document.activeElement as HTMLElement | null
    const input = document.createElement('textarea')
    input.value = text
    input.style.cssText = 'position:fixed;top:0;left:-9999px'
    document.body.append(input)
    input.select()
    try {
      return document.execCommand('copy')
    } catch {
      return false
    } finally {
      input.remove()
      previous?.focus({ preventScroll: true })
    }
  }
}
