# 阿里云镜像构建与 Docker 部署

本项目只保留一条生产部署链路：

```text
提交源码到 Git
  -> 阿里云容器镜像服务 ACR 根据 Dockerfile 构建镜像
  -> 服务器 docker compose pull
  -> 服务器 docker compose up -d
```

服务器不构建前端或后端，不需要安装 Node.js 和 Go。

## 1. 阿里云 ACR 配置

在阿里云容器镜像服务中创建镜像仓库并绑定 Git 仓库：

```text
镜像地址：registry.cn-hangzhou.aliyuncs.com/jiangshitong/card
构建上下文：/
Dockerfile：/Dockerfile
镜像版本：latest
```

Dockerfile 会在阿里云构建环境中完成 Vue 生产构建和 Go Linux 二进制编译，不需要提交`frontend/dist`。

每次发布：

```bash
git add Dockerfile docker-compose.yml .dockerignore .env.example DEPLOY.md README.md backend frontend config.json
git commit -m "release card"
git push
```

Git 推送完成后，在 ACR 构建页面确认新镜像构建成功。

## 2. 服务器文件

服务器不需要完整源码，只需要以下内容：

```text
/www/docker/card/
  docker-compose.yml
  .env
  config.json
  data/
```

准备目录：

```bash
mkdir -p /www/docker/card/data
cd /www/docker/card
```

将仓库中的`docker-compose.yml`和`config.json`放到该目录，将`.env.example`复制为`.env`并填写真实配置。

`.env`至少需要正确配置：

```env
IMAGE_NAME=registry.cn-hangzhou.aliyuncs.com/jiangshitong/card:latest
APP_PORT=3000
APP_CONFIG_FILE=/www/docker/card/config.json
APP_DATA_DIR=/www/docker/card/data
APP_ENV=production
APP_BASE_URL=https://你的域名
```

同时填写全部 JWT、CDK、联系方式加密密钥、SMTP、管理员和支付参数。不要把服务器`.env`提交到 Git。

## 3. 登录阿里云镜像仓库

私有镜像仓库首次拉取前需要登录：

```bash
docker login --username=你的阿里云镜像仓库用户名 registry.cn-hangzhou.aliyuncs.com
```

登录密码使用阿里云容器镜像服务设置的固定密码。

## 4. 首次启动

```bash
cd /www/docker/card
docker compose pull

# 仅首次部署执行：初始化并替换平台商品目录
docker compose run --rm app seed-catalog --replace

docker compose up -d
docker compose ps
curl --fail http://127.0.0.1:${APP_PORT:-3000}/healthz
docker compose logs -f --tail=200 app
```

`seed-catalog --replace`会删除现有平台、商品、SKU、库存和订单关联数据。它只用于首次部署或明确要求重置目录时，正常更新不能执行。

## 5. 更新版本

阿里云新镜像构建成功后，在服务器执行：

```bash
cd /www/docker/card
docker compose pull
docker compose up -d
docker compose ps
curl --fail http://127.0.0.1:${APP_PORT:-3000}/healthz
```

查看和停止：

```bash
docker compose logs -f --tail=200 app
docker compose down
```

## 6. 域名与数据

使用 Nginx、Caddy 或宝塔将 HTTPS 域名反向代理到`127.0.0.1:APP_PORT`。支付回调基础地址必须与`APP_BASE_URL`一致。

SQLite 数据固定保存在`${APP_DATA_DIR}/card.db`。生产环境只能运行一个应用容器，并持续备份：

```text
/www/docker/card/data/
/www/docker/card/.env
/www/docker/card/config.json
```

当前内置`mock`支付仅供开发验收。真实在线支付上线前仍需按最终支付服务商文档接入下单接口和回调签名。
