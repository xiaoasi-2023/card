<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { gsap } from 'gsap'
import {
  BadgeCheck,
  Check,
  ChevronRight,
  CircleAlert,
  RefreshCw,
  ShoppingBag,
  UserRound,
  WalletCards,
} from 'lucide-vue-next'
import { publicApi, rows } from '@/api/services'
import type { Platform, Product } from '@/types'
import PlatformMark from '@/components/PlatformMark.vue'
import ProductCard from '@/components/ProductCard.vue'

type SortKey = 'default' | 'latest' | 'stock' | 'price-asc' | 'price-desc'
type CatalogMode = 'all' | 'hot'

interface MarketProduct extends Product {
  marketKey: string
  displayPlatform?: Platform
  platformKey: string
  displayPrice: number | null
  displayStock: number | null
  highlights: string[]
  backendSort: number
  createdTimestamp: number
  sourceOrder: number
}

const route = useRoute()
const router = useRouter()
const root = ref<HTMLElement>()
const platforms = ref<Platform[]>([])
const products = ref<Product[]>([])
const catalogMode = ref<CatalogMode>('all')
const selectedPlatform = ref('all')
const sortBy = ref<SortKey>('default')
const isRefreshing = ref(false)
const catalogLoaded = ref(false)
const catalogError = ref('')
const refreshMessage = ref('')
const prefersReducedMotion = ref(false)

let animationContext: ReturnType<typeof gsap.context> | undefined
let catalogTimeline: ReturnType<typeof gsap.timeline> | undefined
let feedbackTimer: ReturnType<typeof setTimeout> | undefined
let motionQuery: MediaQueryList | undefined

const sortOptions: { value: SortKey; label: string }[] = [
  { value: 'default', label: '默认' },
  { value: 'latest', label: '最新上架' },
  { value: 'stock', label: '库存充足' },
  { value: 'price-asc', label: '价格从低到高' },
  { value: 'price-desc', label: '价格从高到低' },
]

function numericPrice(product: Product) {
  const value = Number(product.min_price ?? product.skus?.[0]?.price)
  return Number.isFinite(value) && value > 0 ? value : null
}

function numericStock(product: Product) {
  const skusWithStock = product.skus?.filter((sku) => typeof sku.stock === 'number') || []
  if (!skusWithStock.length) return null
  return skusWithStock.reduce((total, sku) => total + Number(sku.stock), 0)
}

function productHighlights(product: Product) {
  const description = product.description?.trim()
  if (!description) return []
  return description
    .split(/[，。；、]/)
    .map((item) => item.trim())
    .filter(Boolean)
    .slice(0, 3)
}

const marketProducts = computed<MarketProduct[]>(() => {
  return products.value.map((product, index) => {
    const displayPlatform = product.platform
      || platforms.value.find((platform) => platform.id === product.platform_id)
    const rawProduct = product as Product & { sort?: number; created_at?: string }
    return {
      ...product,
      marketKey: `${product.id}-${product.slug}`,
      displayPlatform,
      platformKey: displayPlatform?.slug || String(product.platform_id || 'unassigned'),
      displayPrice: numericPrice(product),
      displayStock: numericStock(product),
      highlights: productHighlights(product),
      backendSort: Number(rawProduct.sort) || 0,
      createdTimestamp: rawProduct.created_at ? Date.parse(rawProduct.created_at) || 0 : 0,
      sourceOrder: index,
    }
  })
})

const platformFilters = computed(() => {
  const available = [
    ...platforms.value,
    ...marketProducts.value.flatMap((product) => product.displayPlatform ? [product.displayPlatform] : []),
  ]
  return available.filter((platform, index, list) =>
    list.findIndex((item) => item.slug === platform.slug) === index
    && marketProducts.value.some((product) => product.platformKey === platform.slug),
  )
})

const hotProducts = computed(() => {
  return marketProducts.value.filter((product) => product.backendSort > 0)
})

