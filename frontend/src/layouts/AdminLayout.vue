<script setup lang="ts">
import { ref } from 'vue'
import { RouterLink, RouterView } from 'vue-router'
import { Users, Layers3, Package, Tags, KeyRound, ReceiptText, CreditCard, Store, Menu, X } from 'lucide-vue-next'
const open = ref(false)
const nav = [
  ['/admin/orders','订单',ReceiptText], ['/admin/payments','支付记录',CreditCard], ['/admin/users','用户与余额',Users],
  ['/admin/platforms','平台',Layers3], ['/admin/products','商品',Package], ['/admin/skus','SKU',Tags], ['/admin/cards','卡密入库',KeyRound]
] as const
</script>
<template>
  <div class="admin-shell">
    <aside class="admin-sidebar" :class="{ open }">
      <RouterLink class="admin-brand" to="/"><span class="brand-mark">巳</span><span>阿巳管理台</span></RouterLink>
      <nav><RouterLink v-for="[to,label,Icon] in nav" :key="to" :to="to" @click="open=false"><component :is="Icon" :size="18" />{{ label }}</RouterLink></nav>
      <RouterLink to="/" class="admin-back"><Store :size="17" />返回商城</RouterLink>
    </aside>
    <section class="admin-workspace">
      <header class="admin-topbar"><button class="icon-btn mobile-only" @click="open=!open"><X v-if="open"/><Menu v-else/></button><span>运营管理</span><span class="admin-identity">管理员</span></header>
      <main class="admin-main"><RouterView /></main>
    </section>
  </div>
</template>
