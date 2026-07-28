<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { RouterLink, RouterView, useRoute, useRouter } from 'vue-router'
import {
  BadgeCheck,
  CirclePlus,
  Headphones,
  LogOut,
  Menu,
  Search,
  ShieldCheck,
  UserRound,
  WalletCards,
  X,
  Zap,
} from 'lucide-vue-next'
import { gsap } from 'gsap'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const route = useRoute()
const router = useRouter()
const root = ref<HTMLElement | null>(null)
const mobileNav = ref<HTMLElement | null>(null)
const open = ref(false)

let ctx: gsap.Context | undefined
let menuTween: gsap.core.Tween | undefined

const prefersReducedMotion = () => window.matchMedia('(prefers-reduced-motion: reduce)').matches
const balance = () => `¥${Number(auth.user?.balance ?? 0).toFixed(2)}`

const isCatalogAll = computed(() => {
  if (route.path !== '/' && route.path !== '/products') return false
  return String(route.query.catalog || 'all') !== 'hot'
})

const isCatalogHot = computed(() => {
  if (route.path !== '/' && route.path !== '/products') return false
  return String(route.query.catalog || '') === 'hot'
})

function animateMobileMenu(entering: boolean, onComplete?: () => void) {
  if (!mobileNav.value || window.innerWidth > 760 || prefersReducedMotion()) {
    onComplete?.()
    return
  }

  menuTween?.kill()
  ctx?.add(() => {
    menuTween = entering
      ? gsap.fromTo(
          mobileNav.value,
          { autoAlpha: 0, y: -14, scaleY: 0.96, transformOrigin: 'top center' },
          { autoAlpha: 1, y: 0, scaleY: 1, duration: 0.32, ease: 'power3.out' },
        )
      : gsap.to(mobileNav.value, {
          autoAlpha: 0,
          y: -10,
          scaleY: 0.97,
          duration: 0.2,
          ease: 'power2.in',
          onComplete,
        })
  })
}

async function toggleMenu() {
  if (open.value) {
    animateMobileMenu(false, () => {
      open.value = false
      gsap.set(mobileNav.value, { clearProps: 'opacity,visibility,transform' })
    })
    return
  }

  open.value = true
  await nextTick()
  animateMobileMenu(true)
}

function closeMenu() {
  if (!open.value) return
  open.value = false
  menuTween?.kill()
  if (mobileNav.value) gsap.set(mobileNav.value, { clearProps: 'opacity,visibility,transform' })
}

async function signOut() {
  closeMenu()
  await auth.logout()
  router.push('/')
}

function handleKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') closeMenu()
}

onMounted(() => {
  if (!root.value) return

  ctx = gsap.context(() => {
    if (prefersReducedMotion()) return

    gsap
      .timeline({ defaults: { ease: 'power3.out' } })
      .from('.brand', { autoAlpha: 0, x: -18, duration: 0.48 })
      .from('.header-action, .catalog-link', { autoAlpha: 0, x: 12, duration: 0.34, stagger: 0.045 }, '-=0.26')
  }, root.value)

  window.addEventListener('keydown', handleKeydown)
})

watch(() => route.fullPath, () => {
  closeMenu()
})

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeydown)
  menuTween?.kill()
  ctx?.revert()
})
</script>