const shownProducts = computed(() => {
  const catalogProducts = catalogMode.value === 'hot' ? hotProducts.value : marketProducts.value
  const filtered = selectedPlatform.value === 'all'
    ? catalogProducts
    : catalogProducts.filter((product) =>
      product.platformKey === selectedPlatform.value
      || String(product.platform_id) === selectedPlatform.value,
    )

  return [...filtered].sort((a, b) => {
    if (sortBy.value === 'latest') return b.createdTimestamp - a.createdTimestamp || a.sourceOrder - b.sourceOrder
    if (sortBy.value === 'stock') return (b.displayStock ?? -1) - (a.displayStock ?? -1)
    if (sortBy.value === 'price-asc') return (a.displayPrice ?? Number.POSITIVE_INFINITY) - (b.displayPrice ?? Number.POSITIVE_INFINITY)
    if (sortBy.value === 'price-desc') return (b.displayPrice ?? -1) - (a.displayPrice ?? -1)
    return a.sourceOrder - b.sourceOrder
  })
})

const emptyTitle = computed(() => {
  if (catalogMode.value === 'hot' && selectedPlatform.value !== 'all') return '该平台暂无热门商品'
  if (catalogMode.value === 'hot') return '暂无热门商品'
  if (selectedPlatform.value !== 'all') return '该平台暂无可售商品'
  return '暂无可售商品'
})

function queryValue(value: unknown) {
  return Array.isArray(value) ? String(value[0] || '') : String(value || '')
}

function syncCatalogFromRoute() {
  catalogMode.value = queryValue(route.query.catalog) === 'hot' ? 'hot' : 'all'
  const requestedPlatform = queryValue(route.query.platform)
  if (!requestedPlatform) {
    selectedPlatform.value = 'all'
    return
  }
  const matchingPlatform = platforms.value.find((platform) =>
    platform.slug === requestedPlatform || String(platform.id) === requestedPlatform,
  )
  selectedPlatform.value = matchingPlatform?.slug || requestedPlatform
}

async function setPlatform(slug: string) {
  const query = { ...route.query }
  if (slug === 'all') delete query.platform
  else query.platform = slug
  if (!query.catalog) query.catalog = catalogMode.value
  await router.replace({ path: '/', query, hash: route.hash })
}

function setSort(value: SortKey) {
  sortBy.value = value
}

async function resetCatalog() {
  await router.replace({ path: '/', query: { catalog: 'all' }, hash: route.hash })
}

function animateCatalog() {
  if (!root.value || prefersReducedMotion.value || !animationContext) return
  animationContext.add(() => {
    catalogTimeline?.kill()
    const cards = root.value?.querySelectorAll<HTMLElement>('.market-product-card')
    if (!cards?.length) return
    catalogTimeline = gsap.timeline({ defaults: { ease: 'power3.out' } })
      .fromTo(
        cards,
        { autoAlpha: 0, y: 16, scale: 0.985 },
        { autoAlpha: 1, y: 0, scale: 1, duration: 0.42, stagger: 0.035, clearProps: 'transform,opacity,visibility' },
      )
  })
}

function syncMotionPreference(event?: MediaQueryListEvent) {
  prefersReducedMotion.value = event?.matches ?? motionQuery?.matches ?? false
  if (!prefersReducedMotion.value || !root.value) return

  catalogTimeline?.kill()
  const animatedElements = root.value.querySelectorAll<HTMLElement>('.market-animate')
  gsap.killTweensOf(animatedElements)
  gsap.set(animatedElements, { clearProps: 'transform,opacity,visibility' })
}

function catalogErrorMessage(error: unknown) {
  const responseMessage = (error as { response?: { data?: { error?: { message?: string } } } })
    ?.response?.data?.error?.message
  return responseMessage
    ? `商品加载失败：${responseMessage}`
    : '商品加载失败，请检查网络或服务状态后重试'
}

async function loadAllProducts() {
  const allProducts: Product[] = []
  let page = 1
  const pageSize = 100

  while (true) {
    const response = await publicApi.products({ page, page_size: pageSize })
    const pageProducts = rows(response)
    allProducts.push(...pageProducts)
    if (pageProducts.length < pageSize) return allProducts
    page += 1
  }
}

async function loadCatalog(showFeedback = false) {
  isRefreshing.value = true
  catalogError.value = ''
  try {
    const [platformResponse, liveProducts] = await Promise.all([
      publicApi.platforms(),
      loadAllProducts(),
    ])
    platforms.value = platformResponse
    products.value = liveProducts
    catalogLoaded.value = true
    syncCatalogFromRoute()
    refreshMessage.value = showFeedback ? '商品与库存已更新' : ''
  } catch (error) {
    catalogError.value = catalogErrorMessage(error)
    refreshMessage.value = ''
  } finally {
    isRefreshing.value = false
    await nextTick()
    if (catalogLoaded.value) animateCatalog()
    if (feedbackTimer) clearTimeout(feedbackTimer)
    feedbackTimer = setTimeout(() => {
      refreshMessage.value = ''
    }, 2600)
  }
}

