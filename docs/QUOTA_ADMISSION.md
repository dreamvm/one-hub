# C1c：高账户余额仍须预扣

`Quota.PreQuotaConsumption` 不再因账户余额超过预估预扣额的 100 倍而清零预扣。
所有通过用户额度检查的正数预扣都进入 C2a 的账户与令牌条件事务；有限令牌额度不足会返回
既有 `pre_consume_token_quota_failed` / 403，不设置成功预留标记。

## 正常行为与验证

- 高余额不代表有限令牌额度充足。有限令牌恰好足够时正常预留；无限令牌仍扣账户，令牌额度字段不变。
- 成功请求从“直接结算”变为“预扣后按差额结算”；相同实际用量的最终费用不变。
  零消费、退款、重复终结继续使用 C1b 保护；batch 模式余额由 C2a 立即落库。
- 按 token 和按次计价的正数路径均受检查；免费/零预扣保持原有行为。
- `model/quota-high-balance_test.go` 使用临时 SQLite、Redis 命令 Hook 和本机 HTTP 服务。
  覆盖 Redis / batch 开关、旧阈值上下两侧、高估的缓存余额、有限/无限预留、正常最终费用、
  并发小令牌预算，以及 Task、MJ imagine、MJ swap-face 在拒绝时不上游提交。
- 三数据库专项仍验证 C2a 模型事务；新增高余额调用链测试使用 SQLite，不能将前者称作后者的三数据库实测。

```sh
go test -mod=readonly -race -count=1 -timeout=120s ./model -run '^TestQuotaHighBalance'
go test -mod=readonly -race -count=1 ./model ./relay/relay_util ./relay/task ./relay/midjourney
```

## 范围与剩余工作

本项只移除高余额豁免，不是完整请求预算上限，也不证明 Redis 与数据库同步：

- C1d 已将用户额度读取改为数据库，并移除旧余额缓存写入门槛，见 [QUOTA_CACHE.md](QUOTA_CACHE.md)。
  Realtime 在途预算累计与持久恢复仍是独立未完成项。
- 认证、预扣、结算期间 owner / unlimited 模式变化，仍需真实预留身份信息解决。
- Search 先调用上游再检查、Realtime 不调用此预扣方法、零/负预扣或极小价格截断、
  实际消费超过预估、持久幂等与故障恢复、后台补偿仍开放。完整 C1/C2 继续阻断正式版。

无 schema 或 API 格式变更。高余额请求现在增加实际预扣与差额结算数据库写入，未做生产压测。
升级仍须遵守 [QUOTA_TRANSACTIONS.md](QUOTA_TRANSACTIONS.md) 的旧队列落账要求；
回退会重新允许高账户余额绕过有限令牌预扣。本轮不发布或部署。
