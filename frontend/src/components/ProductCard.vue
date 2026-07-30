<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { ArrowRight, Bolt } from 'lucide-vue-next'
import { gsap } from 'gsap'
import type { Platform, Product } from '@/types'
import { money } from '@/utils'
import PlatformMark from './PlatformMark.vue'

const props = defineProps<{
  product: Product
  platform?: Platform
  highlights?: string[]
  displayPrice?: number | null
  displayStock?: number | null
  animate?: boolean
}>()

const cardRoot = ref<HTMLElement>()
const networkLayer = ref<HTMLElement>()
const ctaIcon = ref<HTMLElement>()

const titleId = computed(() => `product-card-title-${props.product.id}`)
const descriptionId = computed(() => `product-card-description-${props.product.id}`)

const resolvedPlatform = computed(() => props.platform || props.product.platform)
const platformKey = computed(() => resolvedPlatform.value?.slug || String(props.product.platform_id || 'unassigned'))

const activeSkus = computed(() => (props.product.skus || []).filter((sku) => sku.enabled !== false))

const resolvedPrice = computed(() => {
  if (props.displayPrice !== undefined) return props.displayPrice
  const value = Number(props.product.min_price ?? activeSkus.value[0]?.price)
  return Number.isFinite(value) && value > 0 ? value : null
})

const resolvedStock = computed(() => {
  if (props.displayStock !== undefined) return props.displayStock
  const skusWithStock = activeSkus.value.filter((sku) => typeof sku.stock === 'number')
  if (!skusWithStock.length) return null
  return skusWithStock.reduce((total, sku) => total + Number(sku.stock), 0)
})

const resolvedHighlights = computed(() => {
  if (props.highlights) return props.highlights
  const description = props.product.description?.trim()
  if (!description) return []
  return description
    .split(/[，。；、]/)
    .map((item) => item.trim())
    .filter(Boolean)
    .slice(0, 3)
})

const priceText = computed(() => {
  if (resolvedPrice.value === null) return '暂无报价'
  return money(resolvedPrice.value)
})

const stockText = computed(() => {
  if (resolvedStock.value === null) return '库存以结算为准'
  return `库存 ${resolvedStock.value}`
})

function platformTone(slug: string) {
  if (slug.includes('kookeey')) return 'emerald'
  if (slug.includes('bunny')) return 'ember'
  if (slug.includes('cliproxy')) return 'violet'
  return 'azure'
}

type ProductCardContext = ReturnType<typeof gsap.context> & {
  activate?: () => void
  deactivate?: () => void
  press?: () => void
}

let animationContext: ProductCardContext | undefined
let reducedMotion = false

function activate() {
  animationContext?.activate?.()
}

function deactivate() {
  if (cardRoot.value?.matches(':focus-within')) return
  animationContext?.deactivate?.()
}

function deactivateAfterFocus(event: FocusEvent) {
  if (event.relatedTarget instanceof Node && cardRoot.value?.contains(event.relatedTarget)) return
  animationContext?.deactivate?.()
}

function press() {
  animationContext?.press?.()
}

function release() {
  if (cardRoot.value?.matches(':hover, :focus-within')) activate()
  else animationContext?.deactivate?.()
}