watch([selectedPlatform, sortBy], async () => {
  await nextTick()
  animateCatalog()
})

watch(
  () => [route.query.catalog, route.query.platform, route.hash],
  syncCatalogFromRoute,
)

onMounted(() => {
  motionQuery = window.matchMedia('(prefers-reduced-motion: reduce)')
  syncMotionPreference()
  motionQuery.addEventListener('change', syncMotionPreference)
  if (root.value) {
    animationContext = gsap.context(() => {
      if (prefersReducedMotion.value) {
        gsap.set('.market-animate', { clearProps: 'all' })
        return
      }
      gsap.timeline({ defaults: { duration: 0.55, ease: 'power3.out' } })
        .from('.market-filter-row', { autoAlpha: 0, y: 16 })
        .from('.market-sortbar', { autoAlpha: 0, y: 12 }, '-=0.36')
        .from('.market-flow-step, .market-buyer-note', { autoAlpha: 0, y: 12, stagger: 0.06 }, '-=0.28')
        .set('.market-animate', { clearProps: 'transform,opacity,visibility' })
    }, root.value)
  }
  syncCatalogFromRoute()
  void loadCatalog()
})

onUnmounted(() => {
  if (feedbackTimer) clearTimeout(feedbackTimer)
  motionQuery?.removeEventListener('change', syncMotionPreference)
  catalogTimeline?.kill()
  animationContext?.revert()
})
</script>

