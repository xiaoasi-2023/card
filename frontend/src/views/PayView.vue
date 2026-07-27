<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { CheckCircle2, Clock3, ExternalLink, RefreshCw } from 'lucide-vue-next'
import { guestApi, meApi, paymentApi } from '@/api/services'
import { apiMessage } from '@/api/client'
import type { Order } from '@/types'
import { money } from '@/utils'
const route=useRoute(),router=useRouter();const orderNo=String(route.params.orderNo);const cached=sessionStorage.getItem(`order_${orderNo}`);const order=ref<Order>(cached?JSON.parse(cached):{order_no:orderNo,quantity:1,total_amount:0,payment_method:'online',status:'pending_payment'});const paying=ref(false);const error=ref('');const isMockPayment=computed(()=>String(order.value.payment_url||'').includes('/api/v1/dev/payments/'));const mockEnabled=computed(()=>String(route.query.mock)==='1'||import.meta.env.VITE_ENABLE_MOCK_PAYMENT==='true'||isMockPayment.value);let timer:number|undefined
async function refresh(){try{const guest=sessionStorage.getItem(`guest_${orderNo}`);order.value=guest?await guestApi.query({order_no:orderNo,...JSON.parse(guest)}):await meApi.order(orderNo);sessionStorage.setItem(`order_${orderNo}`,JSON.stringify(order.value));if(order.value.status==='completed'&&timer)clearInterval(timer)}catch{}}
async function mockPay(){paying.value=true;error.value='';try{order.value=await paymentApi.mockPay(order.value.order_no);await refresh()}catch(e){error.value=apiMessage(e,'模拟支付失败，请确认开发支付功能已启用')}finally{paying.value=false}}
function seeOrder(){const key=sessionStorage.getItem(`guest_${order.value.order_no}`);router.push(key?`/guest/orders/${order.value.order_no}`:`/me/orders/${order.value.order_no}`)}
onMounted(()=>{refresh();timer=window.setInterval(refresh,3000)});onBeforeUnmount(()=>timer&&clearInterval(timer))
</script>
<template><section class="section container narrow-page"><div class="payment-box"><template v-if="order.status==='completed'"><CheckCircle2 class="payment-icon success"/><h1>支付成功</h1><p>CDK 已分配，可立即查看订单。</p><button class="btn btn--primary" @click="seeOrder">查看订单</button></template><template v-else><Clock3 class="payment-icon"/><h1>等待支付</h1><p class="order-number">订单号 {{order.order_no}}</p><strong class="pay-amount">{{money(order.total_amount)}}</strong><a v-if="order.payment_url&&!isMockPayment" :href="order.payment_url" class="btn btn--primary" target="_blank">前往支付<ExternalLink :size="17"/></a><button v-if="mockEnabled" class="btn btn--secondary" :disabled="paying" @click="mockPay">{{paying?'正在处理':'模拟支付成功'}}</button><button class="text-link" @click="refresh"><RefreshCw :size="16"/>刷新状态</button><p class="muted">页面会自动检查支付结果，请勿重复付款。</p></template><div v-if="error" class="alert alert--error">{{error}}</div></div></section></template>