<template>
  <div ref="root" class="site-shell">
    <header class="site-header">
      <div class="container header-inner">
        <RouterLink to="/" class="brand" aria-label="阿巳首页">
          <span class="brand-mark">巳</span>
          <span class="brand-name">阿巳</span>
        </RouterLink>

        <nav class="catalog-nav" aria-label="商品导航">
          <RouterLink class="catalog-link" :class="{ 'is-active': isCatalogAll }" to="/?catalog=all">
            全部商品
          </RouterLink>
          <RouterLink class="catalog-link" :class="{ 'is-active': isCatalogHot }" to="/?catalog=hot">
            热门推荐
          </RouterLink>
        </nav>

        <button
          class="mobile-toggle mobile-only"
          type="button"
          aria-controls="store-navigation"
          :aria-expanded="open"
          :aria-label="open ? '关闭菜单' : '打开菜单'"
          @click="toggleMenu"
        >
          <X v-if="open" />
          <Menu v-else />
        </button>

        <nav
          id="store-navigation"
          ref="mobileNav"
          class="main-nav"
          :class="{ open }"
          aria-label="账户与订单"
        >
          <div class="mobile-catalog mobile-only">
            <RouterLink class="catalog-link" :class="{ 'is-active': isCatalogAll }" to="/?catalog=all">
              全部商品
            </RouterLink>
            <RouterLink class="catalog-link" :class="{ 'is-active': isCatalogHot }" to="/?catalog=hot">
              热门推荐
            </RouterLink>
          </div>

          <div class="nav-actions">
            <RouterLink class="query-control header-action" to="/guest/orders">
              <Search :size="18" aria-hidden="true" />
              <span>订单查询</span>
            </RouterLink>

            <template v-if="!auth.loggedIn">
              <RouterLink class="auth-link header-action" to="/auth/login">登录</RouterLink>
              <RouterLink class="register-link header-action" to="/auth/register">注册</RouterLink>
            </template>

            <template v-else>
              <RouterLink class="account-link header-action" to="/me">
                <UserRound :size="17" aria-hidden="true" />
                <span>{{ auth.user?.username }}</span>
              </RouterLink>
              <RouterLink class="balance-link header-action" to="/me/balance">
                <WalletCards :size="17" aria-hidden="true" />
                <span class="balance-copy">
                  <small>可用余额</small>
                  <strong>{{ balance() }}</strong>
                </span>
                <span class="recharge-action">
                  <CirclePlus :size="15" aria-hidden="true" />
                  充值
                </span>
              </RouterLink>
              <RouterLink
                v-if="auth.isAdmin"
                class="square-action header-action"
                to="/admin"
                aria-label="管理后台"
                title="管理后台"
              >
                <ShieldCheck :size="18" />
              </RouterLink>
              <button
                class="square-action header-action"
                type="button"
                aria-label="退出登录"
                title="退出登录"
                @click="signOut"
              >
                <LogOut :size="18" />
              </button>
            </template>
          </div>
        </nav>
      </div>
    </header>

    <div class="trust-bar" aria-label="服务承诺">
      <div class="container trust-inner">
        <div class="trust-item">
          <span class="trust-icon"><Zap :size="16" /></span>
          <div>
            <strong>自动发货：支付后立即显示 CDK</strong>
            <small>支付完成后，订单页面直接查看卡密</small>
          </div>
        </div>
        <div class="trust-item">
          <span class="trust-icon"><BadgeCheck :size="16" /></span>
          <div>
            <strong>余额可用：支持余额与在线支付</strong>
            <small>注册用户可使用余额或在线支付</small>
          </div>
        </div>
        <div class="trust-item">
          <span class="trust-icon"><Headphones :size="16" /></span>
          <div>
            <strong>客服在线：7×12 小时</strong>
            <small>专业客服及时为您服务解答</small>
          </div>
        </div>
      </div>
    </div>

    <main><RouterView /></main>

    <footer class="site-footer">
      <div class="container footer-inner">
        <div class="footer-brand">
          <span class="footer-title">阿巳 · 多平台代理 CDK</span>
          <span>数字商品交付后不支持退款，请确认规格后再下单</span>
        </div>
        <div class="footer-links">
          <RouterLink to="/guest/orders">订单查询</RouterLink>
          <RouterLink to="/auth/login">会员登录</RouterLink>
          <RouterLink to="/?catalog=all">全部商品</RouterLink>
        </div>
      </div>
    </footer>
  </div>
</template>

<style scoped>
.site-header {
  height: 68px;
  background: #050709;
  border-bottom: 1px solid #1f252b;
  color: #f7f9fc;
  box-shadow: 0 10px 30px rgb(0 0 0 / 10%);
}

.header-inner {
  width: var(--container);
  gap: 28px;
}

