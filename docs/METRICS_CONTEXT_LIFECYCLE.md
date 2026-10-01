# HTTP 指标与请求上下文生命周期

## 已确认问题与边界

现有 middleware/logger.go 在 c.Next() 返回后调用 metrics.RecordHttp；旧实现把 Gin Context 交给异步闭包，闭包稍后读取方法、匹配路由和响应状态。Gin 处理下一请求时会复用该 Context，造成数据竞争及指标标签/计数错归属。本次范围仅为 HTTP 请求计数与时长直方图，不改变鉴权、请求响应、账务或业务并发控制。

隔离旧版复现：httptest 内存路由，2 个 worker 各 32 次合成 GET 请求，两个不同路由/状态各 32 次；无网络监听或外部调用，测试超时 30 秒、指标收敛等待 3 秒。旧实现的 race 检查报告 RecordHttp 闭包与 Gin reset 竞争，并发生计数断言失败。单请求正常对照通过。旧版证据使用最初的计数断言；最终测试另外要求时长直方图的样本数也匹配。

## 最小修复与验证

在请求仍有效的 RecordHttp 调用内复制 method、path、statusCode 字符串，再交给原有异步记录任务；闭包只读取副本及按值传递的 duration。计数和直方图使用相同标签，异步调度及 SafelyRecordMetric 保持。RecordProvider 原本已复制所需字段，无须改动。

Go1.25.14 下专项测试与正常对照通过；相关 metrics、middleware、model、types、Gemini、Claude、requester 和工作流策略的 race 回归通过。相关 vet、metrics/providers/relay 编译、gofmt、固定 actionlint v1.7.12 工作流检查通过。离线 go run 最初因缺模块元数据未能运行，随后使用同版已有工具对全部工作流显式检查成功，没有把缺工具当通过。

一次独立审阅由未参与实现的既有审阅者执行，复用了先前 OIDC 审阅上下文，不是 fresh-context 审阅。未发现生产修复的具体问题；发现全局累计指标固定断言 1/32 导致同进程 -count=2 第二轮失败。已改为读取各指标初值并断言本轮增量，同时要求计数和直方图均为预期值；修正后 -race -count=2 通过。此测试修正由主代理完成，没有第二轮独立审阅。

## 交付与剩余范围

独立分支 codex/metrics-context-lifecycle，基于 PR #85 已合并 main a95142550659965969b1306c91c4e7cb6aa1f488。准确候选 CI、PR、合并和主分支验证未完成。该修复不等于并发负载、账务一致性及资源释放验收完成，也不建立生产容量结论。回滚源码会重新引入异步上下文竞争；无数据迁移、标签、发布、生产部署或付费调用。
