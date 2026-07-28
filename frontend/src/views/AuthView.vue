<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { LockKeyhole, Mail, ShieldCheck, UserRound } from 'lucide-vue-next'
import { authApi } from '@/api/services'
import { apiMessage } from '@/api/client'
import { useAuthStore } from '@/stores/auth'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const register = computed(() => route.path.endsWith('register'))
const alternateAuth = computed(() => ({
  path: register.value ? '/auth/login' : '/auth/register',
  query: route.query
}))

const username = ref('')
const email = ref('')
const verificationCode = ref('')
const password = ref('')
const confirm = ref('')
const loading = ref(false)
const codeLoading = ref(false)
const countdown = ref(0)
const error = ref('')
const codeMessage = ref('')
const verificationInput = ref<HTMLInputElement>()
const normalizedEmail = computed(() => email.value.trim().toLowerCase())
const isQQEmail = computed(() => /^[^@\s]+@qq\.com$/i.test(normalizedEmail.value))
let countdownTimer: ReturnType<typeof setInterval> | undefined

function stopCountdown() {
  if (countdownTimer) clearInterval(countdownTimer)
  countdownTimer = undefined
}

function startCountdown() {
  stopCountdown()
  countdown.value = 60
  countdownTimer = setInterval(() => {
    countdown.value -= 1
    if (countdown.value <= 0) stopCountdown()
  }, 1000)
}

function normalizeVerificationCode(event: Event) {
  const input = event.target as HTMLInputElement
  const normalized = input.value.replace(/\D/g, '').slice(0, 6)
  input.value = normalized
  verificationCode.value = normalized
}

async function sendRegistrationCode() {
  error.value = ''
  codeMessage.value = ''
  if (!isQQEmail.value) {
    error.value = '请输入有效的 QQ 邮箱（以 @qq.com 结尾）'
    return
  }

  codeLoading.value = true
  try {
    await authApi.registrationCode({ email: normalizedEmail.value })
    codeMessage.value = '验证码已发送，请在 10 分钟内完成注册'
    startCountdown()
    await nextTick()
    verificationInput.value?.focus()
  } catch (e) {
    error.value = apiMessage(e, '验证码发送失败，请稍后重试')
  } finally {
    codeLoading.value = false
  }
}

async function submit() {
  if (register.value) {
    if (!isQQEmail.value) {
      error.value = '注册仅支持 QQ 邮箱（以 @qq.com 结尾）'
      return
    }
    if (!/^\d{6}$/.test(verificationCode.value)) {
      error.value = '请输入邮件中的 6 位验证码'
      return
    }
    if (password.value !== confirm.value) {
      error.value = '两次输入的密码不一致'
      return
    }
  }

  loading.value = true
  error.value = ''
  try {
    if (register.value) {
      await authApi.register({
        username: username.value,
        email: normalizedEmail.value,
        password: password.value,
        verification_code: verificationCode.value
      })
      await auth.login({ username: username.value, password: password.value })
    } else {
      await auth.login({ username: username.value, password: password.value })
    }

    const target = String(route.query.redirect || '/me')
    if (target === '/checkout/account') {
      await router.push({
        path: target,
        query: {
          product_id: route.query.product_id,
          product_slug: route.query.product_slug,
          sku_id: route.query.sku_id,
          quantity: route.query.quantity
        }
      })
    } else {
      await router.push(target)
    }
  } catch (e) {
    error.value = apiMessage(e, register.value ? '注册失败' : '账号或密码错误')
  } finally {
    loading.value = false
  }
}

onBeforeUnmount(stopCountdown)
</script>

