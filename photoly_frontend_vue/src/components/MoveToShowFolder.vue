<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { toast } from 'vue-sonner'
import { ChevronRight, Folder, FolderOpen } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import type { Folder as FolderInfo } from '@/composables/useFolders'
import { call } from '@/lib/http'
import { cn } from '@/lib/utils'

interface TreeNode {
  id: number
  name: string
  disabled: boolean
  expanded: boolean
  /** `null` until the node is first expanded. */
  children: TreeNode[] | null
}

const props = defineProps<{ title?: string; originalNsId?: number; disableOrgNsId?: boolean }>()
const emit = defineEmits<{ moveToUpdate: [nsId: number] }>()

const open = ref(false)
const root = ref<TreeNode>()
const selectedNsId = ref(-1)
const selectedNsName = ref('')

const node = (f: FolderInfo): TreeNode => ({
  id: f.nsId,
  name: f.nsName,
  disabled: !!props.disableOrgNsId && f.nsId === props.originalNsId,
  expanded: false,
  children: null,
})

/** The visible rows, depth first, so a recursive component isn't needed. */
const rows = computed(() => {
  const out: { node: TreeNode; depth: number }[] = []
  const walk = (n: TreeNode, depth: number) => {
    out.push({ node: n, depth })
    if (n.expanded) n.children?.forEach((c) => walk(c, depth + 1))
  }
  if (root.value) walk(root.value, 0)
  return out
})

async function toggle(n: TreeNode) {
  if (n.disabled) return
  selectedNsId.value = n.id
  selectedNsName.value = n.name
  n.expanded = !n.expanded
  if (n.children) return
  try {
    n.children = (
      await call<FolderInfo[]>({ url: '/namespace/getChildren', params: { parentId: n.id } })
    ).map(node)
  } catch {
    n.expanded = false
    toast.warning('Query Folder Error!')
  }
}

function update() {
  emit('moveToUpdate', selectedNsId.value)
  open.value = false
}

onMounted(async () => {
  try {
    root.value = node(await call<FolderInfo>({ url: '/namespace/getRoot' }))
    root.value.name = 'root'
  } catch {
    toast.warning('Load Root Error!')
  }
})

defineExpose({ openModal: () => (open.value = true) })
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent>
      <DialogHeader>
        <DialogTitle>{{ title ?? 'Move to a New Location' }}</DialogTitle>
        <DialogDescription>Currently Selected: {{ selectedNsName }}</DialogDescription>
      </DialogHeader>

      <ul role="tree" class="max-h-72 overflow-auto">
        <li
          v-for="{ node: n, depth } in rows"
          :key="n.id"
          role="treeitem"
          :aria-expanded="n.expanded"
          :aria-selected="n.id === selectedNsId"
        >
          <button
            type="button"
            :disabled="n.disabled"
            :style="{ paddingLeft: `${depth * 1.25 + 0.5}rem` }"
            :class="
              cn(
                'hover:bg-accent flex w-full items-center gap-1.5 rounded-md py-1.5 pr-2 text-left text-sm disabled:opacity-50',
                n.id === selectedNsId && 'bg-accent text-accent-foreground',
              )
            "
            @click="toggle(n)"
          >
            <ChevronRight
              :class="cn('size-4 shrink-0 transition-transform', n.expanded && 'rotate-90')"
            />
            <component :is="n.expanded ? FolderOpen : Folder" class="size-4 shrink-0" />
            <span class="truncate">{{ n.name }}</span>
          </button>
        </li>
      </ul>

      <DialogFooter>
        <Button variant="outline" @click="open = false">Cancel</Button>
        <Button @click="update">Update</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
