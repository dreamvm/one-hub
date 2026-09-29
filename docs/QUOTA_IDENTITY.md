# C1e：预留绑定认证时的归属与额度模式

共享 Quota 将认证时保存的 token、owner、unlimited 模式交给
`PreConsumeTokenQuotaWithInfo`。账户条件扣减与令牌归属、模式核对处于同一个事务；
核对失败整笔回滚，仍返回 `pre_consume_token_quota_failed` / 403。
请求不能在认证后静默切换到另一个账户或另一种令牌计费模式。

预留成功后仍按该次保存的模式结算和退款。用户切换 unlimited、禁用或到期不应抹去
已经发生的消费或改变原预留的退款方式。旧 `PreConsumeTokenQuota` 作为无认证上下文的
兼容入口保留，读取当前快照后调用相同事务；生产 Quota 路径使用显式认证信息入口。

## 验证

- `TestQuotaIdentityRejectsChangedAuthentication`：有限/无限、batch 开关下，认证后切换
  模式或转移到另一个有余额账户，拒绝预扣且两个账户及令牌均不变；重新认证后可正常预留。
- `TestQuotaIdentitySettlementKeepsReservedMode`：成功预留后切换模式，Undo 与实际费用
  0 / 7 / 20 / 35 仍按原模式正确落账。
- `TestQuotaTransactionIdentity`：三个数据库共用夹具，覆盖调用前和事务窗口内变化；
  `TestQuotaTransactionIdentityModeControl` 保留预留后模式变化的正常结算。
- 新增 18 个 Quota 叶子用例和 24 个事务叶子用例；CI 的三数据库选择器包含后者。

```sh
go test -mod=readonly -race -count=1 ./model -run '^TestQuota(Identity|Transaction)'
```

## 尚未解决的边界

这不是持久预留凭据。成功预留后转移归属、删除令牌/用户，非零结算会按现有行为原子拒绝，
可能留下待恢复预留；后续持久幂等与恢复工作负责处理。差额为零不再次校验归属。
免费、零预扣、Realtime 直接结算及 Search 调用顺序仍待各自专项修复。
无 schema 改动；每次预留沿用账户先于令牌的锁顺序。回退会恢复前后身份不一致风险。
