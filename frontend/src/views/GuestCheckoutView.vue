<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Eye, EyeOff, Info } from 'lucide-vue-next'
import { publicApi, guestApi } from '@/api/services'
import { apiMessage } from '@/api/client'
import { demoProducts } from '@/demo'
import { idempotencyKey, money } from '@/utils'
import type { Product, Sku } from '@/types'

const route = useRoute()
const router = useRouter()

const product = ref<Product>()
const sku = ref<Sku>()
const type = ref<'qq' | 'phone'>('qq')
const contact = ref('')
const password = ref('')
const show = ref(false)
const agree = ref(false)
const loading = ref(false)
const error = ref('')
const quantity = Number(route.query.quantity || 1)

const total = computed(() => Number(sku.value?.price || 0) * quantity)

onMounted(async () => {
  const fallback = demoProducts.find((item) => item.id === Number(route.query.product_id))
  const slug = String(route.query.product_slug || fallback?.slug || '')
  try {
    if (!slug) throw new Error('missing product slug')
    product.value = await publicApi.product(slug)
  } catch {
    product.value = fallback
  }
  sku.value = product.value?.skus?.find((item) => item.id === Number(route.query.sku_id))
    || product.value?.skus?.[0]
})

async function submit() {
  if (!sku.value) return
  if (!contact.value || password.value.length < 6 || !agree.value) {
    error.value = '请完整填写联系方式、6 至 32 位查询密码并确认交付规则'
    return
  }

  loading.value = true
  error.value = ''
  try {
    const order = await guestApi.createOrder({
      sku_id: sku.value.id,
      quantity,
      contact_type: type.value,
      contact: contact.value,
      query_password: password.value,
      idempotency_key: idempotencyKey(),
    })
    sessionStorage.setItem(
      `guest_${order.order_no}`,
      JSON.stringify({
        contact_type: type.value,
        contact: contact.value,
        query_password: password.value,
      }),
    )
    sessionStorage.setItem(`order_${order.order_no}`, JSON.stringify(order))
    router.push(`/pay/${order.order_no}`)
  } catch (e) {
    error.value = apiMessage(e, '创建游客订单失败')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <section class="section container checkout-page">
    <div class="page-title">
      <div>
        <h1>游客购买</h1>
        <p>无需注册，使用在线支付完成本次购买。请妥善保存查单凭证。</p>
      </div>
    </div>

    <div class="checkout-grid">
      <div class="checkout-main">
        <div class="form-section">
          <h2>查单凭证</h2>
          <div class="segmented">
            <button type="button" :class="{ active: type === 'qq' }" @click="type = 'qq'">QQ 号</button>
            <button type="button" :class="{ active: type === 'phone' }" @click="type = 'phone'">手机号</button>
          </div>

          <label class="form-field">
            <span>{{ type === 'qq' ? 'QQ 号' : '手机号' }}</span>
            <input
              v-model.trim="contact"
              :placeholder="type === 'qq' ? '请输入 QQ 号' : '请输入手机号'"
              autocomplete="off"
            />
          </label>

          <label class="form-field">
            <span>本单查询密码</span>
            <div class="input-with-icon">
              <input
                v-model="password"
                :type="show ? 'text' : 'password'"
                placeholder="请设置 6 至 32 位密码"
                autocomplete="new-password"
                maxlength="32"
              />
              <button
                type="button"
                :aria-label="show ? '隐藏密码' : '显示密码'"
                @click="show = !show"
              >
                <EyeOff v-if="show" />
                <Eye v-else />
              </button>
            </div>
          </label>

          <div class="notice">
            <Info :size="18" />
            <span>请妥善保存订单号和查询密码。密码仅用于本单查询，无法找回。</span>
          </div>
        </div>

        <div class="form-section">
          <h2>商品信息</h2>
          <div class="line-item">
            <div>
              <strong>{{ product?.name || '商品加载中' }}</strong>
              <span>{{ sku?.name || '-' }} × {{ quantity }}</span>
            </div>
            <b>{{ money(total) }}</b>
          </div>
        </div>

        <label class="check-row">
          <input v-model="agree" type="checkbox" />
          <span>我已确认商品规格，并了解 CDK 交付后不支持退款</span>
        </label>

        <div v-if="error" class="alert alert--error">{{ error }}</div>
      </div>

      <aside class="order-summary">
        <h2>在线支付</h2>
        <dl>
          <div>
            <dt>商品金额</dt>
            <dd>{{ money(total) }}</dd>
          </div>
          <div>
            <dt>账号</dt>
            <dd>无需登录</dd>
          </div>
          <div class="summary-total">
            <dt>应付</dt>
            <dd>{{ money(total) }}</dd>
          </div>
        </dl>
        <button class="btn btn--cta btn--wide" type="button" :disabled="loading" @click="submit">
          {{ loading ? '正在创建' : '创建订单并支付' }}
        </button>
        <p>创建后将跳转支付页，支付成功即可查看 CDK。</p>
      </aside>
    </div>
  </section>
</template>
