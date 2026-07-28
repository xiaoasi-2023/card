<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { Search } from 'lucide-vue-next'
import { guestApi } from '@/api/services'
import { apiMessage } from '@/api/client'
import { dateTime, money } from '@/utils'
import UiStatus from '@/components/UiStatus.vue'
import CdkReveal from '@/components/CdkReveal.vue'
import type { Order } from '@/types'

const route = useRoute()
const orderNo = ref(String(route.params.orderNo || ''))
const type = ref<'qq' | 'phone'>('qq')
const contact = ref('')
const password = ref('')
const order = ref<Order>()
const loading = ref(false)
const error = ref('')

async function query() {
  loading.value = true
  error.value = ''
  try {
    order.value = await guestApi.query({
      order_no: orderNo.value,
      contact_type: type.value,
      contact: contact.value,
      query_password: password.value,
    })
  } catch (e) {
    order.value = undefined
    error.value = apiMessage(e, '订单信息不匹配')
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  const saved = sessionStorage.getItem(`guest_${orderNo.value}`)
  if (saved) {
    const value = JSON.parse(saved)
    type.value = value.contact_type
    contact.value = value.contact
    password.value = value.query_password
    query()
  }
})
</script>

<template>
  <section class="section container lookup-page">
    <div class="page-title">
      <div>
        <h1>游客订单查询</h1>
        <p>输入下单时的完整凭证查看订单和 CDK。</p>
      </div>
    </div>

    <div class="lookup-layout">
      <form class="lookup-form" @submit.prevent="query">
        <label class="form-field">
          <span>订单号</span>
          <input v-model.trim="orderNo" placeholder="请输入订单号" required />
        </label>

        <div class="segmented">
          <button type="button" :class="{ active: type === 'qq' }" @click="type = 'qq'">QQ 号</button>
          <button type="button" :class="{ active: type === 'phone' }" @click="type = 'phone'">手机号</button>
        </div>

        <label class="form-field">
          <span>联系方式</span>
          <input
            v-model.trim="contact"
            :placeholder="type === 'qq' ? '请输入 QQ 号' : '请输入手机号'"
            required
          />
        </label>

        <label class="form-field">
          <span>本单查询密码</span>
          <input v-model="password" type="password" placeholder="请输入查询密码" required />
        </label>

        <div v-if="error" class="alert alert--error">{{ error }}</div>

        <button class="btn btn--primary btn--wide" type="submit" :disabled="loading">
          <Search :size="17" />
          {{ loading ? '正在查询' : '查询订单' }}
        </button>
      </form>

      <div class="lookup-result">
        <div v-if="!order" class="empty-result">
          <Search :size="32" />
          <strong>等待查询</strong>
          <span>订单信息仅会展示给凭证匹配的买家</span>
        </div>

        <template v-else>
          <div class="result-head">
            <div>
              <span>订单 {{ order.order_no }}</span>
              <h2>{{ order.product_name || 'CDK 商品' }}</h2>
            </div>
            <UiStatus :value="order.status" />
          </div>

          <dl class="detail-list">
            <div>
              <dt>商品规格</dt>
              <dd>{{ order.sku_name || '-' }} × {{ order.quantity }}</dd>
            </div>
            <div>
              <dt>订单金额</dt>
              <dd>{{ money(order.total_amount) }}</dd>
            </div>
            <div>
              <dt>创建时间</dt>
              <dd>{{ dateTime(order.created_at) }}</dd>
            </div>
          </dl>

          <div v-if="order.status === 'completed'" class="cdk-section">
            <CdkReveal :cards="order.cards" empty-text="CDK 正在准备中，请稍后刷新" />
          </div>
          <div v-else class="notice">订单完成后才会展示 CDK。</div>
        </template>
      </div>
    </div>
  </section>
</template>
