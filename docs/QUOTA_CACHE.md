# C1d：用户余额以数据库为准

共享 `CacheGetUserQuota` 保留调用接口，始终读取数据库已提交余额。
普通预扣不再先减 Redis；成功结算和 MJ 失败退款不再以缓存写入成功为条件。
已弃用的余额写入 helper 删除，旧 `user_quota:*` 数据不再参与本版本授权，无需清空 Redis。

数据库仍是失败关闭的边界。预扣仍执行账户、有限令牌的条件事务；Redis 故障不会
跳过数据库校验。数据库结算成功后，继续执行原有日志、渠道和用户统计。
本项不保证这些后续统计自身的数据库故障可恢复。

## 验证范围

`model/quota-cache_test.go` 使用临时 SQLite、Redis 命令 Hook、关闭的 Redis client 和本机模拟上游：

- 旧余额为零、负数、高值、非数字、溢出字符串、缺失；后续数据库变化立即可见。
- 令牌不足预留失败、Undo、零消费和正常消费，batch 开关均核对余额与后续合法预留。
- Redis 在预扣前或预扣后不可用，正常结算仍保留账户/令牌余额、消费日志、请求数与渠道用量。
- 数据库不可用时不能从旧缓存取得授权余额。
- MJ 正常失败退款在 Redis 可用和不可用时均执行；不声称已解决该后台路径的事务和幂等。

```sh
go test -mod=readonly -race -count=1 ./model -run '^TestQuota'
go test -mod=readonly -race -count=1 ./model ./relay/relay_util ./relay/task ./relay/midjourney ./controller
```

## 兼容与限制

Redis 仍可用于其他缓存、限流和 Realtime 在途使用量；本项只移除旧用户余额快照。
Realtime 每次余额读取现在查询数据库，但其完整预算执行仍待专项修复。
免费路径、有限/无限令牌及最终实际费用规则保持原行为；不新增 schema。
每次原缓存读取增加数据库查询，未做生产压测。混合部署旧实例仍有旧缓存错误，应统一升级。
本机命令 Hook 不等于真实 Redis 故障验收；三数据库事务专项也不替代全部业务路径多数据库测试。
回退会恢复缓存误拒、退款遗漏和结算审计遗漏风险。
