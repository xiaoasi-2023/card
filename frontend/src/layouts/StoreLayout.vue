<script setup lang="ts">
import { nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { RouterLink, RouterView, useRoute, useRouter } from 'vue-router'
import {
  CirclePlus,
  LogOut,
  Menu,
  Search,
  ShieldCheck,
  UserRound,
  WalletCards,
  X,
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
      .from('.header-action', { autoAlpha: 0, x: 12, duration: 0.34, stagger: 0.045 }, '-=0.26')
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

        <button
          class="mobile-toggle mobile-only"
          type="button"
          aria-controls="store-navigation"
          :aria-expanded="open"
          :aria-label="open ? '关闭账户菜单' : '打开账户菜单'"
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

    <main><RouterView /></main>

    <footer class="site-footer">
      <div class="container footer-inner">
        <span>阿巳 · 多平台代理 CDK</span>
        <span>数字商品交付后不支持退款</span>
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
  width: min(1420px, calc(100% - 48px));
  gap: 36px;
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
  background: #0878ff;
  color: #020911;
  box-shadow: 0 0 0 1px rgb(255 255 255 / 7%) inset;
  font-size: 21px;
  font-weight: 900;
}

.brand-name {
  white-space: nowrap;
}

.main-nav {
  flex: 1;
  min-width: 0;
  justify-content: flex-end;
  gap: 28px;
}

.nav-actions {
  display: flex;
  align-items: center;
}

.nav-actions {
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
  border-color: #0878ff;
  background: #0878ff;
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
  background: #0878ff;
  color: #fff;
  font-size: 12px;
}

.balance-link:hover,
.balance-link:focus-visible {
  border-color: #0878ff;
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
}

.mobile-toggle svg {
  width: 20px;
}

@media (max-width: 1180px) {
  .account-link {
    display: none;
  }
}

@media (max-width: 760px) {
  .site-header {
    height: 60px;
  }

  .header-inner {
    width: min(100% - 28px, 1180px);
  }

  .brand-mark {
    width: 33px;
    height: 33px;
    font-size: 19px;
  }

  .main-nav {
    position: absolute;
    top: 60px;
    right: 0;
    left: 0;
    display: none;
    max-height: calc(100vh - 60px);
    align-items: stretch;
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
}

@media (prefers-reduced-motion: reduce) {
  .main-nav a,
  .main-nav button {
    transition: none;
  }
}
</style>
