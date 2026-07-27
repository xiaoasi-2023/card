# Card 后端

单实例 Go 服务，使用 Gin、GORM 和 SQLite。注册用户可选择余额支付或在线支付，游客只能在线支付。

## 本地运行

```powershell
$env:APP_ENV = "development"
$env:DATABASE_PATH = ".\data\card.db"
$env:JWT_SECRET = "replace-this-jwt-secret"
$env:CARD_ENCRYPT_KEY = "replace-this-card-encryption-key"
$env:CARD_HASH_KEY = "replace-this-card-hash-key"
$env:CONTACT_ENCRYPT_KEY = "replace-this-contact-encryption-key"
$env:CONTACT_HASH_KEY = "replace-this-contact-hash-key"
$env:PAYMENT_PROVIDER = "mock"
$env:PAYMENT_MERCHANT_ID = "mock-merchant"
$env:PAYMENT_MERCHANT_KEY = "replace-this-payment-key"
$env:BOOTSTRAP_ADMIN_EMAIL = "admin@example.com"
$env:BOOTSTRAP_ADMIN_PASSWORD = "change-this-password"
go run .
```

默认监听`:3000`，健康检查为`GET /healthz`。设置`WEB_ROOT`后，服务会托管其中的前端静态文件并为前端路由回退到`index.html`。

## 支付模拟

开发环境且`PAYMENT_PROVIDER=mock`时，可调用：

```text
POST /api/v1/dev/payments/:order_no/pay
```

该接口模拟支付服务商成功通知，只在非生产环境注册。正式回调接口为：

```text
POST /api/v1/webhooks/payments/:provider
X-Payment-Signature: HMAC-SHA256 十六进制签名
```

签名原文按以下顺序用`|`拼接：

```text
merchant_id|order_no|trade_no|amount_cents|currency|timestamp
```

## 主要接口

- 公开：`/api/v1/public/platforms`、`/api/v1/public/products`
- 账号：`/api/v1/auth/register`、`/api/v1/auth/login`
- 会员：`/api/v1/me`、`/api/v1/me/orders`、`/api/v1/me/balance-ledgers`
- 游客：`/api/v1/guest/orders`、`/api/v1/guest/orders/query`
- 管理：`/api/v1/admin/users`、`platforms`、`products`、`skus`、`card-batches`、`cards`、`orders`、`payments`、`audit-logs`

金额均使用人民币分的整数。所有需要登录的接口使用`Authorization: Bearer <JWT>`。

## 测试

```powershell
go test ./...
```

测试覆盖余额扣款与发卡原子性、幂等购买、游客查单凭证、支付回调幂等、在线库存预留和过期释放、最后一张卡并发竞争。
