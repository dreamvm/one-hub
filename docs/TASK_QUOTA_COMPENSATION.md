# 异步任务失败补偿

本批覆盖 Suno、Kling、Midjourney 的可信轮询失败路径，依赖额度持久账本（PR #35）。

## 账务与重试

- 新任务保存内部 `reservation_id`，不在用户响应中公开。任务的估算 `quota` 不作为退款依据。
- 每次 Suno 提交重试先释放前一次预留，再按实际渠道和价格建立新预留。成功任务关联最后一次预留，避免跨渠道重试后收据错配。
- 首次任务终态与收据 `compensation_state=pending` 在同一事务提交。另一个事务将实际 `final_quota` 退回原用户和有限令牌，同时写补偿日志并设为 `applied`。
- 使用收据记录的原始有限/无限模式；管理员后来改变模式不改变已发生的账务。无限令牌只退用户额度，免费任务不会凭估算值获得额度。
- 收据原始 `state`/`outcome` 不被补偿改写。若原请求已撤销预留，补偿记为 `not_charged`，不再次加钱；尚未结算时保持 pending。
- 账户/令牌/日志/补偿状态任一写失败，整笔补偿回滚。独立恢复 worker 每 15 秒按批重试持久意图，不依赖任务继续被轮询。
- 相同收据只能补偿一次，包括多个进程、重复响应和重复本地任务行。已完成任务不会被旧进度或批量故障更新重新打开。
- 保留既有累计请求和使用统计；补偿另记系统日志，不回写旧消费日志或重算历史统计。余额与这些累计统计的含义不同。

## 轮询绑定

Suno/Kling 按平台、渠道、外部编号和本地行绑定；相同编号在其他渠道或同渠道的多个本地行不会被 map 覆盖。
Suno 拒绝批次外编号和空/异常响应；Kling 验证上游实际 task_id，只将 failed 状态的说明视为失败原因。
Kling 工厂此前未注册、轮询又使用与落库不同的大小写平台名；本批补齐实际渠道注册，并直接使用已绑定的任务动作。
MJ 保留已有整批身份校验和通知仅唤醒轮询的边界。已有的一小时超时失败策略保持；本批不改变该业务退款规则。

## 旧任务与待核对状态

`refund_status=requested` 只表示已保存补偿意图，不表示退款已经成功。最终状态在对应收据的 `compensation_state`。
`refund_status=review_required` 表示历史任务缺少收据，或任务身份与收据不匹配，不能证明该笔实际扣款与原模式。
旧任务仍会同步到失败终态，但不猜测退款，也不自动重放旧版本可能已经退过的钱。
原令牌被删除/转移、余额加法超出整数范围、原请求仍只有预留而无确定终局时，补偿保持 pending，需要核对。
不提供自动历史回填、管理员强制退款接口或按超时时间猜测原扣款的机制。

只读核查示例（表名前缀需按实际配置调整）：

```sql
SELECT id, user_id, token_id, channel_id, final_quota, state, compensation_state,
       compensation_attempt_at
FROM quota_reservations WHERE compensation_state = 'pending';
SELECT id, task_id, platform, channel_id, refund_status
FROM tasks WHERE refund_status = 'review_required';
SELECT id, mj_id, channel_id, refund_status
FROM midjourneys WHERE refund_status = 'review_required';
```

## 升级与回退

变更仅增加收据补偿字段与任务关联/状态字段，由启动迁移创建。切换前停止旧写入和轮询进程并备份数据库；
不能混跑会绕过账本的旧退款 worker。回退旧代码会失去成对、幂等退款和恢复保证，并可能再次退款，
因此必须停止旧轮询、核对 pending/applied 状态后制定回退操作。普通镜像回退 smoke 不证明混跑安全。
本批没有发布镜像、修改生产数据、调用真实付费上游或自动补发历史退款。