<template>
  <section class="auth-page">
    <div class="auth-box" :class="{ 'auth-box--register': register }">
      <header class="auth-head">
        <RouterLink class="brand auth-brand" to="/" aria-label="返回阿巳首页">
          <span class="brand-mark">巳</span>
          <span>阿巳</span>
        </RouterLink>
        <div class="auth-heading">
          <span class="auth-kicker">{{ register ? 'REGISTER' : 'MEMBER ACCESS' }}</span>
          <h1>{{ register ? '创建账号' : '欢迎回来' }}</h1>
          <p>{{ register ? 'QQ 邮箱验证后即可使用余额或在线支付' : '登录查看余额、订单与已购 CDK' }}</p>
        </div>
      </header>

      <form class="auth-form" :class="{ 'auth-form--register': register }" @submit.prevent="submit">
        <label class="form-field auth-field auth-field--username">
          <span class="field-label">用户名</span>
          <div class="input-with-leading">
            <UserRound aria-hidden="true" />
            <input v-model.trim="username" autocomplete="username" placeholder="输入用户名或登录账号" required />
          </div>
        </label>

        <div v-if="register" class="form-field auth-field auth-field--email">
          <label class="field-label" for="registration-email">
            <span>QQ 邮箱</span>
            <small>仅支持 @qq.com</small>
          </label>
          <div class="input-with-leading">
            <Mail aria-hidden="true" />
            <input
              id="registration-email"
              v-model.trim="email"
              type="email"
              inputmode="email"
              autocomplete="email"
              pattern="[^@\s]+@qq\.com"
              placeholder="例如 123456789@qq.com"
              aria-describedby="email-hint"
              required
            />
          </div>
          <span id="email-hint" class="sr-only">目前仅支持 @qq.com 邮箱注册</span>
        </div>

        <div v-if="register" class="form-field auth-field auth-field--verification">
          <label class="field-label" for="verification-code">
            <span>邮箱验证码</span>
            <small>有效期 10 分钟</small>
          </label>
          <div class="verification-row">
            <div class="input-with-leading">
              <ShieldCheck aria-hidden="true" />
              <input
                id="verification-code"
                ref="verificationInput"
                :value="verificationCode"
                type="text"
                inputmode="numeric"
                autocomplete="one-time-code"
                pattern="\d{6}"
                maxlength="6"
                placeholder="6 位验证码"
                required
                @input="normalizeVerificationCode"
              />
            </div>
            <button
              type="button"
              class="btn btn--secondary code-button"
              :disabled="codeLoading || loading || countdown > 0 || !isQQEmail"
              :aria-label="countdown > 0 ? `${countdown} 秒后可重新发送验证码` : '发送邮箱验证码'"
              @click="sendRegistrationCode"
            >
              {{ codeLoading ? '发送中' : countdown > 0 ? `${countdown} 秒` : '发送验证码' }}
            </button>
          </div>
          <small v-if="codeMessage" class="field-hint field-hint--success" role="status" aria-live="polite">
            {{ codeMessage }}
          </small>
        </div>

        <label class="form-field auth-field auth-field--password">
          <span class="field-label">密码</span>
          <div class="input-with-leading">
            <LockKeyhole aria-hidden="true" />
            <input
              v-model="password"
              type="password"
              :autocomplete="register ? 'new-password' : 'current-password'"
              placeholder="请输入至少 8 位密码"
              minlength="8"
              required
            />
          </div>
        </label>

        <label v-if="register" class="form-field auth-field auth-field--confirm">
          <span class="field-label">确认密码</span>
          <div class="input-with-leading">
            <LockKeyhole aria-hidden="true" />
            <input
              v-model="confirm"
              type="password"
              autocomplete="new-password"
              placeholder="请再次输入密码"
              required
            />
          </div>
        </label>

        <div v-if="error" class="alert alert--error" role="alert" aria-live="assertive">{{ error }}</div>
        <button class="btn btn--primary btn--wide auth-submit" :disabled="loading || codeLoading">
          {{ loading ? '正在提交' : register ? '注册并登录' : '登录' }}
        </button>
      </form>

      <footer class="auth-links">
        <p class="auth-switch">
          {{ register ? '已有账号？' : '还没有账号？' }}
          <RouterLink :to="alternateAuth">{{ register ? '直接登录' : '免费注册' }}</RouterLink>
        </p>
        <span class="auth-link-divider" aria-hidden="true"></span>
        <RouterLink class="guest-link" to="/?catalog=all">游客购买</RouterLink>
      </footer>
    </div>
  </section>
</template>

<style scoped>
.auth-page {
  --auth-accent: var(--brand);
  --auth-ink: var(--ink);
  min-height: calc(100vh - 180px);
  padding: clamp(22px, 4vh, 38px) 18px;
  background: var(--canvas);
}

.auth-box {
  width: min(500px, 100%);
  padding: 28px 30px 24px;
  border: 1px solid var(--line);
  border-top: 3px solid var(--nav);
  border-radius: 8px;
  box-shadow: var(--shadow);
  background: #fff;
  animation: auth-enter 420ms cubic-bezier(0.22, 1, 0.36, 1) both;
}

.auth-box--register {
  width: min(760px, 100%);
}

