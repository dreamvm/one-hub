# Realtime 被拒绝的完成报告

独立分支 `codex/realtime-invalid-usage`，当前叠加在 PR #78 候选
`11a0e3762fc2d8a0b440a30cb7e631414e387616`。合并前需核对前置交付、基线与准确候选 CI。

## 问题与兼容约定

供应商 `response.done` 类型无法解码、计数为负或累计溢出时，报告未被计入累计。
若此前没有该响应的开始/进度记录，收尾仍会把已知部分当作完整费用，或在首帧退款重试。
已识别工作而不能接受其报告，不足以证明未发生费用。

本项沿用供应商专有生命周期元数据和未完成项的哈希关联：

- 可识别 created/done 的类型解码失败仍返回工作元数据，并保留原
  `json_unmarshal_failed` / `invalid_event`；无效 ID 不猜测其他字段。
- 首帧及后续转发先消费该元数据再处理原错误；主循环先关闭供应商，避免向慢客户端报错时继续工作。
- 合并报告失败且未接受用量时，记录相应未完成项。连接结束后复用
  `realtime_unfinished_response` 持久待核对状态，保留预留与已知部分，不确定退款/扣费。
- 合法的同 ID 更正可以在终局前清除该项；无关报告或匿名错误无法据此归属。
- 已接受收据保持既定 first-accepted 权威，包括随后冲突/无效同 ID 重放；
  接受用量后的续额/容量错误仍按已接受报告结算。本项不改收据冲突产品策略。
- 正常完整报告、显式空 usage 的零控制、无工作握手和客户端方向保持既有语义。

## 验证

全新只读预调查已覆盖该独立问题；父任务在 PR #78 候选临时副本复核：
6 个 WebSocket 失败、4 个直接计费失败；5 个正常流及 1 个合法更正控制通过。
候选 281 个专项叶子/299 个节点通过，包含 first/main、非法 ID/元数据、匿名、
负明细、单报告/累计溢出、原协议错误优先级、上游关闭及相邻 Realtime 回归。
原首帧测试中“负数退款重试”的旧断言被改为保留 20 预留、零确定账单、仅一次上游调用；
其他正常首帧控制未改。规定及受影响包 race、策略、vet、providers/relay 编译通过。
测试均为临时 SQLite、合成数据和 localhost 随机端口，明确超时，没有真实付费调用。

独立候选审阅被平台内容检查中断，没有最终结论。中间报告指出第一层信封解析
可能在保留可识别事件类型的同时返回字段错误，提前返回仍会丢弃证据。父任务依据
Go1.25.14 encoding/json 的源码和本项目早退分支确认，补上已识别 created/done/progress
的保守元数据并保留错误。这是审阅后的源码修改，未独立复审；中断报告涉及的新变体
未由父任务重跑，不把中间报告或原有回归当作该变体验证通过。
没有重试、改名、拆分或转交被中断审阅。准确候选 CI/PR/合并未完成，草稿不得合并。后续对既有离线回归、vet、编译的重跑通过，
仅说明原受检路径未回归，不能替代缺失的完整独立审阅或新增变体动态验证。

## 限制与回退

截断/无法辨认类型的 JSON、没有响应 ID 的独立 item 状态、客户端请求尚未被供应商确认、
进程在证据持久化前崩溃、跨连接及历史账目处理仍开放。
没有新增数据库列或实际资金处置；错误报告不用于推算真实供应商金额。
回退会恢复丢弃无效报告后的错误终局路径，必须保留既有待核对记录。
最终八文件源码/测试补丁 SHA256：
`a8486131096bb22513fde98ce8057bb6261998ceb59026bd9f02bd918ea30733`。
未创建标签、发布镜像或部署生产。


## 2026-10-02: Invalid-report candidate synchronized with refreshed prerequisite

PR #79 now incorporates #78 candidate 04be2d23b2248579385b24ab14b1d32391e7906b and main 1a89e9e28e884ebee417d54e647e012718b4c4e2. Current ledger and all main fixes are retained; the invalid-report source/test patch is unchanged. Historical CI belongs to candidate 26465acd, not this refreshed revision.

The interrupted independent review still has no final conclusion. The previously corrected typed-envelope path remains without completed independent re-review and without parent dynamic verification of the additional reported variants. Existing regression checks must not be described as those missing checks. Both #78/#79 remain draft and held. No interrupted review is retried, renamed, split or rerouted. No actual account/balance changes, tag, image publication, deployment or paid call.

Refreshed-source checks passed with Go1.25.14: existing Realtime regressions, required five-package offline race tests, affected-package vet and providers/relay compilation. All eight invalid-report source/test files remain byte-identical to old candidate 26465acd. This is not independent review and does not validate the interrupted-review additional variants. New exact-candidate CI is pending.


### Invalid-report integration with delivered Vite/main

PR #79 incorporates refreshed #78 f63d6e96 and main 1f6d08bf. All eight invalid-report source/test files remain byte-identical to f5cbf834; the synchronization changes build dependencies and acceptance records only. Existing tests and exact new CI must be distinguished from the missing independent-review/additional-variant verification. Both PRs remain draft; no Realtime exception, merge, release or deployment is authorized by the Vite decision.

After integration, existing Go1.25.14 Realtime regressions and required five-package race tests, affected vet and providers/relay compilation passed. No missing additional-variant check or independent review is claimed. Exact new candidate CI remains pending.
