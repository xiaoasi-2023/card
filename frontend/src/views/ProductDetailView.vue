<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Check, Minus, Plus, ShieldCheck } from 'lucide-vue-next'
import { publicApi } from '@/api/services'
import { demoProducts } from '@/demo'
import { useAuthStore } from '@/stores/auth'
import type { Product, Sku } from '@/types'
import { money } from '@/utils'
import PlatformMark from '@/components/PlatformMark.vue'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

const product = ref<Product>()
const sku = ref<Sku>()
const quantity = ref(1)
const loading = ref(true)

const total = computed(() => Number(sku.value?.price || 0) * quantity.value)

const platformTone = computed(() => {
  const slug = product.value?.platform?.slug || ''
  if (slug.includes('kookeey')) return 'emerald'
  if (slug.includes('bunny')) return 'ember'
  if (slug.includes('cliproxy')) return 'violet'
  return 'azure'
})

onMounted(async () => {
  try {
    product.value = await publicApi.product(String(route.params.slug))
  } catch {
    product.value = demoProducts.find((item) => item.slug === route.params.slug) || demoProducts[0]
  } finally {
    sku.value = product.value?.skus?.[0]
    loading.value = false
  }
})

function buy(guest = false) {
  if (!sku.value || !product.value) return
  const query = {
    product_id: String(product.value.id),
    product_slug: product.value.slug,
    sku_id: String(sku.value.id),
    quantity: String(quantity.value),
  }
  router.push({
    path: guest ? '/checkout/guest' : auth.loggedIn ? '/checkout/account' : '/auth/login',
    query: guest ? query : { ...query, redirect: '/checkout/account' },
  })
}
</script>

<template>
  <section class="section container">
    <div v-if="loading" class="state">正在加载商品</div>

    <div v-else-if="product" class="detail-grid">
      <div class="detail-visual" :data-tone="platformTone">
        <span class="detail-visual-bg" aria-hidden="true" />
        <PlatformMark :name="product.platform?.name" size="lg" />
        <strong>{{ product.platform?.name || '代理平台' }}</strong>
        <span>DIGITAL ACCESS CODE</span>
      </div>

      <div class="detail-info">
        <p class="breadcrumb">
          <RouterLink to="/?catalog=all">全部商品</RouterLink>
          <span> / {{ product.platform?.name || '平台' }}</span>
        </p>
        <h1>{{ product.name }}</h1>
        <p class="detail-description">
          {{ product.description || '支付成功后，CDK 将即时展示在订单中。' }}
        </p>

        <div class="form-field">
          <label>选择规格</label>
          <div class="sku-list">
            <button
              v-for="item in product.skus"
              :key="item.id"
              type="button"
              :class="{ selected: sku?.id === item.id }"
              @click="sku = item"
            >
              <span>
                <strong>{{ item.name }}</strong>
                <small>库存 {{ item.stock ?? '充足' }}</small>
              </span>
              <b>{{ money(item.price) }}</b>
              <Check v-if="sku?.id === item.id" :size="17" />
            </button>
          </div>
        </div>

        <div class="purchase-row">
          <div class="stepper">
            <button type="button" aria-label="减少数量" @click="quantity = Math.max(1, quantity - 1)">
              <Minus />
            </button>
            <span>{{ quantity }}</span>
            <button type="button" aria-label="增加数量" @click="quantity = Math.min(20, quantity + 1)">
              <Plus />
            </button>
          </div>
          <div class="purchase-total">
            <small>应付金额</small>
            <strong>{{ money(total) }}</strong>
          </div>
        </div>

        <button class="btn btn--cta btn--wide" type="button" @click="buy(false)">
          {{ auth.loggedIn ? '选择支付方式' : '登录后购买' }}
        </button>
        <button class="btn btn--secondary btn--wide" type="button" @click="buy(true)">
          游客直接购买
        </button>

        <p class="delivery-note">
          <ShieldCheck :size="17" />
          支付成功后在订单详情即时查看 CDK。数字商品交付后不支持退款。
        </p>
      </div>
    </div>
  </section>
</template>

<style scoped>
.detail-visual[data-tone='violet'] {
  background: #100d29;
}

.detail-visual[data-tone='emerald'] {
  background: #071812;
}

.detail-visual[data-tone='ember'] {
  background: #1b0c05;
}

.detail-visual[data-tone='violet'] .detail-visual-bg {
  background-image: url('/art/network-mesh-blue.png');
  filter: hue-rotate(34deg) saturate(1.18);
}

.detail-visual[data-tone='emerald'] .detail-visual-bg {
  background-image: url('/art/network-mesh-blue.png');
  filter: hue-rotate(106deg) saturate(0.9);
}

.detail-visual[data-tone='ember'] .detail-visual-bg {
  background-image: url('/art/global-routing-orange.png');
  filter: none;
}

.detail-visual :deep(.platform-mark--lg) {
  position: relative;
  z-index: 1;
  width: min(220px, 70%);
  height: 72px;
  color: #fff;
}

.detail-visual :deep(.platform-mark:not(.platform-mark--logo)) {
  border-color: rgba(255, 255, 255, 0.35);
  color: #fff;
  background: rgba(255, 255, 255, 0.08);
}

.detail-visual :deep(.platform-mark--kookeey) {
  padding: 10px 14px;
  background: #0b1c16;
}

.detail-visual :deep(.platform-mark--cliproxy img),
.detail-visual :deep(.platform-mark--b2proxy img),
.detail-visual :deep(.platform-mark--711proxy img),
.detail-visual :deep(.platform-mark--ipweb img) {
  filter: brightness(0) invert(1);
}

.breadcrumb a {
  color: var(--brand);
  font-weight: 600;
}

.detail-info > .btn + .btn {
  margin-top: 10px;
}
</style>
