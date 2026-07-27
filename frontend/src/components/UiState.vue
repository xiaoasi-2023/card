<script setup lang="ts">
import { Inbox, LoaderCircle } from 'lucide-vue-next'
defineProps<{ loading?: boolean; empty?: boolean; error?: string; label?: string }>()
</script>
<template>
  <div v-if="loading" class="state"><LoaderCircle class="spin" :size="24" /><span>正在加载</span></div>
  <div v-else-if="error" class="alert alert--error">{{ error }}</div>
  <div v-else-if="empty" class="state"><Inbox :size="28" /><strong>{{ label || '暂无数据' }}</strong></div>
  <slot v-else />
</template>
