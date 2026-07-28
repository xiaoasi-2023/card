<script setup lang="ts">
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'

const router = useRouter()

onMounted(() => {
  router.replace({ path: '/', query: { catalog: 'all' } })
})
</script>

<template>
  <section class="section container">
    <div class="state">正在跳转商品列表…</div>
  </section>
</template>