<template>
  <div ref="root" class="market-page">
    <main id="products" class="market-container market-catalog">
      <div class="market-filter-row market-animate">
        <span class="market-filter-label">平台筛选：</span>
        <div class="market-platform-tabs" role="group" aria-label="按平台筛选">
          <button
            type="button"
            class="market-platform-tab"
            :class="{ 'is-active': selectedPlatform === 'all' }"
            :aria-pressed="selectedPlatform === 'all'"
            @click="setPlatform('all')"
          >
            全部
          </button>
          <button
            v-for="platform in platformFilters"
            :key="platform.id"
            type="button"
            class="market-platform-tab"
            :class="{ 'is-active': selectedPlatform === platform.slug }"
            :aria-pressed="selectedPlatform === platform.slug"
            @click="setPlatform(platform.slug)"
          >
            <PlatformMark :name="platform.name" size="sm" />
            <span class="market-platform-name">{{ platform.name }}</span>
          </button>
        </div>
      </div>

      <div class="market-sortbar market-animate">
        <div class="market-sort-group" role="group" aria-label="商品排序">
          <span class="market-filter-label">排序：</span>
          <button
            v-for="option in sortOptions"
            :key="option.value"
            type="button"
            :class="{ 'is-active': sortBy === option.value }"
            :aria-pressed="sortBy === option.value"
            @click="setSort(option.value)"
          >
            {{ option.label }}
          </button>
        </div>
        <div class="market-refresh-wrap" aria-live="polite">
          <span v-if="refreshMessage" class="market-refresh-message">
            <Check :size="14" />
            {{ refreshMessage }}
          </span>
          <button
            type="button"
            class="market-refresh"
            :disabled="isRefreshing"
            @click="loadCatalog(true)"
          >
            <RefreshCw :size="17" :class="{ 'is-spinning': isRefreshing }" />
            {{ isRefreshing ? '更新中' : '刷新' }}
          </button>
        </div>
      </div>

      <div
        v-if="catalogError"
        class="market-error"
        :class="{ 'is-blocking': !marketProducts.length }"
        role="alert"
      >
        <CircleAlert :size="25" />
        <div>
          <strong>{{ catalogError }}</strong>
          <small v-if="marketProducts.length">当前仍显示上次成功加载的真实商品数据。</small>
        </div>
        <button type="button" :disabled="isRefreshing" @click="loadCatalog(true)">
          <RefreshCw :size="16" :class="{ 'is-spinning': isRefreshing }" />
          {{ isRefreshing ? '重试中' : '重新加载' }}
        </button>
      </div>

      <div v-if="isRefreshing && !catalogLoaded" class="market-loading" aria-live="polite">
        <RefreshCw :size="28" class="is-spinning" />
        <strong>正在读取商品数据</strong>
      </div>

      <div v-else-if="shownProducts.length" class="market-product-grid" aria-live="polite">
        <ProductCard
          v-for="product in shownProducts"
          :key="product.marketKey"
          class="market-product-card market-animate"
          :product="product"
          :platform="product.displayPlatform"
          :highlights="product.highlights"
          :display-price="product.displayPrice"
          :display-stock="product.displayStock"
          :animate="false"
        />
      </div>
      <div v-else-if="!catalogError" class="market-empty">
        <ShoppingBag :size="30" />
        <strong>{{ emptyTitle }}</strong>
        <button
          v-if="catalogMode !== 'all' || selectedPlatform !== 'all'"
          type="button"
          @click="resetCatalog"
        >
          查看全部商品
        </button>
      </div>
    </main>

    <section class="market-purchase-band">
      <div class="market-container market-purchase-layout">
        <ol class="market-flow" aria-label="购买流程">
          <li class="market-flow-step market-animate">
            <span>1</span>
            <div><strong>选择商品</strong><small>挑选合适的代理套餐</small></div>
            <ChevronRight :size="18" />
          </li>
          <li class="market-flow-step market-animate">
            <span>2</span>
            <div><strong>在线支付 / 余额支付</strong><small>注册用户支持余额和在线支付</small></div>
            <ChevronRight :size="18" />
          </li>
          <li class="market-flow-step market-animate">
            <span>3</span>
            <div><strong>自动发货</strong><small>支付订单自动显示 CDK</small></div>
            <ChevronRight :size="18" />
          </li>
          <li class="market-flow-step market-animate">
            <span>4</span>
            <div><strong>获取 CDK</strong><small>复制卡密，即开即用</small></div>
          </li>
        </ol>

        <div class="market-buyer-note market-animate">
          <div><UserRound :size="18" /><span><strong>注册用户：</strong>支持余额和在线支付</span></div>
          <div><WalletCards :size="18" /><span><strong>游客下单：</strong>免登录，仅支持在线支付</span></div>
          <small>游客购买需提供 QQ 或手机号 + 查询密码</small>
        </div>

        <aside class="market-purchase-notice market-animate">
          <strong><BadgeCheck :size="17" />购买说明</strong>
          <ul>
            <li>支付后订单直接显示 CDK，即时可见</li>
            <li>CDK 为一次性使用，开通后立即生效</li>
            <li>请妥善保管卡密，谨防泄露</li>
          </ul>
        </aside>
      </div>
    </section>
  </div>
</template>

<style scoped>
.market-page {
  --market-blue: var(--brand);
  --market-orange: var(--cta);
  --market-ink: var(--ink);
  --market-muted: var(--muted);
  --market-line: var(--line);
  min-height: 100%;
  color: var(--market-ink);
  background: var(--canvas);
}

.market-container {
  width: var(--container);
  margin: 0 auto;
}

.market-catalog {
  padding-top: 24px;
  padding-bottom: 26px;
}

.market-filter-row {
  min-height: 58px;
  display: grid;
  grid-template-columns: 84px 1fr;
  align-items: center;
  gap: 14px;
  will-change: transform, opacity;
}

.market-filter-label {
  color: #2c3542;
  font-size: 14px;
  font-weight: 700;
  white-space: nowrap;
}

.market-platform-tabs {
  display: grid;
  grid-template-columns: repeat(8, minmax(112px, 1fr));
  gap: 14px;
}

.market-platform-tab {
  height: 48px;
  min-width: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 9px;
  overflow: hidden;
  border: 1px solid #d7dde6;
  border-radius: 5px;
  color: #232c39;
  background: #fff;
  font-size: 13px;
  font-weight: 600;
}

.market-platform-tab:hover {
  border-color: #9cb7de;
  color: var(--market-blue);
  transform: translateY(-1px);
}

.market-platform-tab.is-active {
  border-color: var(--market-blue);
  color: var(--market-blue);
  box-shadow: inset 0 0 0 1px rgba(18, 104, 243, 0.18), 0 5px 16px rgba(18, 104, 243, 0.08);
}

