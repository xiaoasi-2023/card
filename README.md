# 代理卡密销售平台（卡网）

对标 [888proxy](https://www.888proxy.com/) 的自动化卡密商城：自备卡密池发卡、多级子代理、邀请下单返利（不可提现）。

## 文档

- **[完整规划文档](docs/规划文档.md)** — 产品范围、多级代理、邀请返利、数据模型、API、Docker、分期路线

## 关键能力

- 七大代理平台 CDK 卡密池自动发货（CliProxy / Kookeey / B2Proxy / 711Proxy / IPWEB / BunnyProxy / UdealProxy）
- 多级子代理开店、差价佣金（可提现）
- 邀请人下单返利 ¥0.5 / ¥1，仅入账站内返利余额，不可提现
- 充值余额 / 返利余额 / 代理佣金三账分离
- Docker Compose 部署

## 技术栈（规划）

| 层 | 选型 |
|----|------|
| 后端 | Go（Gin/Fiber） |
| 前端 | Vue 3 + Vite |
| 数据库 | PostgreSQL + Redis |
| 部署 | Docker Compose |

## 状态

规划已定稿，尚未开始编码。实施顺序见规划文档第 14 节。
