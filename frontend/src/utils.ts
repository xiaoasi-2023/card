export const money = (value: number | string | undefined) => `¥${Number(value || 0).toFixed(2)}`
export const dateTime = (value?: string) => value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '-'
export const orderStatus: Record<string, string> = { pending_payment: '待支付', completed: '已完成', expired: '已过期', cancelled: '已取消', delivery_failed: '发卡异常', created: '已创建', success: '支付成功', failed: '失败', active: '启用', enabled: '启用', disabled: '停用', available: '可用', reserved: '已预留', sold: '已售出', void: '已作废' }
export const ledgerType: Record<string, string> = { admin_credit: '管理员充值', admin_debit: '管理员扣减', purchase: '购买扣款' }
export function idempotencyKey() { return crypto.randomUUID?.() || `${Date.now()}-${Math.random().toString(16).slice(2)}` }
