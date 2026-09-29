# C1f：Search 辅助模型先预留额度

`executeQuery` 在调用辅助模型之前，按照渠道 `PreCost` 计算输入 token，并执行共享额度预扣。
余额不足时不调用上游；上游失败时释放预留。上游成功后先按用量结算，再解析关键词，
因此无搜索工具、空结果或工具参数解析失败仍计入已经发生的辅助模型费用。
输入估算也传给 provider 的 usage，保留 OpenAI 在上游未提供 usage 时的输入用量回填。

搜索为可选增强，失败后继续普通聊天的行为不变。`PreContNotAll` 仍关闭输入估算；
免费模型保持原有费用规则。本项没有修改搜索引擎本身的定价。

`relay/search-quota_test.go` 使用临时 SQLite、模拟 provider 和本机 OpenAI HTTP 服务，
覆盖账户不足、令牌不足、长输入预扣、有限/无限、上游错误退款、成功后的不同解析结果、
缺失 usage 回填、原费用/关闭估算/免费对照，共 17 个叶子用例。CI 新增根 relay 包回归。

```sh
go test -mod=readonly -race -count=1 ./relay -run '^TestSearchQuota'
```

本项不解决零预扣政策、实际消费超过预留、Realtime 或进程崩溃恢复；三数据库专项不等于
Search 调用链的三数据库验收。本地不调用真实模型或搜索服务。无 schema 改动，回退会恢复
上游先执行后授权的顺序。真实付费上游与生产未验收。