.market-platform-tab :deep(.platform-mark--sm) {
  width: 65px;
  max-width: 65px;
  height: 25px;
}

.market-platform-tab :deep(.platform-mark--kookeey) {
  background: #101b17;
}

.market-platform-tab :deep(.platform-mark--logo) + .market-platform-name {
  display: none;
}

.market-platform-name {
  overflow: hidden;
  text-overflow: ellipsis;
}

.market-sortbar {
  min-height: 64px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  will-change: transform, opacity;
}

.market-sort-group {
  display: flex;
  align-items: center;
  gap: 10px;
}

.market-sort-group button {
  min-height: 36px;
  padding: 0 18px;
  border: 1px solid #dce2ea;
  border-radius: 4px;
  color: #445062;
  background: #fff;
  font-size: 12px;
}

.market-sort-group button:hover,
.market-sort-group button.is-active {
  border-color: var(--market-blue);
  color: var(--market-blue);
  background: #f5f9ff;
}

.market-sort-group button.is-active {
  font-weight: 700;
}

.market-refresh-wrap {
  min-height: 36px;
  display: flex;
  align-items: center;
  gap: 12px;
}

.market-refresh-message {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  color: #278052;
  font-size: 12px;
}

.market-refresh {
  min-height: 36px;
  display: inline-flex;
  align-items: center;
  gap: 7px;
  border: 0;
  color: #394557;
  background: transparent;
  font-size: 13px;
}

.market-refresh:hover {
  color: var(--market-blue);
}

.market-refresh:disabled {
  opacity: 0.65;
  cursor: wait;
}

.market-catalog .is-spinning {
  animation: market-spin 0.75s linear infinite;
}

.market-error {
  min-height: 58px;
  display: grid;
  grid-template-columns: 28px minmax(0, 1fr) auto;
  align-items: center;
  gap: 13px;
  margin-bottom: 18px;
  padding: 13px 16px;
  border: 1px solid #f3b5ab;
  border-radius: 6px;
  color: #9f2d20;
  background: #fff6f4;
}

.market-error.is-blocking {
  min-height: 220px;
  place-content: center;
  grid-template-columns: 28px minmax(0, 420px) auto;
}

.market-error > div {
  min-width: 0;
  display: grid;
  gap: 4px;
}

.market-error strong {
  font-size: 13px;
  line-height: 1.45;
}

.market-error small {
  color: #a85b52;
  font-size: 11px;
}

.market-error button {
  min-height: 34px;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 0 13px;
  border: 1px solid #d86456;
  border-radius: 4px;
  color: #9f2d20;
  background: #fff;
  font-size: 12px;
  font-weight: 700;
}

.market-error button:disabled {
  opacity: 0.62;
  cursor: wait;
}

.market-loading {
  min-height: 300px;
  display: grid;
  place-content: center;
  justify-items: center;
  gap: 13px;
  border: 1px solid #e1e6ed;
  color: #667386;
  background: #fff;
}

.market-loading strong {
  font-size: 13px;
}

.market-product-grid {
  display: grid;
  grid-template-columns: repeat(6, minmax(0, 1fr));
  gap: 18px;
}

.market-empty {
  min-height: 380px;
  display: grid;
  place-content: center;
  justify-items: center;
  gap: 12px;
  border: 1px dashed #cfd7e2;
  color: #687486;
  background: #fff;
}

.market-empty button {
  border: 0;
  color: var(--market-blue);
  background: transparent;
  font-weight: 700;
}

.market-purchase-band {
  border-top: 1px solid var(--market-line);
  border-bottom: 1px solid var(--market-line);
  background: #fff;
}

.market-purchase-layout {
  min-height: 112px;
  display: grid;
  grid-template-columns: minmax(0, 1.7fr) minmax(310px, 0.78fr) minmax(255px, 0.52fr);
  align-items: center;
  gap: 0;
}

.market-flow {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  margin: 0;
  padding: 0 28px 0 0;
  list-style: none;
}

.market-flow-step {
  min-width: 0;
  display: grid;
  grid-template-columns: 26px minmax(0, 1fr) 20px;
  align-items: center;
  gap: 10px;
  will-change: transform, opacity;
}

.market-flow-step:last-child {
  grid-template-columns: 26px minmax(0, 1fr);
}

