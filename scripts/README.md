# 验收脚本

`api_acceptance.py`会通过真实 HTTP 请求创建一组带时间戳的隔离测试数据，覆盖：

- 注册用户余额支付及重复请求幂等
- 注册用户在线支付与模拟支付成功
- 游客必填联系方式和本单查询密码
- 游客只能在线支付，支付前不得获取 CDK
- 游客错误凭证拒绝查单，正确凭证可在支付后查看 CDK
- 提现和退款接口不存在
- 可选的 SQLite 余额、订单、支付和敏感字段密文校验

先以开发环境和`mock`支付通道启动后端，再运行：

```bash
python scripts/api_acceptance.py \
  --base-url http://127.0.0.1:3000 \
  --admin-email admin@example.com \
  --admin-password 'AdminPass123!' \
  --database backend/data/acceptance.db
```

测试会写入数据但不会删除数据；建议使用专用临时数据库。生产环境不会提供`/api/v1/dev/payments/:order_no/pay`，因此不要对生产实例运行此脚本。
