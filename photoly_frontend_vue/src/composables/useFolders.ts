import { ref } from 'vue'
import { call } from '@/lib/http'

export interface Folder {
  nsId: number
  nsName: string
}

/** The backend rejects these; HomePart used to repeat this check inline. */
export function isValidFolderName(name: string | null) {
  return !!name && name !== 'root' && name !== '/'
}

/** The folder being browsed: its children and breadcrumb path, plus folder mutations. */
export function useFolders() {
  const current = ref<Folder | null>(null)
  const children = ref<Folder[]>([])
  /** Root → current. */
  const path = ref<Folder[]>([])

  async function refresh() {
    if (!current.value) return
    const { nsId } = current.value
    ;[children.value, path.value] = await Promise.all([
      call<Folder[]>({ url: '/namespace/getChildren', params: { parentId: nsId } }),
      call<Folder[]>({ url: '/namespace/trace', params: { nsId } }),
    ])
  }

  async function open(nsId: number, nsName = '') {
    current.value = { nsId, nsName }
    await refresh()
    current.value = path.value.at(-1) ?? current.value
  }

  async function openRoot() {
    const root = await call<Folder>({ url: '/namespace/getRoot' })
    await open(root.nsId, root.nsName)
  }

  async function create(nsName: string) {
    await call({ method: 'post', url: '/namespace/insert', data: { nsParentId: current.value?.nsId, nsName } })
    await refresh()
  }

  async function rename(nsId: number, nsName: string) {
    await call({ method: 'post', url: '/namespace/updateName', data: { nsId, nsName } })
    await refresh()
  }

  async function move(nsId: number, nsParentId: number) {
    await call({ method: 'post', url: '/namespace/updateParent', data: { nsId, nsParentId } })
    await refresh()
  }

  async function remove(nsId: number) {
    await call({ method: 'post', url: '/namespace/delete', params: { nsId } })
    await refresh()
  }

  return { current, children, path, open, openRoot, refresh, create, rename, move, remove }
}