.market-flow-step > span {
  width: 24px;
  height: 24px;
  display: grid;
  place-items: center;
  border-radius: 50%;
  color: #fff;
  background: var(--market-blue);
  font: 700 12px/1 Arial, sans-serif;
}

.market-flow-step > div {
  min-width: 0;
  display: grid;
  gap: 5px;
}

.market-flow-step strong {
  overflow: hidden;
  font-size: 13px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.market-flow-step small {
  overflow: hidden;
  color: #748092;
  font-size: 10px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.market-flow-step > svg {
  color: #9da8b6;
}

.market-buyer-note,
.market-purchase-notice {
  min-height: 80px;
  display: grid;
  align-content: center;
  gap: 9px;
  border-left: 1px solid var(--market-line);
  padding-left: 24px;
  will-change: transform, opacity;
}

.market-buyer-note > div {
  display: flex;
  align-items: center;
  gap: 9px;
  color: #485466;
  font-size: 11px;
}

.market-buyer-note svg {
  flex: 0 0 auto;
  color: #49576a;
}

.market-buyer-note small {
  padding-left: 27px;
  color: #788395;
  font-size: 10px;
}

.market-purchase-notice {
  padding-left: 24px;
}

.market-purchase-notice > strong {
  display: flex;
  align-items: center;
  gap: 7px;
  font-size: 12px;
}

.market-purchase-notice ul {
  display: grid;
  gap: 3px;
  margin: 0;
  padding-left: 17px;
  color: #687486;
  font-size: 10px;
}

@keyframes market-spin {
  to { transform: rotate(360deg); }
}

@media (max-width: 1180px) {
  .market-platform-tabs {
    grid-template-columns: repeat(4, minmax(130px, 1fr));
  }

  .market-product-grid {
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }

  .market-purchase-layout {
    grid-template-columns: 1fr 1fr;
    padding-top: 18px;
    padding-bottom: 18px;
    row-gap: 18px;
  }

  .market-flow {
    grid-column: 1 / -1;
    padding-right: 0;
  }

  .market-buyer-note {
    border-left: 0;
    padding-left: 0;
  }
}

@media (max-width: 820px) {
  .market-filter-row {
    grid-template-columns: 1fr;
    padding-top: 8px;
  }

  .market-platform-tabs {
    display: flex;
    overflow-x: auto;
    padding-bottom: 5px;
    scrollbar-width: none;
  }

  .market-platform-tabs::-webkit-scrollbar {
    display: none;
  }

  .market-platform-tab {
    min-width: 128px;
    flex: 0 0 auto;
  }

  .market-sortbar {
    align-items: flex-start;
    flex-direction: column;
    padding: 14px 0;
  }

  .market-sort-group {
    width: 100%;
    overflow-x: auto;
    padding-bottom: 3px;
  }

  .market-sort-group button {
    flex: 0 0 auto;
  }

  .market-refresh-wrap {
    width: 100%;
    justify-content: flex-end;
  }

  .market-error,
  .market-error.is-blocking {
    min-height: 0;
    grid-template-columns: 26px minmax(0, 1fr);
  }

  .market-error button {
    grid-column: 2;
    justify-self: start;
  }

  .market-product-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .market-flow {
    grid-template-columns: 1fr 1fr;
    gap: 18px;
  }

  .market-flow-step:nth-child(2) > svg {
    display: none;
  }

  .market-purchase-layout {
    grid-template-columns: 1fr;
  }

  .market-flow,
  .market-buyer-note,
  .market-purchase-notice {
    grid-column: 1;
  }

  .market-purchase-notice {
    border-top: 1px solid var(--market-line);
    border-left: 0;
    padding: 18px 0 0;
  }
}

@media (max-width: 480px) {
  .market-product-grid {
    grid-template-columns: 1fr;
  }

  .market-sort-group .market-filter-label {
    display: none;
  }

  .market-flow {
    grid-template-columns: 1fr;
  }

  .market-flow-step > svg {
    display: none;
  }

  .market-flow-step,
  .market-flow-step:last-child {
    grid-template-columns: 26px minmax(0, 1fr);
  }
}

@media (prefers-reduced-motion: reduce) {
  .market-platform-tab:hover {
    transform: none;
  }

  .market-catalog .is-spinning {
    animation: none;
  }
}
</style>
