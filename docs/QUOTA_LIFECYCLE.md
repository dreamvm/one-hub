# 请求内额度结算与退款

## C1b：同一 Quota 至多终结一次

共享 `Quota.Consume` 和 `Quota.Undo` 使用同一个线程安全终结保护。同一对象重复或并发调用，
只有第一个终局操作向模型层提交；正常非批量模式在方法返回前完成写入，批量模式只保证增量已入队。
零消费按“实际消费减去成功预扣”返还差额。无预扣的正常结算、无限令牌和免费模型仍可使用。
负的计算金额拒绝结算，避免异常用量变成超额退款。

Task 提交在整个重试过程中保留首次预留，成功结算或最终失败退款一次。
重试成功同步完成任务保存，并按首轮成功契约返回正文和成功指标。
Midjourney fast→relax 新预扣失败不再触发 defer 空指针；发送失败的显式退款与 defer 不会重复入账。
两种 Midjourney 入口、非收费 InPaint/CustomZoom 和 21/22 响应的原有收费规则保持。
通用 relay 每次尝试使用新 Quota，Recraft 保持一次预留到终局，两者无需重排。

## 兼容与故障边界

- 结算和退款不再由 Quota 自己启动后台 goroutine，数据库/日志/缓存操作可能增加调用方延迟；未压测。
  realtime 调用方自身仍有关闭 goroutine，本项不宣称清除了所有异步 Gin context 使用。
- 终结失败也不自动重放：用户与令牌写入可能部分成功，重放会重复记账。错误沿既有日志路径记录；
  不保证事务性、故障自动补偿或进程崩溃恢复。批量队列刷盘失败处理不在本项范围。
- 保护范围是同一个进程内 Quota 对象，不是跨请求、跨实例或后台任务退款幂等。
- Task/Recraft 重试继续采用首次计价与渠道归属；本次不改重试重新定价，也不改变后台任务只退用户的历史语义。
- Redis 余额预留补偿、批量尚未刷盘额度的授权可见性和结算时缓存刷新问题仍开放。
  本项不能据此证明 Redis 余额与模型账目同步，或证明有限令牌不再超支。
- 高账户余额跳过预扣、搜索先请求后检查和实时会话限额待后续修复；数据库原子预留、部分写入恢复属于 C2。
  完整 C1/C2 仍是正式版阻断项。

## 验证与回退

`model/quota-lifecycle_test.go` 使用临时 SQLite，在 Redis 命令 Hook 开/关、batch 开/关、
有限/无限令牌下核对账户余额、Token remain/used、请求数与日志数。测试导出 helper 只在 model 测试二进制中
同步执行既有批量 writer；断言是实际刷盘后模型账目，不把未刷盘旧数据误当成功。

`model/quota-callers_test.go` 使用本机 HTTP 模拟上游，覆盖首轮成功、重试成功、连续失败、
换渠道失败、不可重试，以及两个 Midjourney 入口的模式切换成功、预扣拒绝、网络失败、业务拒绝和合法收费对照。
并发终结测试验证单对象至多一次；故障注入明确展示部分写入仍需后续恢复，不能把它当成事务通过证明。

```sh
go test -race -count=1 ./model -run '^TestQuota'
go test -race -count=1 ./model ./relay/relay_util ./relay/task ./relay/midjourney ./controller ./middleware
go vet ./model ./relay/relay_util ./relay/task ./relay/midjourney
```

不调用真实模型或支付。专项使用 SQLite；通用 MySQL＋Redis smoke 不替代专项多数据库、Redis 故障或生产验证。
无 schema/API 格式变更。回退代码会恢复重复结算/退款、零消费预扣残留以及相关重试问题。
