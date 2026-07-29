<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { FileUp, Search } from 'lucide-vue-next'
import { adminApi, rows } from '@/api/services'
import { apiMessage } from '@/api/client'
import { skuLabel } from '@/utils'
import type { Sku } from '@/types'
import AdminPageHeader from '@/components/AdminPageHeader.vue'
import UiStatus from '@/components/UiStatus.vue'
const cards=ref<any[]>([]),skus=ref<Sku[]>([]),skuId=ref<number>(),content=ref(''),batch=ref<any>(),error=ref(''),query=ref('')
async function load(){try{cards.value=rows(await adminApi.list('cards',{q:query.value}));skus.value=rows(await adminApi.skus())}catch(e){error.value=apiMessage(e)}}
async function importCards(){error.value='';const values=content.value.split(/\r?\n/).map(v=>v.trim()).filter(Boolean);if(!skuId.value||!values.length){error.value='请选择 SKU 并填写至少一条卡密';return}try{batch.value=await adminApi.create('card-batches',{sku_id:skuId.value,cards:values});content.value='';load()}catch(e){error.value=apiMessage(e,'导入失败')}}
function cardSkuLabel(card: any) {
  const matched = skus.value.find(s => s.id === card.sku_id)
  return skuLabel(matched, { fallback: card.sku_name || card.sku_id })
}
function skuOptionLabel(s: Sku) {
  return skuLabel(s, { withId: true })
}
onMounted(load)
</script>
<template><AdminPageHeader title="卡密库存" description="批量导入 CDK，追踪库存分配状态。"/><div class="import-layout"><form class="import-panel" @submit.prevent="importCards"><div class="form-title"><FileUp/><div><h2>批量导入</h2><p>每行一条，系统会校验重复卡密。</p></div></div><label class="form-field"><span>目标 SKU</span><select v-model.number="skuId" required><option :value="undefined" disabled>请选择 SKU</option><option v-for="s in skus" :key="s.id" :value="s.id">{{skuOptionLabel(s)}}</option></select></label><label class="form-field"><span>CDK 内容</span><textarea v-model="content" class="code-input" rows="10" placeholder="CDK-XXXX-XXXX&#10;CDK-YYYY-YYYY" required/></label><button class="btn btn--primary btn--wide">开始导入</button><div v-if="batch" class="alert alert--success">导入任务已创建，批次 ID：{{batch.id}}</div></form><div class="data-panel"><div class="admin-toolbar"><label class="search-field"><Search/><input v-model="query" placeholder="搜索卡密尾号或订单" @keyup.enter="load"/></label><button class="btn btn--secondary btn--sm" @click="load">查询</button></div><div v-if="error" class="alert alert--error">{{error}}</div><div class="table-wrap"><table><thead><tr><th>ID</th><th>SKU</th><th>卡密</th><th>状态</th><th>关联订单</th></tr></thead><tbody><tr v-for="c in cards" :key="c.id"><td>{{c.id}}</td><td>{{cardSkuLabel(c)}}</td><td><code>{{c.masked_value||c.value_masked||'••••••••'}}</code></td><td><UiStatus :value="c.status"/></td><td>{{c.order_no||'-'}}</td></tr></tbody></table></div><div v-if="!cards.length" class="state">暂无卡密记录</div></div></div></template>
