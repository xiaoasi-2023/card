<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { ChevronRight, Coins, KeyRound, LockKeyhole, ReceiptText } from 'lucide-vue-next'
import { meApi, rows } from '@/api/services'
import { apiMessage } from '@/api/client'
import { useAuthStore } from '@/stores/auth'
import { dateTime, ledgerType, money } from '@/utils'
import type { Ledger, Order } from '@/types'
import UiStatus from '@/components/UiStatus.vue'

const route = useRoute()
const auth = useAuthStore()

const orders = ref<Order[]>([])
const ledgers = ref<Ledger[]>([])
const loading = ref(true)
const error = ref('')
const current = ref('')
const next = ref('')
const confirm = ref('')
const success = ref('')

const tab = computed(() =>
  route.path === '/me/balance'
    ? 'balance'
    : route.path === '/me/orders'
      ? 'orders'
      : route.path === '/me/security'
        ? 'security'
        : 'overview',
)

async function load() {
  loading.value = true
  try {
    const [orderResponse, ledgerResponse] = await Promise.all([
      meApi.orders({ page_size: 20 }),
      meApi.ledgers({ page_size: 20 }),
    ])
    orders.value = rows(orderResponse)
    ledgers.value = rows(ledgerResponse)
    await auth.restore()
  } catch (e) {
    error.value = apiMessage(e)
  } finally {
    loading.value = false
  }
}

async function changePassword() {
  error.value = ''
  success.value = ''
  if (next.value !== confirm.value) {
    error.value = '两次输入的新密码不一致'
    return
  }
  try {
    await meApi.password({
      current_password: current.value,
      new_password: next.value,
    })
    success.value = '密码已更新'
    current.value = ''
    next.value = ''
    confirm.value = ''
  } catch (e) {
    error.value = apiMessage(e)
  }
}

onMounted(load)
watch(() => route.path, () => {
  error.value = ''
  success.value = ''
})
</script>

<template>
  <section class="section container member-page">
    <div class="member-head">
      <div>
        <span>会员中心</span>
        <h1>{{ auth.user?.username }}</h1>
      </div>
      <div class="balance-block">
        <small>可用余额</small>
        <strong>{{ money(auth.user?.balance) }}</strong>
      </div>
    </div>

    <nav class="member-tabs">
      <RouterLink to="/me">概览</RouterLink>
      <RouterLink to="/me/orders">我的订单</RouterLink>
      <RouterLink to="/me/balance">余额流水</RouterLink>
      <RouterLink to="/me/security">账号安全</RouterLink>
    </nav>

    <div v-if="loading" class="state">正在加载</div>
    <div v-else-if="error && tab !== 'security'" class="alert alert--error">{{ error }}</div>

    <template v-else>
      <div v-if="tab === 'overview'" class="member-overview">
        <div class="metric-row">
          <div>
            <Coins />
            <span>账户余额</span>
            <strong>{{ money(auth.user?.balance) }}</strong>
          </div>
          <div>
            <ReceiptText />
            <span>全部订单</span>
            <strong>{{ orders.length }}</strong>
          </div>
          <div>
            <KeyRound />
            <span>已完成</span>
            <strong>{{ orders.filter((item) => item.status === 'completed').length }}</strong>
          </div>
        </div>

        <div class="panel">
          <div class="panel-head">
            <h2>最近订单</h2>
            <RouterLink to="/me/orders">查看全部</RouterLink>
          </div>
          <div v-for="order in orders.slice(0, 5)" :key="order.order_no" class="list-row">
            <div>
              <strong>{{ order.product_name || order.order_no }}</strong>
              <span>{{ dateTime(order.created_at) }} · {{ money(order.total_amount) }}</span>
            </div>
            <UiStatus :value="order.status" />
            <RouterLink :to="`/me/orders/${order.order_no}`" class="icon-btn" aria-label="查看订单">
              <ChevronRight />
            </RouterLink>
          </div>
          <div v-if="!orders.length" class="state">
            暂无订单
            <RouterLink class="text-link" to="/?catalog=all">去选购商品</RouterLink>
          </div>
        </div>
      </div>

      <div v-if="tab === 'orders'" class="panel">
        <div class="panel-head">
          <h2>我的订单</h2>
          <span>共 {{ orders.length }} 笔</span>
        </div>
        <div v-for="order in orders" :key="order.order_no" class="list-row order-list-row">
          <div>
            <strong>{{ order.product_name || 'CDK 商品' }}</strong>
            <span>{{ order.order_no }} · {{ dateTime(order.created_at) }}</span>
          </div>
          <span>{{ money(order.total_amount) }}</span>
          <UiStatus :value="order.status" />
          <RouterLink :to="`/me/orders/${order.order_no}`" class="icon-btn" aria-label="查看订单">
            <ChevronRight />
          </RouterLink>
        </div>
        <div v-if="!orders.length" class="state">
          暂无订单
          <RouterLink class="text-link" to="/?catalog=all">去选购商品</RouterLink>
        </div>
      </div>

      <div v-if="tab === 'balance'" class="panel">
        <div class="panel-head">
          <h2>余额流水</h2>
          <strong>{{ money(auth.user?.balance) }}</strong>
        </div>
        <div class="table-wrap">
          <table>
            <thead>
              <tr>
                <th>类型</th>
                <th>说明</th>
                <th>金额</th>
                <th>变动后余额</th>
                <th>时间</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="ledger in ledgers" :key="ledger.id">
                <td>{{ ledgerType[ledger.type] || ledger.type }}</td>
                <td>{{ ledger.reason || '-' }}</td>
                <td :class="Number(ledger.amount) > 0 ? 'amount-plus' : 'amount-minus'">
                  {{ Number(ledger.amount) > 0 ? '+' : '' }}{{ money(ledger.amount) }}
                </td>
                <td>{{ money(ledger.balance_after) }}</td>
                <td>{{ dateTime(ledger.created_at) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-if="!ledgers.length" class="state">暂无余额流水</div>
      </div>

      <form v-if="tab === 'security'" class="security-form" @submit.prevent="changePassword">
        <div class="form-title">
          <LockKeyhole />
          <div>
            <h2>修改登录密码</h2>
            <p>更新后请使用新密码登录。</p>
          </div>
        </div>
        <label class="form-field">
          <span>当前密码</span>
          <input v-model="current" type="password" required />
        </label>
        <label class="form-field">
          <span>新密码</span>
          <input v-model="next" type="password" minlength="6" required />
        </label>
        <label class="form-field">
          <span>确认新密码</span>
          <input v-model="confirm" type="password" required />
        </label>
        <div v-if="error" class="alert alert--error">{{ error }}</div>
        <div v-if="success" class="alert alert--success">{{ success }}</div>
        <button class="btn btn--primary" type="submit">保存新密码</button>
      </form>
    </template>
  </section>
</template>
