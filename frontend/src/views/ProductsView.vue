<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { Search } from 'lucide-vue-next'
import { publicApi, rows } from '@/api/services'
import { demoPlatforms, demoProducts } from '@/demo'
import type { Platform, Product } from '@/types'
import ProductCard from '@/components/ProductCard.vue'
const platforms = ref<Platform[]>(demoPlatforms); const products = ref<Product[]>(demoProducts); const selected = ref(''); const keyword = ref(''); const loading = ref(false)
const shown = computed(() => products.value.filter(p => (!selected.value || p.platform?.slug === selected.value || String(p.platform_id) === selected.value) && (!keyword.value || `${p.name}${p.description}${p.platform?.name}`.toLowerCase().includes(keyword.value.toLowerCase()))))
onMounted(async () => { loading.value = true; try { platforms.value = await publicApi.platforms(); products.value = rows(await publicApi.products()) } catch {} finally { loading.value = false } })
</script>
<template><section class="section container catalog-page"><div class="page-title"><h1>全部商品</h1><p>选择平台和适合你的套餐，付款后自动交付。</p></div><div class="catalog-tools"><div class="filter-tabs"><button :class="{active: !selected}" @click="selected=''">全部</button><button v-for="p in platforms" :key="p.id" :class="{active: selected===p.slug}" @click="selected=p.slug">{{ p.name }}</button></div><label class="search-field"><Search :size="18"/><input v-model="keyword" placeholder="搜索商品" /></label></div><div v-if="loading" class="state">正在加载商品</div><div v-else-if="shown.length" class="product-grid"><ProductCard v-for="p in shown" :key="p.id" :product="p"/></div><div v-else class="state">没有符合条件的商品</div></section></template>
