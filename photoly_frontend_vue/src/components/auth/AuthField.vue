<script setup lang="ts">
import { computed, ref, useId } from 'vue'
import { MorphIcon } from 'morphicons/vue'
import { Eye, EyeOff } from 'lucide'

const props = defineProps<{
  label: string
  type?: string
  autocomplete?: string
  placeholder?: string
  hint?: string
  /** Shown under the field and marks it invalid. */
  error?: string
}>()

const model = defineModel<string>({ default: '' })

const id = useId()
const messageId = `${id}-msg`
const isPassword = computed(() => props.type === 'password')
const revealed = ref(false)
const inputType = computed(() => (isPassword.value && revealed.value ? 'text' : props.type))
</script>

<template>
  <div class="flex flex-col gap-1.5">
    <div class="flex items-center justify-between gap-2">
      <label :for="id" class="text-sm font-medium text-white">{{ label }}</label>
      <slot name="aside" />
    </div>
    <div class="relative">
      <input
        :id="id"
        v-model="model"
        :type="inputType"
        :autocomplete="autocomplete"
        :placeholder="placeholder"
        :aria-invalid="error ? true : undefined"
        :aria-describedby="error || hint ? messageId : undefined"
        class="h-12 w-full rounded-xl border border-white/15 bg-white/5 px-3.5 text-base text-white transition-[border-color,box-shadow,background-color] duration-200 outline-none placeholder:text-teal-100/40 focus:border-teal-300 focus:ring-3 focus:ring-teal-300/25 sm:h-11 sm:rounded-[10px] sm:bg-[#031417]/60 sm:text-[15px] aria-invalid:border-red-400 aria-invalid:ring-3 aria-invalid:ring-red-400/25"
        :class="{ 'pr-12': isPassword || $slots.trailing }"
      />
      <button
        v-if="isPassword"
        type="button"
        :aria-label="revealed ? 'Hide password' : 'Show password'"
        :aria-pressed="revealed"
        class="absolute inset-y-0 right-0 flex w-11 items-center justify-center rounded-r-[10px] text-teal-100/60 transition-colors outline-none hover:text-white focus-visible:text-teal-300"
        @click="revealed = !revealed"
      >
        <MorphIcon
          :icon="revealed ? EyeOff : Eye"
          :size="18"
          spring="smooth"
          reduced-motion="user"
        />
      </button>
      <div
        v-else-if="$slots.trailing"
        class="pointer-events-none absolute inset-y-0 right-0 flex w-11 items-center justify-center"
      >
        <slot name="trailing" />
      </div>
    </div>
    <Transition name="reveal" mode="out-in">
      <span v-if="error" :id="messageId" :key="error" class="text-xs text-red-300" role="alert">{{
        error
      }}</span>
      <span v-else-if="hint" :id="messageId" class="text-xs text-teal-100/70">{{ hint }}</span>
    </Transition>
    <slot name="below" />
  </div>
</template>