.auth-head {
  display: grid;
  grid-template-columns: 148px minmax(0, 1fr);
  align-items: center;
  gap: 26px;
  margin-bottom: 22px;
  padding-bottom: 20px;
  border-bottom: 1px solid #e4e9eb;
}

.auth-brand {
  justify-content: flex-start;
  margin: 0;
  color: var(--auth-ink);
  font-size: 21px;
}

.auth-brand .brand-mark {
  width: 38px;
  height: 38px;
  background: var(--auth-accent);
  font-family: "Microsoft YaHei", sans-serif;
  font-size: 20px;
}

.auth-heading {
  min-width: 0;
}

.auth-kicker {
  display: block;
  margin-bottom: 5px;
  color: var(--brand);
  font-family: Consolas, "SFMono-Regular", monospace;
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0;
}

.auth-heading h1 {
  margin: 0 0 4px;
  color: var(--auth-ink);
  font-size: 24px;
  line-height: 1.25;
  letter-spacing: 0;
}

.auth-heading p {
  margin: 0;
  color: #6b7780;
  font-size: 13px;
  line-height: 1.55;
}

.auth-form {
  display: grid;
}

.auth-form--register {
  grid-template-columns: repeat(2, minmax(0, 1fr));
  column-gap: 16px;
}

.auth-field {
  gap: 7px;
  margin-bottom: 14px;
}

.auth-field--verification,
.auth-form .alert,
.auth-submit {
  grid-column: 1 / -1;
}

.field-label {
  min-height: 18px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  color: #26323a;
  font-size: 12px;
  font-weight: 700;
}

.field-label small {
  color: #7a858d;
  font-size: 10px;
  font-weight: 500;
}

.input-with-leading {
  min-height: 44px;
  border-color: #cbd4d8;
  border-radius: 6px;
  background: #fbfcfc;
}

.input-with-leading:focus-within {
  border-color: var(--brand);
  box-shadow: 0 0 0 3px rgba(8, 120, 255, 0.12);
  background: #fff;
}

.input-with-leading input {
  min-height: 42px;
  padding-block: 9px;
  background: transparent;
}

.input-with-leading > svg {
  width: 17px;
  margin-left: 13px;
  color: #75838a;
}

.field-hint {
  margin-top: -2px;
  color: var(--muted);
  font-size: 11px;
  line-height: 1.4;
}

.field-hint--success {
  color: var(--auth-accent);
}

.verification-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 132px;
  gap: 10px;
}

.code-button {
  min-height: 44px;
  border-radius: 6px;
  padding-inline: 14px;
  color: #26343b;
  white-space: nowrap;
}

.auth-form .alert {
  margin: 0 0 14px;
}

.auth-submit {
  min-height: 44px;
  border-radius: 6px;
  background: var(--auth-accent);
  color: #fff;
}

.auth-submit:hover {
  background: var(--brand-strong);
}

.auth-links {
  min-height: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 14px;
  margin-top: 18px;
  color: #6d7880;
  font-size: 12px;
}

.auth-switch {
  margin: 0 !important;
  color: inherit !important;
  font-size: inherit !important;
}

.auth-switch a,
.guest-link {
  color: var(--auth-accent);
  font-weight: 700;
}

.auth-link-divider {
  width: 1px;
  height: 13px;
  background: #d4dade;
}

.guest-link {
  display: inline;
  text-align: left;
  font-size: inherit;
}

.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
}

@keyframes auth-enter {
  from {
    opacity: 0;
    transform: translateY(12px) scale(0.99);
  }

  to {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
}

@media (max-width: 680px) {
  .auth-page {
    min-height: calc(100vh - 130px);
    padding: 18px 14px;
    place-items: start center;
  }

  .auth-box,
  .auth-box--register {
    width: min(500px, 100%);
    padding: 24px 20px 22px;
  }

  .auth-head {
    grid-template-columns: auto minmax(0, 1fr);
    gap: 18px;
    margin-bottom: 20px;
    padding-bottom: 18px;
  }

  .auth-brand > span:last-child {
    display: none;
  }

  .auth-heading h1 {
    font-size: 22px;
  }

  .auth-form--register {
    grid-template-columns: 1fr;
  }

  .auth-field--verification,
  .auth-form .alert,
  .auth-submit {
    grid-column: auto;
  }
}

@media (max-width: 430px) {
  .verification-row {
    grid-template-columns: minmax(0, 1fr) 108px;
    gap: 8px;
  }

  .code-button {
    padding-inline: 8px;
    font-size: 13px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .auth-box {
    animation: none;
  }
}
</style>