onMounted(() => {
  const root = cardRoot.value
  const network = networkLayer.value
  const icon = ctaIcon.value
  if (!root || !network || !icon) return
  reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches
  if (reducedMotion || props.animate === false) return

  animationContext = gsap.context((context) => {
    const scopedContext = context as ProductCardContext

    scopedContext.add('activate', () => {
      gsap.to(root, {
        y: -4,
        duration: 0.28,
        ease: 'power3.out',
        overwrite: 'auto',
      })
      gsap.to(network, {
        scale: 1.04,
        duration: 0.4,
        ease: 'power3.out',
        overwrite: 'auto',
      })
      gsap.to(icon, {
        x: 2,
        duration: 0.22,
        ease: 'power2.out',
        overwrite: 'auto',
      })
    })

    scopedContext.add('deactivate', () => {
      gsap.to(root, {
        y: 0,
        duration: 0.32,
        ease: 'power3.out',
        overwrite: 'auto',
      })
      gsap.to(network, {
        scale: 1,
        duration: 0.42,
        ease: 'power3.out',
        overwrite: 'auto',
      })
      gsap.to(icon, {
        x: 0,
        duration: 0.24,
        ease: 'power2.out',
        overwrite: 'auto',
      })
    })

    scopedContext.add('press', () => {
      gsap.to(root, {
        y: -1,
        scale: 0.995,
        duration: 0.12,
        ease: 'power2.out',
        overwrite: 'auto',
      })
    })
  }, root) as ProductCardContext
})

onUnmounted(() => {
  animationContext?.revert()
  animationContext = undefined
})
</script>

<template>
  <article
    ref="cardRoot"
    class="pcard"
    :aria-labelledby="titleId"
    :aria-describedby="descriptionId"
    @mouseenter="activate"
    @mouseleave="deactivate"
    @focusin="activate"
    @focusout="deactivateAfterFocus"
  >
    <header class="pcard__visual" :data-tone="platformTone(platformKey)">
      <span ref="networkLayer" class="pcard__network" aria-hidden="true" />
      <div class="pcard__brand">
        <PlatformMark :name="resolvedPlatform?.name || '平台未配置'" size="md" />
      </div>
    </header>

    <div class="pcard__body">
      <h3 :id="titleId" class="pcard__title">{{ product.name }}</h3>

      <ul v-if="resolvedHighlights.length" :id="descriptionId" class="pcard__highlights">
        <li v-for="feature in resolvedHighlights" :key="feature">{{ feature }}</li>
      </ul>
      <p v-else :id="descriptionId" class="pcard__empty">暂无商品说明</p>

      <div class="pcard__stock-row">
        <span class="pcard__stock" :class="{ 'is-empty': resolvedStock === 0 }">
          <template v-if="resolvedStock !== null">
            库存 <strong>{{ resolvedStock }}</strong>
          </template>
          <template v-else>{{ stockText }}</template>
        </span>
        <span class="pcard__delivery"><Bolt :size="13" aria-hidden="true" />立即发货</span>
      </div>

      <footer class="pcard__footer">
        <strong class="pcard__price">{{ priceText }}</strong>
        <RouterLink
          :to="`/products/${product.slug}`"
          class="pcard__cta"
          :aria-label="`查看并购买 ${product.name}`"
          @pointerdown="press"
          @pointerup="release"
          @pointercancel="release"
          @pointerleave="release"
        >
          立即购买
          <span ref="ctaIcon" class="pcard__cta-icon" aria-hidden="true">
            <ArrowRight :size="15" />
          </span>
        </RouterLink>
      </footer>
    </div>
  </article>
</template>

<style scoped>
.pcard {
  min-width: 0;
  overflow: hidden;
  background: #fff;
  border: 1px solid #dce2ea;
  border-radius: 6px;
  transition: border-color 180ms ease, box-shadow 180ms ease;
  will-change: transform;
}

.pcard:hover,
.pcard:focus-within {
  z-index: 1;
  border-color: #9eb4d2;
  box-shadow: 0 14px 32px rgba(28, 45, 69, 0.12);
}

.pcard__visual {
  position: relative;
  height: 82px;
  display: grid;
  place-items: center;
  overflow: hidden;
  isolation: isolate;
  background: #071329;
}

.pcard__visual[data-tone='violet'] {
  background: #100d29;
}

.pcard__visual[data-tone='emerald'] {
  background: #071812;
}

.pcard__visual[data-tone='ember'] {
  background: #1b0c05;
}

.pcard__network {
  position: absolute;
  inset: -8px;
  z-index: -1;
  opacity: 0.62;
  background: url('/art/server-aisle-blue.png') center / cover no-repeat;
  transform: scale(1.04);
  transform-origin: 50% 50%;
  will-change: transform;
  pointer-events: none;
}

