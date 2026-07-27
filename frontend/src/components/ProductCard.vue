<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { ArrowRight, Zap } from 'lucide-vue-next'
import { gsap } from 'gsap'
import type { Product } from '@/types'
import { money } from '@/utils'
import PlatformMark from './PlatformMark.vue'

const props = defineProps<{ product: Product }>()

const cardRoot = ref<HTMLElement>()
const networkLayer = ref<HTMLElement>()
const ctaIcon = ref<HTMLElement>()
const titleId = computed(() => `product-card-title-${props.product.id}`)
const descriptionId = computed(() => `product-card-description-${props.product.id}`)

const activeSkus = computed(() => (props.product.skus || []).filter((sku) => sku.enabled !== false))
const hasStockCount = computed(() => activeSkus.value.some((sku) => sku.stock !== undefined))
const availableStock = computed(() => activeSkus.value.reduce((total, sku) => total + Math.max(0, Number(sku.stock || 0)), 0))
const stockText = computed(() => hasStockCount.value ? `库存 ${availableStock.value}` : '库存充足')
const specText = computed(() => {
  const skus = activeSkus.value
  if (!skus.length) return '标准 CDK 授权'
  if (skus.length === 1) return skus[0].name
  return `${skus.length} 种规格 · ${skus[0].name}起`
})
const displayPrice = computed(() => props.product.min_price ?? activeSkus.value[0]?.price ?? 0)

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
  if (reducedMotion) return

  animationContext = gsap.context((context) => {
    const scopedContext = context as ProductCardContext

    scopedContext.add('activate', () => {
      gsap.to(root, {
        y: -6,
        scale: 1.012,
        duration: 0.32,
        ease: 'power3.out',
        overwrite: 'auto',
      })
      gsap.to(network, {
        x: 5,
        y: -2,
        scale: 1.025,
        duration: 0.45,
        ease: 'power3.out',
        overwrite: 'auto',
      })
      gsap.to(icon, {
        x: 3,
        duration: 0.24,
        ease: 'power2.out',
        overwrite: 'auto',
      })
    })

    scopedContext.add('deactivate', () => {
      gsap.to(root, {
        y: 0,
        scale: 1,
        duration: 0.38,
        ease: 'power3.out',
        overwrite: 'auto',
      })
      gsap.to(network, {
        x: 0,
        y: 0,
        scale: 1,
        duration: 0.5,
        ease: 'power3.out',
        overwrite: 'auto',
      })
      gsap.to(icon, {
        x: 0,
        duration: 0.28,
        ease: 'power2.out',
        overwrite: 'auto',
      })
    })

    scopedContext.add('press', () => {
      gsap.to(root, {
        y: -2,
        scale: 0.99,
        duration: 0.12,
        ease: 'power2.out',
        overwrite: 'auto',
      })
      gsap.to(icon, {
        x: 1,
        duration: 0.12,
        ease: 'power2.out',
        overwrite: 'auto',
      })
    })

    gsap.from(root, {
      autoAlpha: 0,
      y: 18,
      duration: 0.55,
      ease: 'power3.out',
      clearProps: 'visibility,opacity',
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
    <header class="pcard__visual">
      <span ref="networkLayer" class="pcard__network" aria-hidden="true" />
      <div class="pcard__brand">
        <PlatformMark :name="product.platform?.name" size="lg" />
      </div>
      <span class="pcard__visual-label">{{ product.platform?.name || '数字授权' }}</span>
    </header>

    <div class="pcard__body">
      <div class="pcard__eyebrow">
        <span>{{ product.platform?.name || '代理平台' }}</span>
        <span class="pcard__delivery"><Zap :size="12" aria-hidden="true" />即时发货</span>
      </div>

      <h3 :id="titleId" class="pcard__title">{{ product.name }}</h3>
      <p :id="descriptionId" class="pcard__description">
        {{ product.description || '支付完成后，CDK 将即时展示在订单中。' }}
      </p>

      <div class="pcard__spec-row">
        <span class="pcard__spec">{{ specText }}</span>
        <span class="pcard__stock" :class="{ 'pcard__stock--empty': hasStockCount && availableStock === 0 }">
          <i aria-hidden="true" />{{ stockText }}
        </span>
      </div>

      <footer class="pcard__footer">
        <div class="pcard__price">
          <span>售价</span>
          <strong>{{ money(displayPrice) }}</strong>
          <small>起</small>
        </div>
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
            <ArrowRight :size="16" />
          </span>
        </RouterLink>
      </footer>
    </div>
  </article>
</template>

<style scoped>
.pcard {
  --pcard-orange: #f05a28;
  --pcard-ink: #111827;
  position: relative;
  min-width: 0;
  overflow: hidden;
  background: #fff;
  border: 1px solid #dce1e7;
  border-radius: 6px;
  box-shadow: 0 5px 18px rgba(18, 27, 39, 0.055);
  transform-origin: 50% 82%;
  will-change: transform;
}

.pcard:hover,
.pcard:focus-within {
  border-color: #c7cdd5;
  box-shadow: 0 15px 34px rgba(18, 27, 39, 0.11);
}

.pcard__visual {
  position: relative;
  height: 116px;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  isolation: isolate;
  background: #080d14;
  border-bottom: 1px solid #202936;
}

.pcard__network {
  position: absolute;
  inset: -28px -22px;
  z-index: -1;
  opacity: 0.86;
  background: url('/art/network-mesh-blue.png') center / cover no-repeat;
  transform-origin: 50% 50%;
  will-change: transform;
}

.pcard__visual::after {
  content: '';
  position: absolute;
  inset: 0;
  z-index: -1;
  background: rgba(5, 9, 14, 0.22);
  pointer-events: none;
}

.pcard__brand {
  position: relative;
  z-index: 1;
  display: grid;
  place-items: center;
  min-width: 150px;
  min-height: 58px;
  padding: 6px 14px;
}

.pcard__brand :deep(.platform-mark--lg) {
  width: 62px;
  height: 62px;
  border-radius: 6px;
}

.pcard__brand :deep(.platform-mark--logo.platform-mark--lg) {
  width: 138px;
  height: 48px;
  border-radius: 0;
}

.pcard__visual-label {
  position: absolute;
  right: 12px;
  bottom: 9px;
  color: rgba(224, 232, 240, 0.7);
  font-size: 10px;
  font-weight: 600;
  line-height: 1;
}

.pcard__body {
  padding: 15px 15px 14px;
}

.pcard__eyebrow,
.pcard__spec-row,
.pcard__footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.pcard__eyebrow {
  min-height: 21px;
  color: #667085;
  font-size: 11px;
  font-weight: 600;
}

.pcard__delivery {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  padding: 4px 6px;
  color: #147a43;
  background: #edf9f1;
  border: 1px solid #cdebd7;
  border-radius: 4px;
  white-space: nowrap;
}

.pcard__title {
  min-height: 44px;
  margin: 11px 0 5px;
  color: var(--pcard-ink);
  font-size: 17px;
  font-weight: 700;
  line-height: 1.35;
  letter-spacing: 0;
  display: -webkit-box;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.pcard__description {
  min-height: 40px;
  margin: 0;
  color: #667085;
  font-size: 12px;
  line-height: 1.65;
  display: -webkit-box;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.pcard__spec-row {
  min-height: 28px;
  margin-top: 11px;
  padding-top: 10px;
  border-top: 1px solid #edf0f3;
  color: #475467;
  font-size: 11px;
}

.pcard__spec {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pcard__stock {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  flex: none;
  color: #198754;
  font-variant-numeric: tabular-nums;
}

.pcard__stock i {
  width: 6px;
  height: 6px;
  background: currentColor;
  border-radius: 50%;
  box-shadow: 0 0 0 3px rgba(25, 135, 84, 0.1);
}

.pcard__stock--empty {
  color: #b54708;
}

.pcard__footer {
  min-height: 44px;
  margin-top: 10px;
}

.pcard__price {
  display: flex;
  align-items: baseline;
  min-width: 0;
  color: var(--pcard-ink);
  white-space: nowrap;
}

.pcard__price > span {
  margin-right: 6px;
  color: #98a2b3;
  font-size: 10px;
}

.pcard__price strong {
  font-size: 20px;
  font-weight: 750;
  font-variant-numeric: tabular-nums;
}

.pcard__price small {
  margin-left: 3px;
  color: #98a2b3;
  font-size: 10px;
}

.pcard__cta {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 5px;
  flex: none;
  min-height: 36px;
  padding: 0 11px;
  color: #fff;
  background: var(--pcard-orange);
  border: 1px solid #dd4918;
  border-radius: 5px;
  box-shadow: 0 4px 10px rgba(240, 90, 40, 0.2);
  font-size: 12px;
  font-weight: 700;
  text-decoration: none;
}

.pcard__cta:hover {
  color: #fff;
  background: #dc4c1c;
}

.pcard__cta:focus-visible {
  outline: 3px solid rgba(240, 90, 40, 0.28);
  outline-offset: 2px;
}

.pcard__cta-icon {
  display: grid;
  place-items: center;
  will-change: transform;
}

@media (max-width: 430px) {
  .pcard__visual {
    height: 108px;
  }

  .pcard__body {
    padding: 14px;
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
