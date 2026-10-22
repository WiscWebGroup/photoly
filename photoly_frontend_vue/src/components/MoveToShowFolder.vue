<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { toast } from 'vue-sonner'
import { Check, ChevronRight, Folder, FolderOpen } from '@lucide/vue'
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

const props = defineProps<{
  title?: string
  originalNsId?: number
  disableOrgNsId?: boolean
  /** How many items are moving, and the name of the one when it's one. */
  count?: number
  itemName?: string
  /** Confirm button text; defaults to "Move to <folder>". */
  confirm?: string
}>()
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
    root.value.name = 'Library'
  } catch {
    toast.warning('Load Root Error!')
  }
})

const heading = computed(
  () =>
    props.title ??
    (props.count && props.count > 1 ? `Move ${props.count} Items` : 'Move to a Folder'),
)

defineExpose({
  openModal: () => {
    selectedNsId.value = -1
    selectedNsName.value = ''
    open.value = true
  },
})
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent>
      <DialogHeader>
        <DialogTitle>{{ heading }}</DialogTitle>
        <DialogDescription>
          {{ itemName ? `Pick a folder for ${itemName}.` : 'Pick a folder.' }}
        </DialogDescription>
      </DialogHeader>

      <ul role="tree" aria-label="Folders" class="max-h-72 overflow-auto rounded-xl border p-1.5">
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
                'hover:bg-accent flex h-9 w-full items-center gap-2 rounded-lg pr-2.5 text-left text-sm transition-colors disabled:cursor-default disabled:text-muted-foreground disabled:hover:bg-transparent',
                n.id === selectedNsId && 'bg-accent text-accent-foreground',
              )
            "
            @click="toggle(n)"
          >
            <ChevronRight
              :class="
                cn(
                  'text-muted-foreground size-4 shrink-0 transition-transform duration-200',
                  n.expanded && 'rotate-90',
                )
              "
              aria-hidden="true"
            />
            <component
              :is="n.expanded ? FolderOpen : Folder"
              class="size-4 shrink-0"
              aria-hidden="true"
            />
            <span class="truncate">{{ n.name }}</span>
            <span
              v-if="n.disabled"
              class="ml-auto rounded-full border px-2 text-xs leading-5 text-muted-foreground"
              >Current</span
            >
            <Check
              v-else-if="n.id === selectedNsId"
              class="text-primary animate-in zoom-in-50 ml-auto size-4 shrink-0"
              aria-hidden="true"
            />
          </button>
        </li>
      </ul>
      <p class="sr-only" aria-live="polite">
        {{ selectedNsName ? `Selected ${selectedNsName}` : '' }}
      </p>

      <DialogFooter>
        <Button variant="outline" @click="open = false">Cancel</Button>
        <Button :disabled="selectedNsId === -1" @click="update">{{
          confirm ?? (selectedNsName ? `Move to ${selectedNsName}` : 'Move')
        }}</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
