# 请求中途 Redis 故障验收

分支 `codex/redis-inflight-acceptance` 基于 PR #82 已合并 main
`4ba29b48f9629512ce7170a509a3383739387b07`。只扩展隔离 smoke，不修改应用逻辑。
现有 PR #47/#48 已覆盖请求前 Redis 停止、错误类型、过期额度缓存和恢复；本项补充已接纳请求的中途故障。

## 场景与边界

模拟上游最多暂停一个请求，8 秒后自动释放；支持取消和明确释放，不接受外部目标或任意队列。
JSON 在返回业务响应前暂停；SSE 在模拟上游刷出首个内容块后、终局用量前暂停。
这证明上游已输出，不能从它推断客户端在故障瞬间已收到该数据块。

在 MySQL+Redis、PostgreSQL+Redis 隔离镜像中，分别使用无限与有限额度令牌执行两类请求：

1. 正常请求先取得对照费用。确认暂停请求已有归属明确的 `reserved` 收据，用户与有限令牌预留一致，尚无终局统计。
2. 仅停止当前测试创建的 Redis 容器，然后释放模拟上游。核对中文响应、终局用量和一次上游调用。
3. 核对消费收据、用户余额/用量/请求数、令牌余额/用量、消费日志与渠道统计；费用须与正常控制相同。
4. `finally` 恢复 Redis，确认空缓存启动后正常请求仍只结算一次；有限令牌通过合成账号 API 创建和删除。

沿用现有内部 Docker 网络、随机运行名称、临时数据库、资源限制和最终清理。
不访问生产、真实用户、真实模型或外部故障目标。数据库中途故障、网络分区、进程崩溃和并发负载仍开放。

## 验证状态

新增夹具测试覆盖 JSON/SSE 的释放、取消、单槽上限、超时及后续正常请求；账务断言有正常与错误终局/重复日志等负向控制。
本地 Go1.25.14 夹具和策略 race、相关 vet、21 项 Python 离线测试通过。
真实两数据库镜像验收等待准确候选 CI。
尚未取得这些新场景的端到端通过记录；不能用 PR #82 的 CI 替代本候选。
本项没有应用安全修复，也未另行执行安全修复的独立审阅；父任务检查测试边界与账务路径，候选 CI 仍为必需门槛。

## 合并验收

[PR #83](https://github.com/dreamvm/one-hub/pull/83) head
`6ab65ba2ea7f56d5842112fa5714898012137ea9` 已合并为
`53be6eb04942814f812d756dfbd809bd553e1b66`。候选、GitHub 合成、本地计算和实际合并树
均为 `74f9bcb63e7731298121daabeb8e78d0f5809ed7`。
[36831550015](https://github.com/dreamvm/one-hub/actions/runs/36831550015) /
[36831550324](https://github.com/dreamvm/one-hub/actions/runs/36831550324) 全 9 项成功。
MySQL 与 PostgreSQL 分别完成有限/无限令牌 × JSON/SSE 的四个新增中途 Redis 故障检查；
原有控制和升级/回滚共计 56 个 PASS，四种 Compose 启动通过。

最终 runner 镜像 `sha256:8041f82fec3b8e91b27278017a8b8d7bafe0c3ee68dd2d695270ea64e4e1b80d`，
实际程序 Go1.25.14 / one-api / linux/amd64 / CGO1，binary SHA256
`77ca69d23f454e8e100159809fe4268f883867a44e523584e089017cf6a5a9fc`。未发布。
合并后 main [36832999472](https://github.com/dreamvm/one-hub/actions/runs/36832999472) 已成功。
数据库中途故障另见 [独立候选](DATABASE_INFLIGHT_ACCEPTANCE.md)，不能将本项 Redis 成功扩展为数据库或网络分区成功。