.brand {
  flex: none;
  gap: 9px;
  color: #fff;
  font-size: 22px;
  font-weight: 800;
}

.brand-mark {
  width: 36px;
  height: 36px;
  border-radius: 4px;
  background: var(--brand);
  color: #fff;
  box-shadow: 0 0 0 1px rgb(255 255 255 / 7%) inset;
  font-size: 21px;
  font-weight: 900;
}

.brand-name {
  white-space: nowrap;
}

.catalog-nav {
  display: flex;
  align-items: center;
  gap: 6px;
  flex: 1;
  min-width: 0;
}

.catalog-link {
  min-height: 36px;
  display: inline-flex;
  align-items: center;
  padding: 0 14px;
  border-radius: 5px;
  color: #c5ced8;
  font-size: 14px;
  font-weight: 600;
  white-space: nowrap;
}

.catalog-link:hover,
.catalog-link:focus-visible {
  color: #fff;
  background: #11151a;
  outline: none;
}

.catalog-link.is-active {
  color: #fff;
  background: #111820;
  box-shadow: inset 0 -2px 0 var(--brand);
}

.main-nav {
  flex: none;
  min-width: 0;
  justify-content: flex-end;
  gap: 28px;
}

.nav-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.main-nav a,
.main-nav button {
  font-size: 14px;
  font-weight: 600;
}

.query-control,
.auth-link,
.register-link,
.account-link,
.balance-link,
.square-action {
  min-height: 38px;
  border: 1px solid #343b44;
  border-radius: 5px;
  background: transparent;
  color: #eef2f7;
}

.query-control,
.auth-link,
.register-link,
.account-link {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 7px;
  padding: 0 15px;
  white-space: nowrap;
}

.query-control:hover,
.query-control:focus-visible,
.auth-link:hover,
.auth-link:focus-visible,
.account-link:hover,
.account-link:focus-visible,
.square-action:hover,
.square-action:focus-visible {
  border-color: #7b8795;
  background: #11151a;
  color: #fff;
  outline: none;
  transform: translateY(-1px);
}

.query-control:focus-visible {
  box-shadow: 0 0 0 3px rgb(22 119 255 / 30%);
}

.register-link {
  border-color: var(--brand);
  background: var(--brand);
  color: #fff;
}

.register-link:hover,
.register-link:focus-visible {
  border-color: #2d8cff;
  background: #2d8cff;
  outline: none;
  transform: translateY(-1px);
}

.account-link {
  max-width: 140px;
}

.account-link span {
  overflow: hidden;
  text-overflow: ellipsis;
}

.balance-link {
  display: flex;
  min-width: 204px;
  align-items: center;
  gap: 9px;
  padding: 5px 7px 5px 12px;
}

.balance-copy {
  display: grid;
  min-width: 66px;
  line-height: 1.1;
}

.balance-copy small {
  color: #919ba7;
  font-size: 10px;
  font-weight: 500;
}

.balance-copy strong {
  margin-top: 3px;
  color: #fff;
  font-size: 13px;
}

.recharge-action {
  display: inline-flex;
  min-height: 28px;
  align-items: center;
  gap: 4px;
  margin-left: auto;
  padding: 0 8px;
  border-radius: 4px;
  background: var(--brand);
  color: #fff;
  font-size: 12px;
}

.balance-link:hover,
.balance-link:focus-visible {
  border-color: var(--brand);
  background: #0c1724;
  outline: none;
}

.square-action {
  display: inline-grid;
  width: 38px;
  flex: none;
  place-items: center;
  padding: 0;
}

.mobile-toggle {
  width: 40px;
  height: 40px;
  place-items: center;
  border: 1px solid #343b44;
  border-radius: 5px;
  background: #0d1116;
  color: #fff;
  padding: 0;
  margin-left: auto;
}

.mobile-toggle svg {
  width: 20px;
}

.mobile-catalog {
  display: none;
}

.trust-bar {
  border-bottom: 1px solid var(--line);
  background: #fff;
}

.trust-inner {
  width: var(--container);
  min-height: 72px;
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  align-items: center;
}

