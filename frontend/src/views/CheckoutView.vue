<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { CreditCard, WalletCards } from 'lucide-vue-next'
import { publicApi, meApi } from '@/api/services'
import { demoProducts } from '@/demo'
import { apiMessage } from '@/api/client'
import { idempotencyKey, money } from '@/utils'
import type { Product, Sku } from '@/types'
const route=useRoute(),router=useRouter(); const product=ref<Product>();const sku=ref<Sku>();const method=ref<'balance'|'online'>('balance');const loading=ref(false);const error=ref('');const quantity=Number(route.query.quantity||1)
const total=computed(()=>Number(sku.value?.price||0)*quantity)
onMounted(async()=>{ const fallback=demoProducts.find(p=>p.id===Number(route.query.product_id)); const slug=String(route.query.product_slug||fallback?.slug||''); try{if(!slug)throw new Error('missing product slug');product.value=await publicApi.product(slug)}catch{product.value=fallback} sku.value=product.value?.skus?.find(s=>s.id===Number(route.query.sku_id))||product.value?.skus?.[0] })
async function submit(){ if(!sku.value)return; loading.value=true;error.value='';try{const order=await meApi.createOrder({sku_id:sku.value.id,quantity,payment_method:method.value,idempotency_key:idempotencyKey()});sessionStorage.setItem(`order_${order.order_no}`,JSON.stringify(order));router.push(method.value==='online'?`/pay/${order.order_no}`:`/me/orders/${order.order_no}`)}catch(e){error.value=apiMessage(e,'下单失败，请检查余额或库存')}finally{loading.value=false}}
</script>
<template><section class="section container checkout-page"><div class="page-title"><h1>确认订单</h1><p>登录用户可选择余额或在线支付。</p></div><div class="checkout-grid"><div class="checkout-main"><div class="form-section"><h2>支付方式</h2><div class="payment-options"><button :class="{selected:method==='balance'}" @click="method='balance'"><WalletCards/><span><strong>余额支付</strong><small>从账户余额扣款并立即交付</small></span></button><button :class="{selected:method==='online'}" @click="method='online'"><CreditCard/><span><strong>在线支付</strong><small>创建支付单，付款成功后交付</small></span></button></div></div><div class="form-section"><h2>商品信息</h2><div class="line-item"><div><strong>{{product?.name}}</strong><span>{{sku?.name}} × {{quantity}}</span></div><b>{{money(total)}}</b></div></div><div v-if="error" class="alert alert--error">{{error}}</div></div><aside class="order-summary"><h2>结算明细</h2><dl><div><dt>商品金额</dt><dd>{{money(total)}}</dd></div><div><dt>交付方式</dt><dd>订单内展示</dd></div><div class="summary-total"><dt>应付</dt><dd>{{money(total)}}</dd></div></dl><button class="btn btn--primary btn--wide" :disabled="loading" @click="submit">{{loading?'正在提交':'提交订单'}}</button><p>提交即确认数字商品交付后不支持退款。</p></aside></div></section></template>
