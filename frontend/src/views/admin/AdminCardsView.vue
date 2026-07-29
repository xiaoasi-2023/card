<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Copy, FileUp, Search } from 'lucide-vue-next'
import { adminApi, rows } from '@/api/services'
import { apiMessage } from '@/api/client'
import { skuLabel } from '@/utils'
import type { Sku } from '@/types'
import AdminPageHeader from '@/components/AdminPageHeader.vue'
import UiStatus from '@/components/UiStatus.vue'

const cards = ref<any[]>([])
const skus = ref<Sku[]>([])
const skuId = ref<number>()
const content = ref('')
const batch = ref<any>()
const importedCodes = ref<string[]>([])
const error = ref('')
const query = ref('')
const copied = ref('')

const selectedSku = computed(() => skus.value.find(s => s.id === skuId.value))

async function load() {
  try {
    cards.value = rows(await adminApi.list('cards', { q: query.value }))
    skus.value = rows(await adminApi.skus())
  } catch (e) {
    error.value = apiMessage(e)
  }
}

async function importCards() {
  error.value = ''
  importedCodes.value = []
  const values = content.value.split(/\r?\n/).map(v => v.trim()).filter(Boolean)
  if (!skuId.value || !values.length) {
    error.value = '请选择 SKU 并填写至少一条卡密'
    return
  }
  try {
    const result: any = await adminApi.create('card-batches', { sku_id: skuId.value, cards: values })
    batch.value = result?.batch || result
    importedCodes.value = Array.isArray(result?.claim_codes)
      ? result.claim_codes
      : Array.isArray(result?.items)
        ? result.items.map((item: any) => item.claim_code).filter(Boolean)
        : []
    content.value = ''
    await load()
  } catch (e) {
    error.value = apiMessage(e, '导入失败')
  }
}

function displayCode(card: any) {
  return card.claim_code || card.masked_value || card.value_masked || card.secret_masked || '••••••••'
}

function cardSkuLabel(card: any) {
  const matched = skus.value.find(s => s.id === card.sku_id)
  return skuLabel(matched, { fallback: card.sku_name || card.sku_id })
}

function skuOptionLabel(s: Sku) {
  const stock = s.stock == null ? '-' : s.stock
  return `${skuLabel(s, { withId: true })} · 库存 ${stock}`
}

async function copyText(value: string) {
  if (!value) return
  try {
    await navigator.clipboard.writeText(value)
    copied.value = value
    window.setTimeout(() => {
      if (copied.value === value) copied.value = ''
    }, 1200)
  } catch {
    error.value = '复制失败，请手动选择文本'
  }
}

async function copyImported() {
  if (!importedCodes.value.length) return
  await copyText(importedCodes.value.join('\n'))
}

onMounted(load)
</script>

<template>
  <AdminPageHeader title="卡密库存" description="导入上游库存后系统自动生成兑换码（TRAF-...），把兑换码灌给小铺即可。" />
  <div class="import-layout">
    <form class="import-panel" @submit.prevent="importCards">
      <div class="form-title">
        <FileUp />
        <div>
          <h2>批量导入</h2>
          <p>每行一条上游卡密；导入成功后会生成可灌小铺的兑换码。</p>
        </div>
      </div>
      <label class="form-field">
        <span>目标 SKU</span>
        <select v-model.number="skuId" required>
          <option :value="undefined" disabled>请选择 SKU</option>
          <option v-for="s in skus" :key="s.id" :value="s.id">{{ skuOptionLabel(s) }}</option>
        </select>
      </label>
      <label class="form-field">
        <span>上游卡密内容</span>
        <textarea
          v-model="content"
          class="code-input"
          rows="10"
          placeholder="每行一条上游卡密\n不要填写 TRAF 兑换码"
          required
        />
      </label>
      <button class="btn btn--primary btn--wide">开始导入</button>
      <div v-if="batch" class="alert alert--success">
        导入完成，批次 ID：{{ batch.id || batch.ID || '-' }}
        <span v-if="selectedSku"> · {{ skuLabel(selectedSku) }}</span>
      </div>
      <div v-if="importedCodes.length" class="import-result">
        <div class="import-result__head">
          <strong>本次生成的兑换码（灌小铺用）</strong>
          <button type="button" class="btn btn--secondary btn--sm" @click="copyImported">
            <Copy /> {{ copied && importedCodes.includes(copied) ? '已复制' : '复制全部' }}
          </button>
        </div>
        <pre class="code-block">{{ importedCodes.join('\n') }}</pre>
      </div>
    </form>

    <div class="data-panel">
      <div class="admin-toolbar">
        <label class="search-field">
          <Search />
          <input v-model="query" placeholder="搜索兑换码或 ID" @keyup.enter="load" />
        </label>
        <button class="btn btn--secondary btn--sm" @click="load">查询</button>
      </div>
      <div v-if="error" class="alert alert--error">{{ error }}</div>
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>ID</th>
              <th>SKU</th>
              <th>兑换码</th>
              <th>状态</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="c in cards" :key="c.id">
              <td>{{ c.id }}</td>
              <td>{{ cardSkuLabel(c) }}</td>
              <td><code>{{ displayCode(c) }}</code></td>
              <td><UiStatus :value="c.status" /></td>
              <td class="actions">
                <button
                  v-if="c.claim_code"
                  type="button"
                  class="btn btn--ghost btn--sm"
                  @click="copyText(c.claim_code)"
                >
                  {{ copied === c.claim_code ? '已复制' : '复制' }}
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div v-if="!cards.length" class="state">暂无卡密记录</div>
    </div>
  </div>
</template>

<style scoped>
.import-result {
  margin-top: 14px;
  padding: 12px;
  border: 1px solid var(--line, #dce2ea);
  border-radius: 10px;
  background: var(--soft, #f7f9fc);
}
.import-result__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 8px;
}
.code-block {
  margin: 0;
  max-height: 220px;
  overflow: auto;
  padding: 10px 12px;
  border-radius: 8px;
  background: #0f172a;
  color: #e2e8f0;
  font-size: 12px;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-all;
}
</style>