.trust-item {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px 20px;
  border-left: 1px solid var(--line);
}

.trust-item:first-child {
  border-left: 0;
  padding-left: 0;
}

.trust-icon {
  width: 34px;
  height: 34px;
  display: grid;
  place-items: center;
  flex: none;
  border-radius: 50%;
  color: var(--brand);
  background: var(--brand-soft);
}

.trust-item strong {
  display: block;
  overflow: hidden;
  color: var(--ink);
  font-size: 13px;
  font-weight: 700;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.trust-item small {
  display: block;
  margin-top: 3px;
  overflow: hidden;
  color: var(--muted);
  font-size: 11px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.site-footer {
  border-top: 1px solid var(--line);
  background: #fff;
}

.footer-inner {
  width: var(--container);
  min-height: 76px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
}

.footer-brand {
  display: grid;
  gap: 4px;
  color: var(--muted);
  font-size: 12px;
}

.footer-title {
  color: var(--ink);
  font-size: 13px;
  font-weight: 700;
}

.footer-links {
  display: flex;
  align-items: center;
  gap: 18px;
  color: var(--muted);
  font-size: 13px;
  font-weight: 600;
}

.footer-links a:hover {
  color: var(--brand);
}

@media (max-width: 1180px) {
  .account-link {
    display: none;
  }

  .catalog-nav {
    display: none;
  }
}

@media (max-width: 900px) {
  .trust-item small {
    display: none;
  }

  .trust-item {
    padding: 12px 12px;
  }
}

@media (max-width: 760px) {
  .site-header {
    height: 60px;
  }

  .header-inner {
    width: var(--container);
  }

  .brand-mark {
    width: 33px;
    height: 33px;
    font-size: 19px;
  }

  .catalog-nav {
    display: none;
  }

  .main-nav {
    position: absolute;
    top: 60px;
    right: 0;
    left: 0;
    display: none;
    max-height: calc(100vh - 60px);
    align-items: stretch;
    flex-direction: column;
    gap: 10px;
    overflow-y: auto;
    padding: 14px;
    border-bottom: 1px solid #242b33;
    background: #050709;
    box-shadow: 0 22px 42px rgb(0 0 0 / 24%);
  }

  .main-nav.open {
    display: flex;
  }

  .mobile-catalog {
    display: grid;
    gap: 4px;
    padding-bottom: 8px;
    border-bottom: 1px solid #242b33;
  }

  .mobile-catalog .catalog-link {
    width: 100%;
    min-height: 44px;
    justify-content: flex-start;
    color: #eef2f7;
  }

  .mobile-catalog .catalog-link.is-active {
    color: #fff;
    background: #111820;
    box-shadow: inset 3px 0 0 var(--brand);
  }

  .nav-actions {
    height: auto;
    align-items: stretch;
    flex-direction: column;
    gap: 4px;
  }

  .query-control,
  .auth-link,
  .register-link,
  .account-link,
  .balance-link {
    display: flex;
    width: 100%;
    max-width: none;
    min-height: 44px;
    justify-content: flex-start;
  }

  .balance-link {
    min-height: 54px;
  }

  .square-action {
    display: flex;
    width: 100%;
    min-height: 44px;
    justify-content: flex-start;
    gap: 8px;
    padding: 0 14px;
  }

  .square-action[aria-label]::after {
    content: attr(aria-label);
  }

  .trust-inner {
    grid-template-columns: 1fr;
    min-height: 0;
    padding: 8px 0;
  }

  .trust-item {
    border-left: 0;
    border-top: 1px solid var(--line);
    padding: 12px 0;
  }

  .trust-item:first-child {
    border-top: 0;
  }

  .trust-item small {
    display: block;
  }

  .footer-inner {
    flex-direction: column;
    align-items: flex-start;
    padding: 20px 0;
  }

  .footer-links {
    flex-wrap: wrap;
    gap: 12px 16px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .main-nav a,
  .main-nav button,
  .catalog-link {
    transition: none;
  }
}
</style>
