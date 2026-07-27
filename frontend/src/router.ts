import { createRouter, createWebHistory } from 'vue-router'
import StoreLayout from '@/layouts/StoreLayout.vue'
import AdminLayout from '@/layouts/AdminLayout.vue'

const routes = [
  { path: '/', component: StoreLayout, children: [
    { path: '', component: () => import('@/views/HomeView.vue') },
    { path: 'products', component: () => import('@/views/ProductsView.vue') },
    { path: 'products/:slug', component: () => import('@/views/ProductDetailView.vue') },
    { path: 'checkout/account', component: () => import('@/views/CheckoutView.vue'), meta: { auth: true } },
    { path: 'checkout/guest', component: () => import('@/views/GuestCheckoutView.vue') },
    { path: 'pay/:orderNo', component: () => import('@/views/PayView.vue') },
    { path: 'guest/orders/:orderNo?', component: () => import('@/views/GuestOrderView.vue') },
    { path: 'auth/login', component: () => import('@/views/AuthView.vue') },
    { path: 'auth/register', component: () => import('@/views/AuthView.vue') },
    { path: 'me', component: () => import('@/views/MemberView.vue'), meta: { auth: true } },
    { path: 'me/balance', component: () => import('@/views/MemberView.vue'), meta: { auth: true } },
    { path: 'me/orders', component: () => import('@/views/MemberView.vue'), meta: { auth: true } },
    { path: 'me/orders/:orderNo', component: () => import('@/views/OrderDetailView.vue'), meta: { auth: true } },
    { path: 'me/security', component: () => import('@/views/MemberView.vue'), meta: { auth: true } }
  ]},
  { path: '/admin', component: AdminLayout, meta: { auth: true, admin: true }, children: [
    { path: '', redirect: '/admin/orders' },
    { path: 'users', component: () => import('@/views/admin/AdminUsersView.vue') },
    { path: 'platforms', component: () => import('@/views/admin/AdminCatalogView.vue') },
    { path: 'products', component: () => import('@/views/admin/AdminCatalogView.vue') },
    { path: 'skus', component: () => import('@/views/admin/AdminCatalogView.vue') },
    { path: 'cards', component: () => import('@/views/admin/AdminCardsView.vue') },
    { path: 'orders', component: () => import('@/views/admin/AdminOrdersView.vue') },
    { path: 'payments', component: () => import('@/views/admin/AdminOrdersView.vue') }
  ]},
  { path: '/:pathMatch(.*)*', redirect: '/' }
]

const router = createRouter({
  history: createWebHistory(),
  routes,
  scrollBehavior: (to, from, savedPosition) => {
    if (savedPosition) return savedPosition
    if (to.path === from.path && to.hash === from.hash) return false
    if (to.hash) return { el: to.hash, top: 84, behavior: 'smooth' }
    return { top: 0 }
  }
})
router.beforeEach((to) => {
  const token = localStorage.getItem('cdk_token')
  if (to.meta.auth && !token) return { path: '/auth/login', query: { redirect: to.fullPath } }
  if (to.meta.admin) {
    try { if (JSON.parse(localStorage.getItem('cdk_user') || '{}').role !== 'admin') return '/' } catch { return '/' }
  }
})
export default router
