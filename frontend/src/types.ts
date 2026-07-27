export interface Platform { id: number; name: string; slug: string; website?: string; enabled?: boolean; product_count?: number }
export interface Sku { id: number; name: string; price: number | string; stock?: number; stock_status?: string; enabled?: boolean }
export interface Product { id: number; platform_id?: number; platform?: Platform; name: string; slug: string; description?: string; enabled?: boolean; skus?: Sku[]; min_price?: number | string; stock_status?: string; sort?: number; created_at?: string }
export interface User { id: number; username: string; role?: 'user' | 'admin'; status?: string; balance?: number | string; created_at?: string }
export interface Order { id?: number; order_no: string; product_name?: string; sku_name?: string; quantity: number; unit_price?: number | string; total_amount: number | string; payment_method: 'balance' | 'online'; status: string; created_at?: string; expires_at?: string; payment_url?: string; cards?: string[]; contact?: string; contact_masked?: string; user?: User }
export interface Ledger { id: number; type: string; amount: number | string; balance_after: number | string; reason?: string; created_at: string }
export interface Payment { id: number; order_no?: string; provider?: string; provider_trade_no?: string; amount: number | string; status: string; callback_status?: string; created_at?: string }
export interface Paginated<T> { items: T[]; total?: number; page?: number; page_size?: number }
