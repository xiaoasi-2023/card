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
const route=useRoute(), router=useRouter(), auth=useAuthStore(); const product=ref<Product>(); const sku=ref<Sku>(); const quantity=ref(1); const loading=ref(true)
const total=computed(()=>Number(sku.value?.price||0)*quantity.value)
onMounted(async()=>{ try { product.value=await publicApi.product(String(route.params.slug)) } catch { product.value=demoProducts.find(p=>p.slug===route.params.slug) || demoProducts[0] } finally { sku.value=product.value?.skus?.[0]; loading.value=false } })
function buy(guest=false){ if(!sku.value||!product.value)return; const query={product_id:String(product.value.id),product_slug:product.value.slug,sku_id:String(sku.value.id),quantity:String(quantity.value)}; router.push({path: guest?'/checkout/guest':auth.loggedIn?'/checkout/account':'/auth/login',query: guest?query:{...query,redirect:'/checkout/account'}}) }
</script>
<template><section class="section container"><div v-if="loading" class="state">正在加载商品</div><div v-else-if="product" class="detail-grid"><div class="detail-visual"><PlatformMark :name="product.platform?.name" size="lg"/><strong>{{ product.platform?.name }}</strong><span>DIGITAL ACCESS CODE</span></div><div class="detail-info"><p class="breadcrumb">全部商品 / {{ product.platform?.name }}</p><h1>{{ product.name }}</h1><p class="detail-description">{{ product.description }}</p><div class="form-field"><label>选择规格</label><div class="sku-list"><button v-for="item in product.skus" :key="item.id" :class="{selected:sku?.id===item.id}" @click="sku=item"><span><strong>{{ item.name }}</strong><small>库存 {{ item.stock ?? '充足' }}</small></span><b>{{ money(item.price) }}</b><Check v-if="sku?.id===item.id" :size="17"/></button></div></div><div class="purchase-row"><div class="stepper"><button @click="quantity=Math.max(1,quantity-1)"><Minus/></button><span>{{quantity}}</span><button @click="quantity=Math.min(20,quantity+1)"><Plus/></button></div><div class="purchase-total"><small>应付金额</small><strong>{{money(total)}}</strong></div></div><button class="btn btn--primary btn--wide" @click="buy(false)">{{auth.loggedIn?'选择支付方式':'登录后购买'}}</button><button class="btn btn--secondary btn--wide" @click="buy(true)">游客直接购买</button><p class="delivery-note"><ShieldCheck :size="17"/>支付成功后在订单详情即时查看 CDK，数字商品交付后不支持退款。</p></div></div></section></template>
