<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { meApi } from '@/api/services'
import { apiMessage } from '@/api/client'
import { dateTime, money } from '@/utils'
import type { Order } from '@/types'
import UiStatus from '@/components/UiStatus.vue'
import CdkReveal from '@/components/CdkReveal.vue'

const route = useRoute()
const order = ref<Order>()
const loading = ref(true)
const error = ref('')

onMounted(async () => {
  try {
    order.value = await meApi.order(String(route.params.orderNo))
  } catch (e) {
    error.value = apiMessage(e, '订单读取失败')
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <section class="section container narrow-content">
    <div v-if="loading" class="state">正在加载订单</div>
    <div v-else-if="error" class="alert alert--error">{{ error }}</div>

    <template v-else-if="order">
      <div class="order-detail-head">
        <div>
          <p>订单号 {{ order.order_no }}</p>
          <h1>{{ order.product_name || 'CDK 商品' }}</h1>
        </div>
        <UiStatus :value="order.status" />
      </div>

      <div class="panel">
        <dl class="detail-list">
          <div>
            <dt>商品规格</dt>
            <dd>{{ order.sku_name || '-' }} × {{ order.quantity }}</dd>
          </div>
          <div>
            <dt>支付方式</dt>
            <dd>{{ order.payment_method === 'balance' ? '余额支付' : '在线支付' }}</dd>
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
      </div>

      <div class="cdk-section panel">
        <template v-if="order.status === 'completed'">
          <CdkReveal :cards="order.cards" title="交付内容" empty-text="CDK 正在准备中，请稍后刷新" />
        </template>
        <div v-else class="state">订单完成后显示 CDK</div>
      </div>
    </template>
  </section>
</template>
