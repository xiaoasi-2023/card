<script setup lang="ts">
import { ref } from 'vue'
import { Check, Copy } from 'lucide-vue-next'

const props = defineProps<{
  cards?: string[]
  title?: string
  emptyText?: string
}>()

const copied = ref('')

async function copy(value: string) {
  try {
    await navigator.clipboard.writeText(value)
    copied.value = value
    window.setTimeout(() => {
      if (copied.value === value) copied.value = ''
    }, 1400)
  } catch {
    // clipboard may be unavailable in insecure contexts
  }
}

async function copyAll() {
  if (!props.cards?.length) return
  await copy(props.cards.join('\n'))
}
</script>

<template>
  <div class="cdk-reveal">
    <div class="cdk-reveal__head">
      <h3>{{ title || '已交付 CDK' }}</h3>
      <button
        v-if="cards?.length && cards.length > 1"
        type="button"
        class="text-link"
        @click="copyAll"
      >
        全部复制
      </button>
    </div>

    <div v-if="cards?.length" class="cdk-reveal__list">
      <div v-for="card in cards" :key="card" class="cdk-row">
        <code>{{ card }}</code>
        <button
          class="icon-btn"
          type="button"
          :aria-label="copied === card ? '已复制' : '复制卡密'"
          @click="copy(card)"
        >
          <Check v-if="copied === card" :size="16" />
          <Copy v-else :size="16" />
        </button>
      </div>
    </div>
    <div v-else class="state cdk-reveal__empty">
      {{ emptyText || '暂无 CDK' }}
    </div>
  </div>
</template>

<style scoped>
.cdk-reveal__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 8px;
}

.cdk-reveal__head h3 {
  margin: 0;
  font-size: 16px;
}

.cdk-reveal__list {
  display: grid;
  gap: 0;
}

.cdk-reveal__empty {
  min-height: 88px;
}
</style>