.pcard__visual[data-tone='violet'] .pcard__network {
  background-image: url('/art/network-mesh-blue.png');
  filter: hue-rotate(34deg) saturate(1.18);
}

.pcard__visual[data-tone='emerald'] .pcard__network {
  background-image: url('/art/network-mesh-blue.png');
  filter: hue-rotate(106deg) saturate(0.9);
}

.pcard__visual[data-tone='ember'] .pcard__network {
  background-image: url('/art/global-routing-orange.png');
  filter: none;
}

.pcard__brand {
  position: relative;
  z-index: 1;
  display: grid;
  place-items: center;
}

.pcard__brand :deep(.platform-mark--md) {
  width: 114px;
  max-width: 70%;
  height: 42px;
  color: #fff;
}

.pcard__brand :deep(.platform-mark:not(.platform-mark--logo)) {
  border-color: rgba(255, 255, 255, 0.35);
  color: #fff;
  background: rgba(255, 255, 255, 0.08);
}

.pcard__brand :deep(.platform-mark--kookeey) {
  padding: 8px 11px;
  background: #0b1c16;
}

.pcard__brand :deep(.platform-mark--cliproxy img),
.pcard__brand :deep(.platform-mark--b2proxy img),
.pcard__brand :deep(.platform-mark--711proxy img),

.pcard__body {
  padding: 11px 13px 10px;
}

.pcard__title {
  min-height: 34px;
  margin: 0 0 5px;
  overflow: hidden;
  color: #151d2b;
  font-size: 15px;
  font-weight: 700;
  line-height: 1.4;
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.pcard__highlights {
  min-height: 48px;
  display: grid;
  align-content: start;
  gap: 2px;
  margin: 0;
  padding: 0;
  overflow: hidden;
  list-style: none;
  color: #5f6b7c;
  font-size: 11px;
  line-height: 1.45;
}

.pcard__highlights li {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pcard__empty {
  min-height: 48px;
  display: grid;
  align-content: start;
  margin: 0;
  color: #8a94a3;
  font-size: 11px;
  line-height: 1.45;
}

.pcard__stock-row {
  min-height: 28px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
  margin-top: 5px;
  color: #748092;
  font-size: 11px;
}

.pcard__stock strong {
  color: #21935c;
  font-weight: 700;
}

.pcard__stock.is-empty strong {
  color: #b54708;
}

.pcard__delivery {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  padding: 3px 6px;
  border: 1px solid #bfe7cf;
  border-radius: 4px;
  color: #21935c;
  background: #f1fbf5;
  white-space: nowrap;
}

.pcard__footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding-top: 6px;
  border-top: 1px solid #edf0f4;
}

.pcard__price {
  min-width: 0;
  overflow: hidden;
  color: #151d29;
  font-family: Arial, sans-serif;
  font-size: 17px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pcard__cta {
  min-height: 30px;
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  gap: 3px;
  padding: 0 10px;
  border-radius: 4px;
  color: #fff;
  background: var(--cta, #ff4f12);
  font-size: 11px;
  font-weight: 700;
  text-decoration: none;
}

.pcard__cta:hover {
  color: #fff;
  background: var(--cta-strong, #e73f06);
  box-shadow: 0 6px 14px rgba(255, 79, 18, 0.2);
}

.pcard__cta:focus-visible {
  outline: 3px solid rgba(255, 79, 18, 0.28);
  outline-offset: 2px;
}

.pcard__cta-icon {
  display: grid;
  place-items: center;
  will-change: transform;
}

@media (max-width: 480px) {
  .pcard__visual {
    height: 112px;
  }

  .pcard__title {
    min-height: 0;
  }
}

@media (prefers-reduced-motion: reduce) {
  .pcard,
  .pcard__network,
  .pcard__cta-icon {
    transform: none !important;
    will-change: auto;
  }
}
</style>
