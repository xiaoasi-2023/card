import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { authApi, meApi } from '@/api/services'
import type { User } from '@/types'

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  const token = ref(localStorage.getItem('cdk_token'))
  const loggedIn = computed(() => Boolean(token.value))
  const isAdmin = computed(() => user.value?.role === 'admin')
  function persist(nextToken: string, nextUser: User) {
    token.value = nextToken; user.value = nextUser
    localStorage.setItem('cdk_token', nextToken); localStorage.setItem('cdk_user', JSON.stringify(nextUser))
  }
  async function login(body: { username: string; password: string }) { const data = await authApi.login(body); persist(data.token, data.user) }
  async function restore() {
    const cached = localStorage.getItem('cdk_user')
    if (cached) { try { user.value = JSON.parse(cached) } catch {} }
    if (token.value) { try { user.value = await meApi.profile(); localStorage.setItem('cdk_user', JSON.stringify(user.value)) } catch { clear() } }
  }
  function clear() { token.value = null; user.value = null; localStorage.removeItem('cdk_token'); localStorage.removeItem('cdk_user') }
  async function logout() { try { await authApi.logout() } finally { clear() } }
  return { user, token, loggedIn, isAdmin, login, restore, logout, clear }
})
