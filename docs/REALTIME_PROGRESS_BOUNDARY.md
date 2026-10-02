# Realtime 输出进度与未完成计费

独立分支 `codex/realtime-progress-boundary`，依赖 PR #77 的未完成响应状态。
基线为其候选 `236d2b5b2d5f8ee820c7c4a3236019e321c2fed9`，已快进至同树合并
`f5c1ad53ef3e2d4b078d55404fc6e4ac4098cca5`，保留原目录 Babel 暂存工作。

## 问题与处理范围

供应商可能没有发送可识别的 `response.created`，但已发送输出进度。
旧实现忽略这些事件，连接结束后仍按零或已完成响应的部分用量确定结算；
进度首帧写失败还可能退款重试。已观察到工作且没有对应完整报告时，应保留预留待核对。

本项在 OpenAI/Azure 共用供应商解析入口识别 20 类带响应 ID 的输出进度：
旧协议 text/audio/audio_transcript、新协议 output_text/output_audio/output_audio_transcript
的 delta/done；content_part 与 output_item 的 added/done；function_call_arguments 与
mcp_call_arguments 的 delta/done。事件来源依据官方
[当前 SDK](https://raw.githubusercontent.com/openai/openai-node/master/src/resources/realtime/realtime.ts)
及 [beta SDK](https://raw.githubusercontent.com/openai/openai-node/v4.104.0/src/resources/beta/realtime/realtime.ts)。

- 只在供应商文本消息提取专有生命周期元数据，不改变转发内容；客户端不能伪造该元数据。
- 顶层 response_id 字符串用于关联；缺失、null、非字符串成为匿名未知，不猜测嵌套 ID 或 item ID。
- 先读取进度信封，避免无关 response 等字段的类型错误丢弃已观察到的工作。
- 复用已存在的哈希 ID、1024 容量边界、重复收据、完整报告清除及持久待核对机制。
  不从增量估算金额；已完成响应的进度重放不重新打开工作。
- 无关完成不能清除已知未完成项；匿名未知不能靠另一响应完成而清除。
- 普通 session/config/conversation 确认不代表模型输出，本项不使用泛化 response 前缀判断。
  错误事件仍返回原 types.Event 合约。

## 验证记录

全新只读预调查和父任务分别追踪首帧、后续消息与终局结算路径。
临时副本旧版对照：20 类事件共 40 个未完成场景失败，20 个匹配完整报告控制通过。
候选专项使用 localhost 随机端口、临时 SQLite 和合成事件；147 个叶子/150 个节点通过，
其中本项新增 111 个 WebSocket 场景和 18 个解析控制，其余为相邻未完成响应回归。
覆盖正部分费用、首帧写失败、客户端关闭、无关完成、重放、ID 表示与错误合约。
没有测试输出写入真实用户文件、真实上游、真实账本或付费调用。

规定离线及受影响包 race、发布策略、vet、providers/relay 编译通过。
全新只读候选审阅未发现具体存活绕过或新增回归，并独立重跑 Realtime 测试，
补充 OpenAI/Azure 字段大小写、转义、重复字段和无关字段控制。
审阅两文件补丁 SHA256 `a6ec5bdf2470816b5fe3b0b213a664caefaa54fcdeed3d1be933bb987799bf31`，
审阅后无源码/测试修改。审阅未覆盖真实供应商、CI、其他数据库或容量负载。
准确候选 CI/PR/合并仍待完成；本地通过不等于交付。

## 保留限制与回退

不可解析的完成报告、非法/溢出计数属于下一独立问题；本项未改变收据冲突策略。
没有响应 ID 的 MCP item 状态、未观察到供应商工作前的客户端 inference/VAD、
独立输入转录计费、截断/无法辨认 JSON、进程崩溃前证据持久化和历史核销仍开放。
这里不证明真实供应商成本，也不宣称全部 Realtime 异常结束已修复。
回退恢复原来丢弃进度证据的行为；必须保留已持久化的待核对记录，不自动退款。
没有新建标签、发布镜像、部署或调用真实付费模型。


## 2026-10-02 continuation

The branch now incorporates main 1a89e9e28e884ebee417d54e647e012718b4c4e2 without changing the progress parser or its regression file. Earlier pending-CI text is historical: old candidate 11a0e376 already passed its recorded CI. Refreshed candidate checks are separate.

The later #79 review identified a typed-envelope error path that also limits this candidate. Its correction exists only in #79 and lacks completed independent review. Earlier successful progress review does not cover that path. Keep #78 draft and held together with #79; no interrupted review is retried or rerouted. No release or deployment.

Refreshed-source local checks passed with Go1.25.14: existing Realtime regressions, required five-package offline race set, affected-package vet and providers/relay compilation. No new interrupted-review variants were executed. Final-candidate CI remains separate and pending; the delivery hold is unchanged.
