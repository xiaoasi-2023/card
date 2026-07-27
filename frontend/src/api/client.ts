import axios from 'axios'

export const api = axios.create({ baseURL: import.meta.env.VITE_API_BASE_URL || '/api/v1', timeout: 15000 })

api.interceptors.request.use((config) => {
  const token = localStorage.getItem('cdk_token')
  if (token) config.headers.Authorization = `Bearer ${token}`
  return config
})

api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('cdk_token')
      localStorage.removeItem('cdk_user')
    }
    return Promise.reject(error)
  }
)

export function payload<T>(response: { data: any }): T {
  return (response.data?.data ?? response.data) as T
}

export function apiMessage(error: any, fallback = '请求失败，请稍后重试') {
  return error?.response?.data?.message || error?.response?.data?.error || fallback
}
