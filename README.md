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
| 数据库 | PostgreSQL + Redis（**宿主机 / 宝塔**，非 compose 内置） |
| 部署 | Docker Compose 单应用容器 + 阿里云镜像 |

## Docker 部署方式（对齐 image2api）

```text
.env + docker-compose.yml
  → 拉取 IMAGE_NAME 镜像
  → 映射 APP_PORT:80
  → host.docker.internal 连宝塔 PostgreSQL / Redis
  → 挂载 config.json 与数据目录
```

| 文件 | 说明 |
|------|------|
| [`docker-compose.yml`](docker-compose.yml) | 单服务 `app` |
| [`.env.example`](.env.example) | 环境变量模板（复制为 `.env`） |
| [`config.json`](config.json) | 站点配置挂载 |
| [`docs/部署说明.md`](docs/部署说明.md) | 上线步骤 |

```bash
cp .env.example .env   # 改镜像、库连接、密钥、域名
docker compose up -d
```

## 状态

规划 + Docker 部署骨架已定。应用镜像与业务代码尚未实现；实施顺序见规划文档第 14 节。
