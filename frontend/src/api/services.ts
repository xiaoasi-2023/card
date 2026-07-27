import { api, payload } from './client'
import type { Ledger, Order, Paginated, Payment, Platform, Product, Sku, User } from '@/types'

const fromCents = (value: unknown) => Number(value || 0) / 100
const toCents = (value: unknown) => Math.round(Number(value || 0) * 100)
const rawRows = <T>(value: any): T[] => Array.isArray(value) ? value : value?.items || []
const platform = (v: any): Platform => ({ ...v, slug: v.slug || v.code, enabled: v.enabled ?? v.status === 'active' })
const sku = (v: any): Sku => ({ ...v, price: v.price ?? fromCents(v.sale_price_cents), enabled: v.enabled ?? v.status === 'active' })
const productBase = (v: any): Product => ({ ...v, platform: v.platform ? platform(v.platform) : undefined, enabled: v.enabled ?? v.status === 'active' })
const productDetail = (v: any): Product => {
  const base = v?.product ? productBase(v.product) : productBase(v)
  const skus = rawRows<any>(v?.skus || v?.product?.skus || base.skus).map(sku)
  return { ...base, skus, min_price: skus.length ? Math.min(...skus.map(s => Number(s.price))) : base.min_price }
}
const user = (v: any): User => ({ ...v, balance: v.balance ?? fromCents(v.balance_cents), status: v.status === 'active' ? 'enabled' : v.status })
const ledger = (v: any): Ledger => ({ ...v, amount: v.amount ?? (v.direction === 'out' ? -fromCents(v.amount_cents) : fromCents(v.amount_cents)), balance_after: v.balance_after ?? fromCents(v.balance_after_cents) })
const payment = (v: any): Payment => ({ ...v, order_no: v.order_no || v.merchant_order_no, amount: v.amount ?? fromCents(v.amount_cents) })
const order = (v: any): Order => {
  const source = v?.order || v
  const item = v?.item || {}
  const pay = v?.payment
  return {
    ...source,
    order_no: source.order_no,
    product_name: source.product_name || item.product_name,
    sku_name: source.sku_name || item.sku_name,
    quantity: source.quantity || item.quantity || 1,
    unit_price: source.unit_price ?? fromCents(item.unit_price_cents),
    total_amount: source.total_amount ?? fromCents(source.total_amount_cents),
    payment_method: source.payment_method,
    payment_url: source.payment_url || pay?.pay_url,
    cards: v?.cards || source.cards || [],
    user: v?.user ? user(v.user) : source.user,
    contact_masked: v?.contact_masked || source.contact_masked,
    contact: v?.contact || source.contact
  }
}

export const publicApi = {
  platforms: async () => rawRows<any>(payload<any>(await api.get('/public/platforms'))).map(platform),
  products: async (params?: Record<string, unknown>) => {
    const response = payload<any>(await api.get('/public/products', { params }))
    const list = rawRows<any>(response).map(productBase)
    const enriched = await Promise.all(list.map(async p =>
      productDetail(payload<any>(await api.get(`/public/products/${p.slug}`)))
    ))
    return { items: enriched, total: response?.total, page: response?.page, page_size: response?.page_size } as Paginated<Product>
  },
  product: async (slug: string) => productDetail(payload<any>(await api.get(`/public/products/${slug}`)))
}
export const authApi = {
  login: async (body: { username: string; password: string }) => { const v = payload<any>(await api.post('/auth/login', { login: body.username, password: body.password })); return { token: v.token, user: user(v.user) } },
  registrationCode: async (body: { email: string }) => payload<any>(await api.post('/auth/registration-codes', body)),
  register: async (body: { username: string; email: string; password: string; verification_code: string }) => { const v = payload<any>(await api.post('/auth/register', body)); return { token: v.token, user: v.user ? user(v.user) : undefined } },
  logout: async () => api.post('/auth/logout')
}
export const meApi = {
  profile: async () => user(payload<any>(await api.get('/me'))),
  ledgers: async (params?: object) => ({ items: rawRows<any>(payload<any>(await api.get('/me/balance-ledgers', { params }))).map(ledger) }),
  orders: async (params?: object) => ({ items: rawRows<any>(payload<any>(await api.get('/me/orders', { params }))).map(order) }),
  order: async (orderNo: string) => order(payload<any>(await api.get(`/me/orders/${orderNo}`))),
  createOrder: async (body: object) => order(payload<any>(await api.post('/me/orders', body))),
  password: async (body: object) => api.put('/me/password', body)
}
export const guestApi = {
  createOrder: async (body: object) => order(payload<any>(await api.post('/guest/orders', body))),
  query: async (body: object) => order(payload<any>(await api.post('/guest/orders/query', body)))
}
export const paymentApi = {
  mockPay: async (orderNo: string) => order(payload<any>(await api.post(`/dev/payments/${orderNo}/pay`)))
}

function adminBody(path: string, body: any) {
  if (path === 'platforms') return { ...body, code: body.code || body.slug, status: body.enabled === false ? 'disabled' : 'active' }
  if (path === 'products') return { ...body, status: body.enabled === false ? 'disabled' : 'active' }
  if (path === 'skus') return { ...body, sale_price_cents: body.sale_price_cents ?? toCents(body.price), status: body.enabled === false ? 'disabled' : 'active' }
  return body
}
function adminRows<T>(path: string, value: any): T[] {
  const list = rawRows<any>(value)
  if (path === 'users') return list.map(user) as T[]
  if (path === 'platforms') return list.map(platform) as T[]
  if (path === 'products') return list.map(productBase) as T[]
  if (path === 'skus') return list.map(sku) as T[]
  if (path === 'orders') return list.map(order) as T[]
  if (path === 'payments') return list.map(payment) as T[]
  return list
}
export const adminApi = {
  list: async <T>(path: string, params?: object) => ({ items: adminRows<T>(path, payload<any>(await api.get(`/admin/${path}`, { params }))) }),
  create: async <T>(path: string, body: object) => payload<T>(await api.post(`/admin/${path}`, adminBody(path, body))),
  update: async <T>(path: string, id: number, body: object) => payload<T>(await api.put(`/admin/${path}/${id}`, adminBody(path, body))),
  remove: async (path: string, id: number) => api.delete(`/admin/${path}/${id}`),
  adjustBalance: async (id: number, body: any) => payload<any>(await api.post(`/admin/users/${id}/balance-adjustments`, { direction: body.direction, amount_cents: toCents(body.amount), reason: body.reason, idempotency_key: body.idempotency_key })),
  users: (params?: object) => adminApi.list<User>('users', params),
  platforms: (params?: object) => adminApi.list<Platform>('platforms', params),
  products: (params?: object) => adminApi.list<Product>('products', params),
  skus: (params?: object) => adminApi.list<Sku>('skus', params),
  orders: (params?: object) => adminApi.list<Order>('orders', params),
  order: async (orderNo: string) => order(payload<any>(await api.get(`/admin/orders/${orderNo}`))),
  payments: (params?: object) => adminApi.list<Payment>('payments', params)
}

export function rows<T>(value: Paginated<T> | T[] | undefined): T[] { return Array.isArray(value) ? value : value?.items || [] }
