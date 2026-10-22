import type { PickedFile } from '@/stores/uploads'

/** Directory readers return multiple batches; consume every batch, including >100 children. */
export async function droppedFiles(transfer: DataTransfer): Promise<PickedFile[]> {
  async function walk(entry: FileSystemEntry, prefix = ''): Promise<PickedFile[]> {
    const path = prefix + entry.name
    if (entry.isFile) {
      const file = await new Promise<File>((resolve, reject) =>
        (entry as FileSystemFileEntry).file(resolve, reject),
      )
      return [{ file, path }]
    }
    const reader = (entry as FileSystemDirectoryEntry).createReader()
    const result: PickedFile[] = []
    while (true) {
      const entries = await new Promise<FileSystemEntry[]>((resolve, reject) =>
        reader.readEntries(resolve, reject),
      )
      if (!entries.length) break
      for (const child of entries)
        for (const item of await walk(child, path + '/')) result.push(item)
    }
    return result
  }
  const entries = [...transfer.items]
    .map((item) => item.webkitGetAsEntry?.())
    .filter((e): e is FileSystemEntry => !!e)
  if (entries.length) {
    const result: PickedFile[] = []
    for (const entry of entries) for (const item of await walk(entry)) result.push(item)
    return result
  }
  return [...transfer.files].map((file) => ({ file, path: file.webkitRelativePath || file.name }))
}
