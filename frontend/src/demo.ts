import type { Platform, Product } from '@/types'
export const demoPlatforms: Platform[] = [
  { id: 1, name: 'CliProxy', slug: 'cliproxy' }, { id: 2, name: 'Kookeey', slug: 'kookeey' },
  { id: 3, name: 'B2Proxy', slug: 'b2proxy' }, { id: 4, name: '711Proxy', slug: '711proxy' },
  { id: 5, name: 'IPWEB', slug: 'ipweb' }, { id: 6, name: 'BunnyProxy', slug: 'bunnyproxy' },
  { id: 7, name: 'UdealProxy', slug: 'udealproxy' }
]
export const demoProducts: Product[] = [
  { id: 1, platform: demoPlatforms[0], name: '静态住宅代理 30 天', slug: 'static-residential-30d', description: '适合长期稳定业务场景，激活后有效期 30 天。', min_price: 68, stock_status: 'in_stock', skus: [{ id: 11, name: '1 个授权', price: 68, stock: 36 }, { id: 12, name: '5 个授权', price: 318, stock: 12 }] },
  { id: 2, platform: demoPlatforms[1], name: '全球动态流量包', slug: 'global-traffic', description: '多地区动态住宅网络流量额度。', min_price: 45, stock_status: 'in_stock', skus: [{ id: 21, name: '1 GB 流量', price: 45, stock: 80 }, { id: 22, name: '5 GB 流量', price: 199, stock: 22 }] },
  { id: 3, platform: demoPlatforms[3], name: '不限量住宅套餐', slug: 'residential-unlimited', description: '按周期使用的不限量住宅代理套餐。', min_price: 98, stock_status: 'in_stock', skus: [{ id: 31, name: '7 天套餐', price: 98, stock: 9 }] },
  { id: 4, platform: demoPlatforms[4], name: 'ISP 专线授权', slug: 'isp-dedicated', description: '稳定低延迟的 ISP 专线代理授权。', min_price: 128, stock_status: 'low_stock', skus: [{ id: 41, name: '30 天单授权', price: 128, stock: 3 }] }
]
