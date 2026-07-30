<script setup lang="ts">
import { computed, ref, watch } from 'vue'

const props = defineProps<{ name?: string; size?: 'sm' | 'md' | 'lg' }>()

const brandLogos: Record<string, string> = {
  cliproxy: '/brands/cliproxy.png',
  kookeey: '/brands/kookeey.svg',
  b2proxy: '/brands/b2proxy.webp',
  '711proxy': '/brands/711proxy-header.webp',
  bunnyproxy: '/brands/bunnyproxy.png',
  udealproxy: '/brands/udealproxy-logo-new.png',
}

const normalizedName = computed(() => (props.name || '').replace(/[^a-z0-9]/gi, '').toLowerCase())
const logoSrc = computed(() => brandLogos[normalizedName.value])
const logoFailed = ref(false)
const logoClasses = computed(() => logoSrc.value && !logoFailed.value
  ? ['platform-mark--logo', `platform-mark--${normalizedName.value}`]
  : [])
const letters = computed(() => (props.name || 'CDK').replace(/proxy/ig, '').slice(0, 2).toUpperCase())
const hue = computed(() => [...(props.name || '')].reduce((sum, char) => sum + char.charCodeAt(0), 0) % 300 + 20)

watch(normalizedName, () => { logoFailed.value = false })
</script>

<template>
  <span
    class="platform-mark"
    :class="[`platform-mark--${size || 'md'}`, ...logoClasses]"
    :style="logoSrc && !logoFailed ? undefined : { '--mark-hue': hue }"
  >
    <img
      v-if="logoSrc && !logoFailed"
      :src="logoSrc"
      :alt="`${name || 'CDK'} Logo`"
      loading="lazy"
      @error="logoFailed = true"
    >
    <template v-else>{{ letters }}</template>
  </span>
</template>
