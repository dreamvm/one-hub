# 下一版本计划

更新：2026-10-01。计划基线：`76fc8238e2d187a81239d8e8126289efe9460464`。
首批修复集成提交：`e61c155c4ab4f1b3feb996f2aff95555774766a0`（PR #17–#20 已合并）。
RC7 修复集成提交：`7d7218100ec8db69b8f09c757ff65992c0323dc4`（PR #22–#23 已合并）。
执行规范见 [RELEASE_PROCESS.md](RELEASE_PROCESS.md)。

## 已核实的版本与预留编号

| 项目 | 状态 |
|---|---|
| 最近维护标签 | `v0.14.27-dreamvm.1-rc.7` → `d0f180451f770a7b01f41e52b4e14aeeb64dd924`，已从远端核实 |
| 下一个候选版本 | **`v0.14.27-dreamvm.1-rc.8`**，预留给后续额度修复，未创建标签 |
| 正式目标版本 | **`v0.14.27-dreamvm.1`**，仅计划，需完成全部发布阻断项 |
| 本轮 GitHub Release / 镜像 / 生产 | RC7 `publish=false` 构建成功；未创建 Release、发布镜像或部署，生产运行版本尚未核实 |

RC 是预发布候选，编号不代表质量验收。后续候选从 rc.8 起分批，
若候选需返工则继续递增，不能为了维持这张表复用旧编号；每次创建前重新核对远端。

## 修复批次

| 批次 | 范围 | 验收条件 | 状态 |
|---|---|---|---|
| 流程基线 | 版本规则、独立修复、验证与发布状态、PR 模板 | 文档一致且不改变自动发布开关 | PR #17 已合并；分支 CI 与隔离镜像验收通过 |
| rc.6 / A1 | 管理用户响应字段边界 | 列表/搜索不返回凭据，分页/排序/编辑所需字段正常，个人令牌生成仍可用 | PR #18 已合并；本地回归、独立审阅、分支 CI 与镜像验收通过 |
| rc.6 / A2 | 会话当前身份和权限 | 封禁/降级/删除后旧 Cookie 拒绝受限操作，正常 Cookie 与 Bearer 正常 | PR #19 已合并；本地回归、独立审阅、分支 CI 与镜像验收通过 |
| rc.6 / A3 | 请求日志脱敏 | 路径、编码/重复查询参数及失败路径无凭据；正常请求诊断字段保留 | PR #20 已合并；回归、审阅问题修正、分支 CI 与镜像验收通过 |
| rc.7 / B1 | 共享媒体下载目标策略 | 非公网地址、恶意跳转与 DNS 变化拒绝；合法下载及代理正常 | PR #22 已合并；回归、独立审阅、分支 CI 与隔离镜像验收通过 |
| rc.7 / B2 | Midjourney 回调身份和任务绑定、图片读取边界 | 用户通知只触发自有任务刷新；可信轮询绑定渠道和任务；图片读取受限 | PR #23 已合并；本地回归、独立审阅修正、分支 CI 与隔离镜像验收通过 |
| rc.8 / C1a | 令牌对象的当前授权状态 | 已持久化的额度/状态变化不被旧 token 缓存覆盖；正常令牌与额度恢复可用 | [PR #25](https://github.com/dreamvm/one-hub/pull/25) 已合并；CI 与隔离验收通过，完整 C1 仍开放 |
| rc.8 / C1b | 同一请求结算/退款至多一次、零消费释放、任务/MJ 重试终局 | 原问题回归失败后通过；正常有限/无限、免费、批量账目与调用方兼容 | [PR #26](https://github.com/dreamvm/one-hub/pull/26) 已合并；CI 与隔离验收通过，完整额度边界仍开放 |
| rc.8 / C2a | model 预扣条件更新与余额成对事务 | 并发不透支预扣余额、任一写失败整笔回滚、batch 即时落账、三数据库专项 | [PR #27](https://github.com/dreamvm/one-hub/pull/27) 已合并；三数据库专项、CI 与隔离镜像验收通过 |
| rc.8 / C1c | 取消高账户余额的预扣豁免 | 有限令牌不足拒绝；正常最终费用、无限与免费对照通过 | [PR #28](https://github.com/dreamvm/one-hub/pull/28) 已合并；CI 与隔离验收通过，完整额度边界仍开放 |
| rc.8 / C1d | 用户余额读取以数据库为准；缓存故障不阻断已提交账务 | 缓存旧值/缺失/异常不影响正确余额，退款及日志统计完整 | [PR #29](https://github.com/dreamvm/one-hub/pull/29) 已合并；CI 与隔离验收通过 |
| rc.8 / C1e | 预留与结算使用同一认证 owner / mode | 调用前和事务窗口变化拒绝，成功预留后模式变更按原模式结算 | [PR #30](https://github.com/dreamvm/one-hub/pull/30) 已合并；三数据库、CI 与隔离验收通过 |
| rc.8 / C1f | Search 辅助模型上游前预留及输入估算 | 预算不足零上游调用，失败退款，成功与正常对照正确落账 | [PR #31](https://github.com/dreamvm/one-hub/pull/31) 已合并；CI 与隔离验收通过，见 [SEARCH_QUOTA.md](SEARCH_QUOTA.md) |
| rc.8 / C1g | Realtime 入口预留、续额及连接结束后的同步结算 | 余额不足零上游握手，数据库续额、失败关闭、两 worker 退出与正常用量对照 | [PR #32](https://github.com/dreamvm/one-hub/pull/32) 已合并；CI 与隔离验收通过，见 [REALTIME_QUOTA.md](REALTIME_QUOTA.md)；未报告用量等边界仍开放 |
| rc.8 / C1h | 付费最小预留及图片数量估算 | 零舍入付费仍检查余额；负数/溢出拒绝；免费及实际超额对照 | [PR #33](https://github.com/dreamvm/one-hub/pull/33) 已合并；CI 与隔离验收通过，见 [QUOTA_PRICE_ADMISSION.md](QUOTA_PRICE_ADMISSION.md) |
| rc.8 / C1i | 最终费用算术、用量表示与任务费用一致性 | 非法费用拒绝；附加服务与不同规格计费完整；正常舍入/免费对照 | [PR #34](https://github.com/dreamvm/one-hub/pull/34) 已合并；CI 与隔离验收通过，见 [QUOTA_SETTLEMENT_ARITHMETIC.md](QUOTA_SETTLEMENT_ARITHMETIC.md) |
| rc.8 / C2b | 持久化预留、终局和确定意图恢复 | 重复终局不重复入账；余额/统计/日志事务；崩溃与故障可核对 | [PR #35](https://github.com/dreamvm/one-hub/pull/35) 已合并；三数据库、CI 与隔离验收通过，见 [QUOTA_RESERVATIONS.md](QUOTA_RESERVATIONS.md) |
| rc.8 / C2c | 异步任务失败成对补偿、持久幂等与轮询身份绑定 | 重复/并发至多退款一次；合法成功/无限/免费和三数据库通过 | [PR #36](https://github.com/dreamvm/one-hub/pull/36) 已合并；三数据库、CI 与隔离验收通过，见 [TASK_QUOTA_COMPENSATION.md](TASK_QUOTA_COMPENSATION.md) |
| rc.8 / C3a | 兑换码原子领取、充值和已使用终态 | 并发单次领取；错误回滚；过期编辑不重开；正常 NULL 兼容 | [PR #37](https://github.com/dreamvm/one-hub/pull/37) 已合并，三数据库/CI/隔离验收通过，见 [REDEMPTION_TRANSACTIONS.md](REDEMPTION_TRANSACTIONS.md) |
| rc.8 / C3b | 同一订单原子入账、幂等和提交后回执 | 失败可重试、已入账重复回调不重复充值、三数据库正常/故障对照 | [PR #38](https://github.com/dreamvm/one-hub/pull/38) 已合并，三数据库/CI/隔离验收通过，见 [PAYMENT_SETTLEMENT.md](PAYMENT_SETTLEMENT.md) |
| rc.8 / C3c | 支付事实/原渠道绑定、网关流水唯一归属、配置签名生命周期 | 正常优惠支付可用；错额/身份/缺失字段拒绝；历史归属和凭据轮换通过 | [PR #39](https://github.com/dreamvm/one-hub/pull/39) 已合并，三数据库/CI/隔离验收通过，见 [PAYMENT_NOTIFICATION_FACTS.md](PAYMENT_NOTIFICATION_FACTS.md) |
| rc.8 / C3d | 创建前金额/额度准入、先持久订单、微信整数分 | 非法输入零外部调用，提前/超时回调正常，原定价兼容 | [PR #40](https://github.com/dreamvm/one-hub/pull/40) 已合并，三数据库/CI/隔离验收通过，见 [PAYMENT_ORDER_ADMISSION.md](PAYMENT_ORDER_ADMISSION.md) |
| rc.8 / C3e | 已关闭订单与停用/软删除渠道的合法延迟支付 | 严格绑定事实、单次入账，旧渠道不能创建新支付 | [PR #41](https://github.com/dreamvm/one-hub/pull/41) 已合并，三数据库/CI/隔离验收通过，见 [PAYMENT_LATE_SETTLEMENT.md](PAYMENT_LATE_SETTLEMENT.md) |
| rc.8 / C3f | Stripe webhook注册/复用、密钥保留与异步成功订阅 | 不改global key，正常兼容版本可用，失败不误报保存 | [PR #42](https://github.com/dreamvm/one-hub/pull/42) 已合并，三数据库/CI/隔离验收通过，见 [STRIPE_WEBHOOK_REGISTRATION.md](STRIPE_WEBHOOK_REGISTRATION.md) |
| rc.8 / OIDC1 | 声明类型、同名拒绝关联与subject精确匹配 | 已绑定/新注册正常，异常声明和非精确身份拒绝 | PR #43 已合并，9项CI通过，见 [OIDC_CLAIMS.md](OIDC_CLAIMS.md) |
| 后续 C1 / C2 / C3 | Realtime未报告/重复用量与历史歧义数据 | 不重复结算；无法判定的历史预留/支付先核对归属与处理依据 | 部分边界仍开放，阻断正式版；已合并的事务与支付专项见上表 |
| D1 | Compose健康探针退出状态 | 正常状态成功，网络/HTTP/业务失败返回非零 | PR #44 已合并，9项CI及7项本地回归通过 |
| D2 | 锁定依赖、当前前端、失败传播及本地镜像构建 | 实际Task正常/故障对照，依赖输入哈希不变 | PR #45 已合并，全9项CI通过 |
| D3 | Fork镜像、显式秘密、私有数据库及可选依赖 | 默认保留MySQL+Redis，SQLite组合不引入依赖 | PR #46 已合并，全9项CI与四种真实Compose启动通过 |
| D4a | 真实Redis停止/错误类型/旧余额与恢复 | 拒绝请求零上游/零账务，恢复后正常单次结算 | PR #47 已合并，全9项CI、32项业务检查与四种Compose通过 |
| D4b | PostgreSQL候选镜像业务与Redis故障 | 登录/中英文JSON与SSE/工具/持久化/撤销及账务故障 | PR #48 已合并，全9项CI、48项业务检查与四种Compose通过 |
| D5 | 容器Go编译器身份与已验证补丁版本一致 | 读取最终候选binary，错误版本/主包/平台/CGO拒绝 | [PR #52](https://github.com/dreamvm/one-hub/pull/52) 已合并，全9项CI、实际Go1.25.14 binary及隔离业务验收通过 |
| D6 | 邮件依赖的SMTP信封地址编码 | 引号/转义完整，普通地址、显示名称、TLS与失败回执保持 | [PR #54](https://github.com/dreamvm/one-hub/pull/54) 已合并，全9项CI、48项业务检查、四种Compose与三数据库验收通过 |
| D7 | Bedrock上游EventStream解析崩溃 | 非法头和缺失异常类型返回错误，正常连续事件与合法头保持 | [PR #55](https://github.com/dreamvm/one-hub/pull/55) 已合并，全9项CI、48项业务检查、四种Compose与三数据库验收通过 |
| D8 | gRPC接收分片对象放大 | 旧版有界复现，新版数据/EOF完整，IAM正常与错误语义保持 | [PR #57](https://github.com/dreamvm/one-hub/pull/57) 已合并，全9项CI、48项业务检查、四种Compose与三数据库验收通过 |
| D9 | 顶层Vite开发服务器文件边界 | HTTP deny/query、WebSocket、sourcemap越界拒绝，合法开发流程保持 | [PR #60](https://github.com/dreamvm/one-hub/pull/60) 已合并；旧新对照、独立审阅、准确候选全9项CI与隔离镜像验收通过，Windows原生验收仍开放，见 [VITE_DEV_BOUNDARY.md](VITE_DEV_BOUNDARY.md) |
| D10 | Axios浏览器共享配置边界 | 合成前置污染下忽略继承data/reviver，合法XHR与API行为保持 | [PR #62](https://github.com/dreamvm/one-hub/pull/62) 已合并；旧新对照、独立审阅、准确候选全9项CI与隔离镜像验收通过，应用污染来源未证实，见 [AXIOS_BROWSER_BOUNDARY.md](AXIOS_BROWSER_BOUNDARY.md) |
| D11 | PostCSS 编译器 map 文件读取 | 旧版失败及合法 CSS/map 对照，真实 Vite 构建/import 验证 | [PR #64](https://github.com/dreamvm/one-hub/pull/64) 已合并；本地 16 项、只读审阅与准确候选全 9 项 CI 通过，审阅上下文限制见下文；Vite 独立开发加载器边界仍保留，见 [POSTCSS_FILE_BOUNDARY.md](POSTCSS_FILE_BOUNDARY.md) |
| D13 | Babel 开发依赖 source map 文件边界 | 六种编译 API 的词法包外读取拒绝，普通包内/inline/显式 map 与 JSX 保持 | [PR #67](https://github.com/dreamvm/one-hub/pull/67) 已合并；独立审阅、本地回归、准确候选全 9 项 CI 与隔离镜像验收通过；保留符号链接与少见文件名限制，见 [BABEL_SOURCE_MAP_BOUNDARY.md](BABEL_SOURCE_MAP_BOUNDARY.md) |
| D14 | Rollup 最终输出名称边界 | 异常名称拒绝，合法嵌套/归一化路径与构建保持 | [草稿 PR #68](https://github.com/dreamvm/one-hub/pull/68) 本地候选及准确提交手动全 9 项 CI 通过；独立审阅再次被平台中断，无结论，不得合并 |
| 后续 D | 依赖/端到端/负载验收 | 固定 Fork 镜像；健康失败正确退出；三数据库及相关故障路径通过 | 待实施，阻断正式版 |
| E1 | 主题按钮键盘操作和可访问名称 | 桌面/移动端深浅主题、Enter/空格/点击正常 | PR #49 已合并，全9项CI/48项业务/四种Compose与浏览器通过 |
| E2 | 深色填充标签对比度 | 深浅主题文字可读，选择/删除交互正常 | PR #50 已合并，全9项CI/48项业务/四种Compose与浏览器通过 |
| E3 | 前端组件属性声明 | 不改行为，属性类型与实际调用一致 | PR #51 已合并，全9项CI及48项业务检查通过，ESLint由33降至9条 |
| 后续 E | 前端既存警告与加载体积 | 行为回归通过、深浅主题可用、性能变化有依据 | 待实施，按影响安排 |

rc.6 只覆盖身份与凭据；rc.7 增加媒体与 Midjourney 边界。
C/D 仍未完成，不把 RC7 的批次验收作为整体安全整改完成或正式生产就绪证明。
剩余重点：Realtime未报告/重复用量、历史歧义预留与支付数据、OIDC issuer归属及subject唯一性、
依赖公告实际影响、请求中途Redis故障/网络分区和负载、历史凭据暴露与生产事实。上表已合并专项不重复标为未实施；证据不足项不登记为“已修复”。

### 后续执行顺序与完成条件

1. **依赖实际影响与最小升级**：先区分业务调用、浏览器产物和开发工具，再逐项复现/升级/回归；继续核对Go支持周期。D5只关闭容器编译器版本不一致，不能代替所有依赖公告结论。
2. **剩余计费和身份边界**：核对Realtime未报告/重复用量、历史歧义预留与支付，以及OIDC issuer归属/subject唯一性；先给出可验证规则，再修复并保留合法对照。
3. **故障与负载验收**：在隔离数据上补请求中途Redis失败、网络分区和并发负载；现有停止/重启、错误类型与旧余额检查不代表这些场景已经覆盖。
4. **前端行为与体积**：逐项清理剩余9条ESLint警告，验证请求次数、状态刷新及深浅主题，再用构建数据评估拆包；当前56项测试和浏览器专项不等于全页面端到端覆盖。
5. **发布准备**：完成arm64候选与剩余发布阻断项，另行核对历史凭据、生产配置/备份和恢复点。RC8仅预留，创建标签、发布镜像、部署和真实付费验收按各自授权执行。

每项仍采用独立修复、必要回归、准确提交CI、实际合并树核对和台账更新；有真实外部阻塞时仅暂停依赖步骤。

## 每次验收追加的记录

每项记录包含：问题编号、分支/PR、最终 SHA、复现与正常对照、命令及结果、独立审阅结论、
CI/镜像验收链接、剩余限制、兼容性影响与回滚方式。未执行项保留“未执行”，不得用旧提交的成功代替。

### 远端与合并验收

| 批次 / PR | 通过隔离验收的提交 | 合并提交 | CI（含 privacy、frontend、regression、smoke） |
|---|---|---|---|
| 流程 / [#17](https://github.com/dreamvm/one-hub/pull/17) | [672399a2](https://github.com/dreamvm/one-hub/commit/672399a2937f69edd4ca013c41b1c75d58f34162) | [3e017590](https://github.com/dreamvm/one-hub/commit/3e01759082f826c31c33ebc37bc72572c9bfbf4d) | [通过](https://github.com/dreamvm/one-hub/actions/runs/36404912996) |
| A1 / [#18](https://github.com/dreamvm/one-hub/pull/18) | [633aa602](https://github.com/dreamvm/one-hub/commit/633aa602cfd10abfd3116e6f6bac6fd80483f8c4) | [7417c93a](https://github.com/dreamvm/one-hub/commit/7417c93a91d7205a0e3510558df485b5fd668bae) | [通过](https://github.com/dreamvm/one-hub/actions/runs/36404970153) |
| A2 / [#19](https://github.com/dreamvm/one-hub/pull/19) | [9a9bddfc](https://github.com/dreamvm/one-hub/commit/9a9bddfc8fe669439ceab476afa40261de33f8a8) | [18273721](https://github.com/dreamvm/one-hub/commit/1827372145b604cf502400559e9168d4a0755b48) | [通过](https://github.com/dreamvm/one-hub/actions/runs/36404980180) |
| A3 / [#20](https://github.com/dreamvm/one-hub/pull/20) | [cec2278f](https://github.com/dreamvm/one-hub/commit/cec2278f625d799730f783cd97cff15d4ae2832d) | [e61c155c](https://github.com/dreamvm/one-hub/commit/e61c155c4ab4f1b3feb996f2aff95555774766a0) | [通过](https://github.com/dreamvm/one-hub/actions/runs/36404990797) |

- 按 #17 → #18 → #19 → #20 合并，逐项将基线切换至当时的 main，核对准确 SHA 的成功运行，
  并验证 `git merge-tree` 的结果与已受测提交的文件树一致。未改写原始提交或覆盖标签。
- 四个分支均完成 SQLite、MySQL＋Redis、实际运行版本、权限控制、工具调用、持久化及升级/回滚验收。
  最终分支前端 56 项测试、隐私 14 项测试（含真实 Docker 上下文）通过；lint 为 0 错误、33 条既存警告。
- 隔离最终分支镜像 ID 为 `sha256:58abc2becfbb91528f6d2954778abf8fe0e907d07aa5a9bc688a9bef73e583e2`，
  只在 CI runner 加载，未发布。镜像回退只对本次合成数据验证有效，备份恢复会丢弃备份后的写入。
- 最终 main `6ab8eea148cc2ecd0dbbc38b5084d2e503a344c4` 的
  [兼容性 CI](https://github.com/dreamvm/one-hub/actions/runs/36421796191) 与
  [隔离镜像验收](https://github.com/dreamvm/one-hub/actions/runs/36421828003) 已通过。
  在该提交上创建不可变 `v0.14.27-dreamvm.1-rc.6` 标签，远端标签与提交已回读核对。
- [RC6 手动构建](https://github.com/dreamvm/one-hub/actions/runs/36428117562) 成功：
  `publish=false`、`linux/amd64`，源码 SHA 与版本均匹配。
  本次未推送或导出镜像，工作流没有返回镜像 digest；不把 smoke 镜像 ID 当作已发布 digest。
  未创建 GitHub Release，未部署生产。
- 新增安全用例使用临时 SQLite；MySQL＋Redis smoke 只证明其脚本覆盖路径。
  PostgreSQL、浏览器端到端、生产运行版本和历史凭据暴露核实仍未完成，后续 B/C/D 阻断项保持开放。

### A1 本地验收

- 分支：`codex/security-user-responses`，依赖流程分支 `codex/release-process-rc6`。
- 修复前：新增响应测试在列表、搜索及详情路径复现凭据返回；分页和个人令牌对照正常。
- 修复后：显式响应字段白名单排除管理令牌与验证码；空密码字段保留供现有编辑表单使用。
- Go 1.25.14：`go test -race -count=1 ./controller ./model ./.github/tests`、
  `go vet ./controller ./model`、修改文件的 gofmt / goimports 与 diff 检查通过。
- 独立审阅未发现可验证绕过或兼容性回归；本地阶段仅使用临时 SQLite。后续远端验收见上表；PostgreSQL 与浏览器未验证。
- 固定 `actionlint v1.7.12` 检查通过；新增 controller 回归已接入既有 CI，不新增发布触发器。
- 未改 schema、个人令牌显示/轮换或发布开关。回退代码会重新暴露原问题，不能把回退作为凭据补救。
- 防止未来响应泄露不等于撤销已复制的令牌；生产暴露核实与必要轮换仍是待执行的独立操作。

### A2 本地验收

- 分支：`codex/security-session-state`，依赖 A1 提交 `633aa602`。
- 修复前：真实签名 Cookie 在禁用、降级、删除等状态变化后仍被接受；GitHub/飞书禁用账户绑定回归失败。
- 修复后：共享会话解析每次读取当前账户，不使用 Cookie 角色/状态或 Redis 授权缓存。
  强制鉴权、可选身份和两种 OAuth 绑定共用此边界；绑定在兑换授权码前拒绝无效账户。
- 正常对照：账户改名后的身份、降级后的普通权限、Bearer/MCP、匿名公共访问和合法绑定均通过。
- Go 1.25.14：controller/middleware/model 的 race 测试、vet、工作流策略、actionlint、格式检查通过。
  独立审阅未发现可验证绕过或回归；全部数据为临时 SQLite/模拟 OAuth 响应。
- 每次会话请求增加数据库身份查询；数据库不可用时受限接口拒绝请求，可选身份接口按匿名处理。
  未改 schema 或签名密钥，不强制所有正常用户重新登录；不宣称实现密码修改/注销的全设备会话撤销。
- 后续远端 CI 与 MySQL＋Redis smoke 见上表；PostgreSQL 与生产仍未验证，代码回退会恢复旧会话风险。

### A3 本地验收

- 分支：`codex/security-request-logs`，依赖 A2 提交 `9a9bddfc`。
- 原问题由 MCP/Telegram 路径、Gemini 编码/重复/异常查询、OAuth/支付参数、未知路由、错误信息测试复现。
- 日志改用路由模板，未知路径用固定标记；查询只记录是否存在，不记录参数内容。
  状态、方法、请求 ID、耗时和业务标识保留，处理器收到的原始 URL 与签名输入不变。
- 独立审阅发现 debug 自动重定向仍输出 URL、broken-pipe 恢复丢失诊断；新增失败回归后分别修正，
  由主审阅者重跑验证。协议内层恢复也改为相同的安全日志，保留根管理界面的异常历史和栈定位信息。
- 日志不再输出自由格式错误/原始 panic 值，debug 重定向不输出 URL；原有协议错误响应格式保持不变。
  这不代表其他业务日志或反向代理日志已全量审计，历史日志及已泄露凭据不被此修复清理。
- Go 1.25.14：新增正常、异常、debug 重定向和连接断开回归的 race 检查通过；未调用真实模型或支付。
- RC6 整体本地检查包含主程序编译、controller/middleware/model 与既有协议回归、vet、工作流策略、
  actionlint、前端 56 项测试、smoke Python 13 项与隐私 13 项测试（本地跳过的 Docker 上下文已由后续 CI 验证）、历史密钥扫描。
  隐私测试首次因未传扫描器路径而失败，补上固定版本环境变量后通过；流程命令已相应更正。

### B1 媒体下载验收

- 分支：`codex/security-media-downloads`，基线为已标记 RC6 的 `6ab8eea1`，独立修复，无其他 PR 依赖。
- 修复前：本机模拟图片服务收到完整下载、尺寸探测的两次请求，新增回归失败。
  修复后：两个入口均拒绝非公网目标，受限服务收到零次请求；内联图片保持正常。
- 共享下载在解析后固定数字 IP，每次重定向重新检查；混合 DNS、地址变形和同名解析变化均拒绝。
  合法 HTTP/HTTPS、图片/PDF、签名查询、IPv6、代理认证、TLS 和超时/关闭路径均有本机夹具对照。
- 独立审阅提出 IDNA 域名回归；先用 Unicode 与 punycode 对照复现，再统一规范化 DNS 与 TLS 名称。
  修正后重新执行全部媒体回归；审阅未确认其他默认下载路径绕过。
- Go 1.25.14：过滤后的媒体 race 回归、相关 controller/middleware/model/types/Gemini/Claude/requester
  回归、vet、全部 provider/relay 与主程序编译、工作流策略、actionlint 和格式检查通过。
  隐私 14 项中本地 Docker 上下文测试跳过，其余通过；历史/暂存区扫描与 smoke Python 13 项通过。
- [PR #22](https://github.com/dreamvm/one-hub/pull/22) 已合并：`77ff9898332db0ff1011cffe700700ebe994f171`，
  文件树与受测提交 `b92fb33b4913572c7f1fcb8f1b6172855ff8f67d` 一致。
  [兼容性 CI](https://github.com/dreamvm/one-hub/actions/runs/36431985685) 与
  [隔离镜像验收](https://github.com/dreamvm/one-hub/actions/runs/36431986063) 成功，后者 28 项 PASS。
  尚未创建 RC7 标签，也未发布镜像或部署。
- HTTP 代理要求支持目标端口 CONNECT；SOCKS5H 使用网关解析后固定的 IP。
  Worker 仍为管理员显式信任的远端服务，本仓库不能证明它的 DNS/重定向安全。
  完整兼容性与验证边界见 [MEDIA_DOWNLOAD_POLICY.md](MEDIA_DOWNLOAD_POLICY.md)。
- 未修改 schema；回退会恢复无限制媒体下载风险。Midjourney 独立图片入口与回调权限仍在 B2，未随本项关闭。

### RC7 合并与候选准备

| 批次 / PR | 通过验收的分支提交 | 合并提交 | CI / 隔离镜像验收 |
|---|---|---|---|
| B1 / [#22](https://github.com/dreamvm/one-hub/pull/22) | `b92fb33b4913572c7f1fcb8f1b6172855ff8f67d` | `77ff9898332db0ff1011cffe700700ebe994f171` | [CI](https://github.com/dreamvm/one-hub/actions/runs/36431985685) / [smoke](https://github.com/dreamvm/one-hub/actions/runs/36431986063) |
| B2 / [#23](https://github.com/dreamvm/one-hub/pull/23) | `586ce913b16673646d28679993af20af950d5617` | `7d7218100ec8db69b8f09c757ff65992c0323dc4` | [CI](https://github.com/dreamvm/one-hub/actions/runs/36549048487) / [smoke](https://github.com/dreamvm/one-hub/actions/runs/36549048898) |

- 两次合并均核对准确 head 的全部成功检查，并确认合并树与受测分支树一致。
  B2 的 PR 合成合并提交 `992ddbf99c31ee1a3ed8beb35e56b9e55ccaf554` 与修复提交的
  文件树同为 `77ea999681fe0ee7a17d036da697bea6d388dae6`；实际合并后再次核对没有文件差异。
- B2 分支前端 56 项、隐私 14 项（含真实 Docker 上下文）、后端回归及工作流检查通过；
  lint 为 0 错误、33 条既存警告。隔离镜像 28 项 PASS，覆盖 SQLite、MySQL＋Redis、
  实际运行版本、权限、协议工具调用、持久化、升级、镜像回退及备份恢复。
- B2 的 CI 镜像 ID 为 `sha256:baea111860c2fb132f24e5317fe67274002fe8a0bebba0ddc0d7bc97ac97f1be`，
  仅在 CI runner 加载，未发布；它不是已发布镜像 digest。回滚结论只覆盖合成数据，
  备份恢复会丢弃备份后的写入。新 Midjourney 专项测试使用临时 SQLite，通用 MySQL smoke
  不替代该功能的专项多数据库验证。
- 此台账更新是候选准备的一部分。标签必须指向最终已验收的 main 提交，
  不能把上述分支结果当成台账合并后主分支已通过的证据；最终 SHA 与运行链接另记在验收交付中。
- RC7 打标前再次核对远端编号和标签授权；手动镜像流程首先使用 `publish=false`。
  创建标签、正式发布镜像和生产部署分别记录，不由此文档或 CI 成功自动触发。
- 下一批 C 拆分为有限令牌额度、并发预留/结算、支付幂等三个独立修复，先复现再改动。
  真实上游、PostgreSQL、Redis 故障、浏览器端到端、历史凭据暴露和生产运行版本仍未验收。

### B2 Midjourney 本地验收

- 分支：`codex/security-midjourney-boundary`，基线 `77ff9898332db0ff1011cffe700700ebe994f171`。
  依赖已合并的 B1 / PR #22 公网媒体下载策略；本项单独提交和 PR。
- 修复前：普通用户通知能覆盖任务状态与图片 URL；四个模式入口均能访问存储的非公网图片 URL。
  未绑定的轮询响应可导致跨渠道更新或空指针异常。新增失败回归后再实施修复。
- 修复后：普通用户通知只接受自有任务 ID 并唤醒既有轮询；其他通知字段不写库。
  轮询按渠道与任务 ID 绑定，保留同渠道重复本地记录；冲突或未请求的响应在写入前拒绝。
  四个图片入口共用公网下载策略，保留原始二进制响应，并限制 15 秒、20 MiB 和请求取消。
- 正常对照覆盖合法通知、完成任务不重开、原始图片、默认 MIME、同 ID 多渠道、按钮/属性、
  既有失败补偿及单轮询器重复响应。上游非 200、读取失败和过大正文返回固定 502。
- 一次独立审阅指出环境代理未逐跳重选、轮询未继承渠道代理；均先复现再修正。
  同时复现并修正串联上游的一致重复响应兼容性；冲突重复项仍拒绝。
- Go 1.25.14：过滤后的媒体 race、Midjourney/controller/middleware/model/types/Gemini/Claude/requester
  race、vet、工作流策略、全部 provider/relay 与主程序编译、actionlint、格式及 diff 检查通过。
  本地隐私 14 项中 Docker 上下文测试跳过，其余通过；历史扫描与 smoke Python 13 项通过。
  最终 PR 的准确 SHA、CI 和隔离镜像证据另记于 PR 验收记录，不以本地结果代替。
- 通知变为异步刷新提示，按既有 15 秒节奏拉取；自定义 notifyHook 透传语义保持原用途。
  公开图片路由仍公开。具体兼容边界见 [MIDJOURNEY_BOUNDARY.md](MIDJOURNEY_BOUNDARY.md)。
  未修改 schema；代码回退会恢复原风险。真实上游、PostgreSQL、多实例退款原子性和生产未验收。
  未创建 RC7 标签、发布镜像或部署。

### RC7 标签与非发布构建

- 最终 main / 标签目标：`d0f180451f770a7b01f41e52b4e14aeeb64dd924`（含 PR #24 台账）。
  [准确 main 的兼容性 CI](https://github.com/dreamvm/one-hub/actions/runs/36551945470) 与
  [隔离镜像验收](https://github.com/dreamvm/one-hub/actions/runs/36551952077) 均成功，smoke 为 28 项 PASS。
- 已创建并回读远端不可变标签 `v0.14.27-dreamvm.1-rc.7`；annotated tag 对象为
  `ad94b38f095380577a63e1edc521c7f28d642643`，解引用提交与上述 main 一致。
- [手动构建](https://github.com/dreamvm/one-hub/actions/runs/36570420066) 成功，源码 SHA、RC7 版本、
  `linux/amd64`、`push=false` 与 `PUBLISHED=false` 均已从日志核实。
  未推送或导出镜像，`IMAGE_DIGEST` 为空；构建记录附件不等于可部署镜像。
- 未创建 GitHub Release 或部署生产。下一个候选预留 RC8；核对远端时该标签尚不存在。
  本节更新前述历史验收中“尚未打 RC7 标签”的状态，不改变那些检查的受测提交。

### C1a 当前令牌授权本地验收

- 独立分支 `codex/security-finite-token-quota`，基线为上述 RC7 提交，无未合并 PR 依赖。
- 先在旧实现复现缓存令牌覆盖已持久化状态，再令共享读取入口直接查询数据库。
  56 个负向组合覆盖 Redis、batch、新旧格式和 7 种状态变化；每个组合先验证合法令牌。
  额外正向对照覆盖当前字段、额度恢复及无限令牌。
- Go 1.25.14：model/controller/middleware/Midjourney/types/Gemini/Claude/requester race、
  相关 vet、工作流策略、provider/relay 与主程序编译、actionlint 和格式检查通过。
  隐私 14 项中本地 Docker 上下文跳过，其余通过；历史扫描及 smoke Python 13 项通过。
- 一次独立审阅未发现 C1a 范围内可验证绕过或新增功能回归，并独立重跑专项 race。
  Redis 模式每次鉴权增加数据库查询，移除旧缓存填充的 singleflight/1 秒等待包装；未压测。
- 专项夹具为 SQLite 与 Redis 命令 Hook，不能代表 PostgreSQL、真实 Redis 故障或待刷盘额度。
  远端 PR 准确 SHA、CI 与隔离镜像结果在 PR 验收记录交付，不以本地结果代替。
- 完整 C1/C2 保持开放：高余额预扣、批量计费、预扣/退款/重试、搜索与实时会话、并发原子性均待处理。
  未改 schema；回退会恢复旧缓存授权风险。未创建 RC8 标签、发布镜像或部署。


### C1a 合并验收

- [PR #25](https://github.com/dreamvm/one-hub/pull/25) 已合并为 `6b3351aa24e92aa6094fb1b5e837bff3587dd1b3`；
  受测候选 `9e9c75472a125e295a4ba5fd7f7852777dd8fbfa`，合并后文件树一致，本地 main 已快进核对。
- 合并前回读准确 head 的 7 项成功检查：[兼容性 CI](https://github.com/dreamvm/one-hub/actions/runs/36571980381)、
  [隔离镜像](https://github.com/dreamvm/one-hub/actions/runs/36571980617)。隔离验收 28 项 PASS；
  前端 56 项、隐私 14 项含 Docker 上下文通过，lint 0 错误 / 33 条既存警告。
- CI-only 镜像 ID 为 `sha256:9e9c6d00308be2232a914a8b38f34b120c803987a4749848957b636281830fcf`，
  未发布，不是注册表 digest。未创建 RC8 标签或部署。

### C1b 请求终结修复候选

- 分支 `codex/quota-request-lifecycle`，基线 `6b3351aa24e92aa6094fb1b5e837bff3587dd1b3`，依赖已合并 C1a。
- 先复现重复退款使余额超过初值、零消费遗留预扣、任务重试成功无正文、MJ 新模式预扣失败 panic 及重复退款。
- 共享 Quota 以同一个终结保护提交一次模型操作；Task 持有一次预留到终局；MJ 保护未取得新预留的路径。
  专项命令与兼容/故障边界见 QUOTA_LIFECYCLE.md，最终提交、独立审阅和 CI 证据在 PR 验收记录交付。
- Go 1.25.14：专项与相关包 race、vet、provider/relay 和主程序编译、工作流策略及 actionlint 通过。
  本地隐私 14 项中 Docker 上下文跳过，其余通过；smoke Python 13 项、历史密钥扫描通过。
- 一次独立审阅未发现本轮范围内可验证绕过或新增回归，独立专项 race 与相关 vet 通过；未做负载测试。
- 本项不是完整额度上限：Redis 预留补偿/批量同步、高余额跳过预扣、搜索/实时检查、原子性与故障恢复保持开放。
  没有 schema 变更；代码回退会恢复上述请求内问题。下一候选仍 RC8，仅预留，未发布或部署。

### C1b 合并验收

- [PR #26](https://github.com/dreamvm/one-hub/pull/26) 已合并为 `57aca411ad62e0601a7271a3eb4291dce91d25c3`；
  受测候选 `3d6f99e3756e33b0189c4aa6ff0b69d27614bfed`，合并后文件树一致。
- 合并前回读准确 head 的 7 项成功检查：[兼容性 CI](https://github.com/dreamvm/one-hub/actions/runs/36575335526)、
  [隔离镜像](https://github.com/dreamvm/one-hub/actions/runs/36575336483)。隔离验收 28 项 PASS；
  前端 56 项、隐私 14 项含 Docker 上下文通过，lint 0 错误 / 33 条既存警告。
- CI-only 镜像 ID 为 `sha256:b487c22954cca84cafc1ec7d05a6b610abbb9b985edbfabaeb1b642a0db4152b`，
  未发布，不是注册表 digest。未创建 RC8 标签或部署。

### C2a 额度事务候选

- 分支 `codex/quota-atomic-accounting`，基线为 C1b 合并提交 `57aca411ad62e0601a7271a3eb4291dce91d25c3`；无未合并 PR 依赖。
- 旧实现回归复现批量余额未即时落库、并发 24 次获准但预算只够 9 次，以及扣款/退款单边写入。
  同一路径非批量有限/无限正常对照通过。
- Pre/Post 余额改为统一用户→令牌事务；Pre 使用余额条件更新，Post 保留实际用量有符号差额。
  具体范围、兼容、升级旧队列和剩余问题见 QUOTA_TRANSACTIONS.md。
- 新增三数据库专项 CI；本地、独立审阅和远端准确 SHA 的执行结果在验收记录交付，不以配置存在代替通过。
- 本地 SQLite / 临时 PostgreSQL 专项 race、相关包 race/vet、主程序及 provider/relay 编译、工作流策略、
  actionlint、隐私及 smoke 夹具通过；本机无 MySQL，真实 Docker 上下文留待 CI。
  一次独立只读审阅未确认新增绕过或回归，并独立通过 SQLite 专项与 vet；未压测。
- RC8 仍仅预留；不关闭完整 C1/C2，不创建标签、发布镜像或部署。

### C2a 合并验收

- [PR #27](https://github.com/dreamvm/one-hub/pull/27) 合并为 `ceb3d7934e7372bb0208b9852aef1f67a2fdacc5`；
  受测 head `b100967ba3b55e74393e0d42f57481e5851dfdc5`，合并后文件树一致，本地 main 已快进核对。
- 准确 head 的 9 项检查成功：[兼容性 CI](https://github.com/dreamvm/one-hub/actions/runs/36579451038)、
  [隔离镜像](https://github.com/dreamvm/one-hub/actions/runs/36579452482)。三数据库专项各 30 个叶子用例通过，
  前端 56 项、隐私 14 项（含 Docker）通过；lint 为 0 errors / 33 既存 warnings；镜像验收 28 项 PASS。
- CI-only 镜像 ID `sha256:997a4cafa1432cdd91071cec09484ecbbe47879e369ddd9a5926499165ca2d0d`，未发布，
  不是注册表 digest。RC8 仍未打标，未部署生产。

### C1c 高余额预扣候选

- 分支 `codex/quota-high-balance-reservation`，基线为上述 C2a 合并提交，无未合并 PR 依赖。
- 旧实现复现高账户余额使有限令牌不足仍被放行，24 并发请求全部获准而令牌预算仅够 4 次；
  Task 和两种 MJ 入口在不足时仍提交上游。正常有限/无限最终费用对照在旧实现通过。
- 只删除共享预扣中的高余额清零分支，复用 C2a 原子预扣；保留原错误格式、免费与正常结算行为。
  兼容性、验证命令及明确开放的 Redis/入口/预留身份问题见 QUOTA_ADMISSION.md。
- 最终本地、一次独立审阅及准确提交的 CI 结果记录于 PR 验收交付；下一候选仍仅预留 RC8。
- 本地 38 个新增专项及相关包 race、vet、provider/relay 和主程序编译、工作流策略/actionlint、
  隐私与 smoke 夹具通过；本机缺失的真实 Docker 上下文检查留待 CI。
  一次独立只读审阅未确认新增绕过或回归，并独立通过高余额/生命周期/调用方专项 race 和相关 vet。

### C1c 合并验收

- [PR #28](https://github.com/dreamvm/one-hub/pull/28) 已合并为 `ebec64b545955554840e3726f5d0d3d868145b49`；
  受测候选 `01bcd0566c7c9587fb85f147552f7efdf44d23e7`，合并后文件树一致。
- 合并前准确 head 的 9 项检查成功：[兼容性 CI](https://github.com/dreamvm/one-hub/actions/runs/36611313838)、
  [隔离镜像](https://github.com/dreamvm/one-hub/actions/runs/36611314160)。三数据库各 30 项专项、前端 56 项、
  隐私 14 项通过；smoke 28 项 PASS，lint 0 错误 / 33 条既存警告。
- CI-only 镜像 ID `sha256:d61b6edc7e586fd993d9491aedd1bf97e63e690bc1d46f6182adfbb9ba60043f`，未发布。

### 后续执行顺序（2026-09-29）

以下为执行顺序及当前状态，逐项建立回归、独立修复、审阅及验收记录，不以计划代替完成。

1. 已完成（PR #29–#30）：C1d 旧 Redis 余额脱离授权和账务路径；预留 owner / mode 一致性另一个独立修复。
2. 已完成本批边界（PR #31–#32）：Search 上游前预留；Realtime 入口与运行中预算约束。
3. 已完成（PR #33–#34）：负数、零预扣、极小价格取整，以及实际用量超过预估的明确处理。
4. 已完成确定意图恢复（PR #35）：持久预留、结算幂等及重启、数据库错误、多实例恢复。
5. 已完成本批（PR #36）：Suno / Kling / MJ 后台退款成对记账和重复通知幂等。
6. 已合并 PR #37–#41：兑换码、支付幂等/事实绑定、下单与延迟支付；Stripe注册 PR #42 亦已合并验收。
7. OIDC 声明/同名关联与精确subject已合并（PR #43）；身份迁移、历史凭据暴露等剩余安全核实仍开放。
8. 健康检查已合并（PR #44），构建入口已合并（PR #45）；Fork 部署模板正在实施。
9. 三数据库业务覆盖、真实 Redis 故障、浏览器 E2E、依赖扫描及负载验收。
10. 前端既存警告、加载体积和深浅主题；按影响安排。
11. 候选版本和正式发布准备；标签、镜像发布、生产部署、真实付费验收分别核对授权。

### C1d 本地候选验收

- 基线 `ebec64b545955554840e3726f5d0d3d868145b49`；独立分支 `codex/quota-cache-authority`。
- 21 个新增叶子用例覆盖旧余额、缓存不可用、预扣失败、退款、正常结算和数据库错误。
  旧实现先复现失败；正常消费及零消费对照保留。修复后额度专项 race 通过。
- 一次独立审阅未发现可验证缓存绕过或新增兼容性回归，并独立重跑相关额度 race 通过。
- model / relay_util / task / Midjourney / controller / middleware / types / Gemini / Claude / requester
  race、工作流策略、相关 vet、provider / relay / 主程序编译、actionlint、格式与 diff 检查通过。
  隐私 14 项中本机跳过 Docker 上下文，其余通过；历史扫描与 smoke Python 13 项通过。
- 本地已验证不等于 CI 或合并；准确候选 SHA、CI 和隔离镜像结果另记在 PR 验收记录。
  未创建 RC8 标签、发布镜像或部署。

### C1e 候选范围

- 分支 `codex/quota-reservation-identity`，依赖 C1d / PR #29 的 `e1109251666d0dbd83ea492697baf84cb32f0188`。
  合并顺序为 #29 → 本项；合并前回读各自准确 SHA 的 CI 和受测文件树。
- 先以 8 个失败用例复现认证后模式或归属变化仍获预留，10 个预留后模式变化对照正常。
  修复后增加 24 个可在三数据库运行的事务用例；本地额度专项 race 通过。
- 完整持久恢复、预留后的删除/转移、零预扣和 Realtime 仍开放，不以本项关闭完整 C1/C2。
- 一次独立审阅未发现可验证的身份绕过或新增回归，并独立重跑身份和事务 race 通过。
  相关包 race、工作流策略、vet、provider / relay / 主程序构建、actionlint、历史扫描通过。
  隐私 14 项本机跳过 Docker 上下文 1 项；smoke Python 13 项通过。

### C1f 候选范围

- 分支 `codex/search-quota-admission`，依赖 C1e / PR #30 的 `58d8c60f7f46bd180eaaf0ef04468d2576b74aaa`。
  前置 #29 的 CI 曾出现同一测试进程混用签名密钥导致合法测试令牌失败；已统一夹具密钥，
  连续 10 轮事务/令牌回归及完整 model race 通过，远端对新提交重新验收。
- 旧 Search 实现先复现上游前无预留及缺失输入 usage 的 14 个失败用例，3 个正常费用对照通过。
  修复后 Search 专项共 17 项及相关包 race、vet、构建、工作流策略和 actionlint 通过。
- 免费、关闭输入估算和可选搜索降级保持原语义；真实搜索服务独立计费、Realtime 与持久恢复仍开放。
- 一次独立审阅未发现可验证的准入绕过或新增兼容性回归，并独立运行 Search race、relay vet 和 diff 检查。
  隐私 14 项本机跳过 Docker 上下文 1 项，其余及历史扫描通过；smoke Python 13 项通过。
  远端 CI、隔离镜像和合并状态另行记录，未发布或部署。

### C1d / C1e 合并验收

- C1d [PR #29](https://github.com/dreamvm/one-hub/pull/29)：受测 `195c9fed5ce8a52b9e990972ad112ecc502d0148`，
  合并为 `4064bc5943ab470c5eb6e7815bc56536f8ee12e7`；受测及实际合并树一致。
  [兼容性 CI](https://github.com/dreamvm/one-hub/actions/runs/36615405666)、
  [隔离镜像](https://github.com/dreamvm/one-hub/actions/runs/36615406081) 的 9 项检查全部成功，smoke 28 项 PASS。
  CI-only 镜像 ID `sha256:b4ac9522d11f1033d4e5ef29d82ac7bf0e22ea9fb5f2159580dffe74e3ca19d5`。
- C1e [PR #30](https://github.com/dreamvm/one-hub/pull/30)：受测 `58d8c60f7f46bd180eaaf0ef04468d2576b74aaa`，
  合并为 `5698397cc13fdccc16653e7b916a875b5af8acc6`；更新前置 main 后重新核对合成/实际合并树一致。
  [兼容性 CI](https://github.com/dreamvm/one-hub/actions/runs/36615453644)、
  [隔离镜像](https://github.com/dreamvm/one-hub/actions/runs/36615454040) 的 9 项检查全部成功。
  三数据库均含 24 个新增身份事务用例，smoke 28 项 PASS。
  CI-only 镜像 ID `sha256:310c784936052271703357d52be8eb1790e199fa9261657f53936e5a71641b5f`。
- 镜像仅在 CI runner 加载；这些 ID 不是已发布 digest，未创建 RC8 标签、发布或部署。

### C1g 候选范围

- 分支 `codex/realtime-quota-lifecycle`，依赖 C1f / PR #31 的 `8a1facea`。
- 先复现入口无预留、Redis 开关下有限令牌续额失效、两个 worker 未等待及缺失 response panic。
  首轮入口夹具使用非 realtime 模型名，导致 URL 错误，纠正模型名后才取得有效失败证据。
- 一次独立审阅提出零倍率免费分组误拒、慢客户端拖延预算失败后的上游关闭；两项均已回归复现并修正。
- 已报告用量在连接收尾同步结算；未报告费用、重复 usage、独立转录、持久恢复仍保持开放。
- 修正后 Realtime / WSProxy / model 专项及所有受影响包 race 通过；相关 vet、主程序/provider/relay
  构建、工作流策略、actionlint、格式检查通过。隐私 14 项本地跳过 Docker 上下文 1 项，其余、
  历史扫描及 smoke Python 13 项通过。最终准确 SHA 的 CI/镜像验收另行记录。

### C1f 远端验收

- PR #31 候选 `8a1facea746c007753c7cf438f17ff4739472de4`，合并提交
  `068ada2f533d8324c062974c9263ba5088bfbe45`；合并文件树与受测候选一致。
- [兼容性 CI](https://github.com/dreamvm/one-hub/actions/runs/36616236034) 与
  [隔离验收](https://github.com/dreamvm/one-hub/actions/runs/36616236464) 全部成功，smoke 28 PASS。
  隔离镜像 ID `sha256:e0c6488d4f9747158f6d8c0724477d7de944410036081bb0eba0f5a9e5b8a483`，仅 runner 加载。
  未发布、未部署。

### C1h 候选范围

- 依赖 C1g / PR #32 的 `fa9c5354`。共享准入与图片估算见 QUOTA_PRICE_ADMISSION.md。
- 原问题失败回归后通过；正常免费、按次截断、分数费用、零用量退款及 Redis/batch 下实际超额对照通过。
- 独立审阅、相关包 race、vet、编译、工作流策略与 actionlint 通过。远端 CI 尚待此候选提交。
- 最终结算完整算术、额外费用和持久化恢复未包含，继续按顺序执行。

### C1g / C1h 远端验收

- C1g / PR #32：候选 `fa9c53541f92edb67f6274016137fba06ca715f7`，合并
  `92135b5bea1c54c4bcaf380b388a21ed0884c6c7`。
  [兼容性 CI](https://github.com/dreamvm/one-hub/actions/runs/36618386449) 与
  [隔离验收](https://github.com/dreamvm/one-hub/actions/runs/36618386951) 全成功。
  runner 镜像 ID `sha256:ff2d49c48dbf31508116a2302e2653161cff286796e261c41112ea7d3f965e22`。
- C1h / PR #33：候选 `9859d5d036f34ca21450c0693996769b0838740a`，合并
  `009db9ba2a007cd4f4371d7af42802e90da4fa41`。
  [兼容性 CI](https://github.com/dreamvm/one-hub/actions/runs/36619576062) 与
  [隔离验收](https://github.com/dreamvm/one-hub/actions/runs/36619577036) 全成功。
  runner 镜像 ID `sha256:eda5f96e6c382e1d85167bcce41c4ef95f406b0437830fc93a64297957fb2a23`。
- 两批均有三数据库专项及 28 项 smoke PASS；实际合并树与相应受测候选一致。
  镜像未发布、生产未部署，RC8 仍未创建。

### C1i 候选范围

- 依赖 C1h，处理最终费用算术、Realtime 用量表示和任务保存费用，见 QUOTA_SETTLEMENT_ARITHMETIC.md。
- 失败回归、合法对照、一次独立审阅及其确认问题修正完成；相关包 race、vet、编译、工作流和隐私检查通过。
- 此候选远端 CI 与合并仍待执行；持久化预留/终局恢复是下一批，不把 sync.Once 当跨进程幂等保证。

### C1i 远端验收

- PR #34 候选 `1970ec148bea899e7861a1e161bc7161a7e58288`，合并
  `beeb263f02432520e829087cd393a6fa482572b5`，实际合并树与受测候选一致。
- [兼容性 CI](https://github.com/dreamvm/one-hub/actions/runs/36621405580) 与
  [隔离镜像验收](https://github.com/dreamvm/one-hub/actions/runs/36621405734) 全成功，三数据库专项与 28 项 smoke PASS。
  runner 镜像 ID `sha256:293ce49e4dcd058648840c0d9f956cd71287d405d411452a2fe323e6afedb639`，未发布。

### C2b 候选范围

- 依赖 C1i，新增持久账本并接入共享 relay；详见 QUOTA_RESERVATIONS.md。
- 旧版终局故障不恢复已复现；新增 SQLite/PostgreSQL 88 项事务叶用例及相关包回归通过。
- 独立审阅的意图写入失败与日志副本问题均已复现并修正；没有第二轮审阅代替主代理验证。
- MySQL/精确候选 CI 与隔离镜像验收待执行。后台任务成对退款及幂等为下一批；
  没有持久最终用量的预留仍必须核对，不能据此声明所有账务缺口已自动恢复。


### C2b 合并验收

- [PR #35](https://github.com/dreamvm/one-hub/pull/35)，head `961eb97d3efbddca11668035bc555ed4d62de4a3`，合并 `370fff87195118461e9111512ea66d6b75d6c66b`。
- [兼容性 CI](https://github.com/dreamvm/one-hub/actions/runs/36623891941) 和 [隔离镜像验收](https://github.com/dreamvm/one-hub/actions/runs/36623892306) 的 9 项检查全部通过。
- SQLite/MySQL/PostgreSQL 各 88 个额度事务叶子用例通过，镜像冒烟 28 PASS；候选、合成与实际合并树一致。
- runner 镜像 `sha256:fd627104d2f57af8793c1c1c180142f158b900d9785f6073c52a92b5e1db9540` 未发布。
- 没有最终用量且未落库确定意图的预留仍待核对，不自动退款；原令牌删除/转移不会把费用转给新主体。

### C2c 候选范围

- 分支 `codex/task-quota-compensation`，基于 C2b head，PR #35 已合并，无未合并依赖。
- 原回归复现用户已退而有限令牌未退；正常成功对照通过。补偿关联实际收据、原模式及最终费用，终态和意图同事务、成对退款和日志同事务，独立 worker 恢复。
- 新增全链路 Suno/Kling 提交与重复轮询、MJ 退款、正常进度、异常/跨任务响应、跨渠道重试；三数据库用例覆盖并发、重复、失败回滚、未知原终局、模式变化、历史任务和身份不确定。
- 独立审阅确认 Suno 跨渠道重试沿用原收据会阻断合法退款；先复现再按每次实际渠道重建预留修正。
- 本地相关包 race、workflow tests、vet、全提供商/主程序 build、actionlint、隐私与历史扫描通过；PostgreSQL 新增 31 个补偿事务叶子用例通过。精确候选 CI 与镜像验收待执行；不创建 RC8 标签，不发布或部署。

### C2c 合并验收

- [PR #36](https://github.com/dreamvm/one-hub/pull/36)，受测 `af12a12d94d21c3f345c96163bf15db4e2eec887`，合并 `9cf88ce66fa925de793a21f185aa852b3e8511aa`，候选、合成与实际合并树一致。
- [兼容性 CI](https://github.com/dreamvm/one-hub/actions/runs/36626566623) 与 [隔离镜像验收](https://github.com/dreamvm/one-hub/actions/runs/36626567290) 的 9 项检查全部成功；三数据库各 119 个事务叶子用例、smoke 28 PASS。
- runner 镜像 ID `sha256:0154016777f4fe41bdeac75397dc5cdfe49f94f99ca5d8529e4c7a846676002f` 未发布；历史无收据任务仍需人工核对。

### C3a 兑换码候选范围

- 分支 `codex/redemption-atomic-terminal` 依赖已合并 C2c；详见 REDEMPTION_TRANSACTIONS.md。
- 旧版 PostgreSQL 复现一张码并发充值两次；旧编辑可重开已使用记录、用户/日志失败仍消耗兑换码也已复现。正常兑换对照通过。
- 独立审阅确认历史启用状态仍带使用标记的记录可被再次领取，已先复现再修正，并保留 NULL 未使用标记兼容。
- 20 个新增事务叶子用例、HTTP/Redis 正常及故障对照、相关包 race、vet、编译、工作流/actionlint、隐私与历史扫描通过；PostgreSQL 本地通过。MySQL 与精确候选远端 CI/隔离镜像仍待执行。
- 支付回调为下一批；不把兑换码修复等同于支付修复，也不创建 RC8 标签或发布部署。

### C3b 支付入账候选范围

- 分支 `codex/payment-atomic-settlement`，依赖 PR #37 的 `37964e5cf13cff1335839757bcb218940c8acb88`；前置合并前保持草稿。
- 旧版回归复现：缺失用户/日志故障仍确认成功、batch 模式确认时余额未持久化、错误渠道仍可入账；正常非 batch 对照可用。
- 数据库订单锁替代进程锁；余额、晋升、日志、成功状态及结算标记同事务。四种网关回执移至提交后，同一已结算交易可安全重放。
- SQLite/PostgreSQL 25 个新增支付事务叶子用例通过；含真实本地签名的易支付/Stripe 回调、故障后新数据库连接重试、并发16次与等额独立订单。支付宝/微信真实签名端到端仍属下一批，不把回执单测当其完整协议验收。
- 一次独立审阅无范围内已确认问题；相关包 race、vet、主程序/provider/relay 构建、工作流/actionlint、隐私14（本地 Docker1跳过，CI验证）、smoke Python13及历史扫描通过。
- 精确候选远端 CI/MySQL/隔离镜像待执行。金额/币种/商户绑定、延迟支付、关闭/停用配置、创建订单时机及算术仍开放。历史成功但缺少结算标记不自动补款。

### C3a 合并验收

- [PR #37](https://github.com/dreamvm/one-hub/pull/37) head `37964e5cf13cff1335839757bcb218940c8acb88`，合并 `c6fe79f48d1be468ab40f4198ad1179ae28f80b8`；受测候选、合成与实际合并树一致。
- [兼容性 CI](https://github.com/dreamvm/one-hub/actions/runs/36628803639) / [隔离镜像](https://github.com/dreamvm/one-hub/actions/runs/36628804145) 全9项通过，三数据库各139事务叶子用例，smoke28 PASS。
- runner 镜像 `sha256:cc116f2a34e59e2f254f5d6c621372b7cc26a156dc3a0cd2aeca1ad84aba7463` 未发布；RC8 未创建。

### C3b CI 时限调整

- 首次 CI 一套 MySQL 矩阵在既有用例建/删表时触及120秒总时限，另一套同提交矩阵通过；保留失败日志。
- 随新增业务覆盖，单引擎总测试时限调至300秒，保留全部断言和15分钟 job 限制；追加 `b7cdbaba4ad66988085ef0d5f0a8da6c2c94790a` 后重新执行完整 CI，不能复用旧 head 的镜像结果。

### C3c 候选范围

- 分支 `codex/payment-verified-facts` 依赖 PR #38 的 `b7cdbaba`，合并前保持草稿。
- 签名有效的错误金额/商户/方法及跨订单流水重用已先复现；正常及优惠总额对照保留。支付宝重启合法回调失败、微信错误 app/merchant/amount/currency 也有旧版执行证据。
- 一次独立审阅指出历史流水归属与同商户凭据轮换，两项均复现后修正。历史夹具先纠正从旧订单继承终态的问题；微信冷启动夹具改用 SDK 要求的 PKCS8 私钥，再取得正常/轮换通过证据。
- 远端精确候选/MySQL/隔离镜像仍待提交后验收。订单创建、关闭/停用的延迟通知、Stripe 订阅与既有 webhook secret 保留是下一批。

- 审阅修正后 SQLite/PostgreSQL 支付专项 64 个事务叶子用例、四网关本地签名对照、相关包 race/vet/build、工作流/actionlint、隐私14（Docker1本机跳过）及 smoke Python13、历史扫描通过。

### C3b / C3c 合并验收

- C3b PR #38 head `b7cdbaba4ad66988085ef0d5f0a8da6c2c94790a`，合并 `1f01159be84aa483422e5ea058147cb2242125a7`。CI [36630748759](https://github.com/dreamvm/one-hub/actions/runs/36630748759) / [36630749358](https://github.com/dreamvm/one-hub/actions/runs/36630749358) 全9项通过，三数据库各164事务叶子用例、smoke28 PASS。
- C3c PR #39 head `b98f630b357b554072d8c4471f533ee73119a993`，合并 `6b3fe8f83d77b69f65471a632a251226d3d68f0e`。CI [36632459687](https://github.com/dreamvm/one-hub/actions/runs/36632459687) / [36632459867](https://github.com/dreamvm/one-hub/actions/runs/36632459867) 全9项通过，三数据库各203事务叶子用例、smoke28 PASS。
- 两项候选、合成合并、实际合并树分别一致。C3b runner 镜像 `sha256:cda9686f654b85ef2809f89a29add92aa92fb7ab31fb60bd99195ec69eed094e`；C3c 为 `sha256:db4af93c07a73f40de62a9054ba60a75b5e00e1196f33e5fa3e1cbdce98dd62c`；均未发布。

### C3d 订单创建候选

- 分支 `codex/payment-order-admission` 依赖已合并 PR #39；先验证定价和额度，再持久订单，再发起外部支付。超时保留待核对状态，不覆盖已由提前回调提交的成功。
- 旧实现的提前回调、数据库插入失败后仍创建外部支付、非法额度/金额及微信4.10元截断为409分已复现，正常对照通过。
- 一次独立审阅发现新十进制舍入改变正常手续费，补充失败用例后恢复既有算法，保留有限值/范围检查；额度乘法单独用精确运算。核对驱动后撤销无依据的 schema 扩宽，保留大额合法订单对照。
- 新增16个事务叶子用例及微信本地签名请求/服务币种对照；SQLite/PostgreSQL专项、相关包race/vet/build、工作流/actionlint及格式检查通过。精确候选 MySQL/CI/镜像验收待执行。
- 关闭/停用渠道的延迟通知与 Stripe webhook 注册仍属下一批；RC8 未创建，未发布部署。

### C3e 延迟支付候选

- 分支 `codex/payment-late-settlement` 依赖 PR #40；旧实现中正常支付通过，已关闭、停用/软删除渠道和关单竞争的合法支付失败。
- 回调专用配置读取保留原验签；创建入口仍拒绝停用/软删除配置。只增加closed可结算，不开放failed、历史歧义success或异常标记。
- 新增12个事务叶子用例，原closed拒绝用例由合法延迟及异常标记控制替换；净增11项。独立只读审阅无阻断发现，指出两个重复状态夹具，已分别改为pending停用/删除与closed组合。
- 本地SQLite/PostgreSQL与相关race/vet/build通过；精确候选CI/MySQL/镜像待执行。Stripe webhook注册为下一独立修复。

### C3f Stripe 注册候选

- 分支 `codex/stripe-webhook-registration` 依赖 PR #41。接口由模拟HTTP提供，无真实Stripe操作。
- 先修正测试夹具为SDK实际Bearer鉴权，再取得旧实现失败与正常新建控制证明；修复保留既有secret和其他订阅、补齐异步成功事件，使用实例key。
- 一次独立审阅提出同API系列兼容日期被拒绝，新增回归复现后按固定SDK规则修正。新增保存数据库失败对照；共15个新增事务叶子用例。
- SQLite/PostgreSQL、相关race/vet/build和工作流检查通过后提交；精确候选CI/MySQL/镜像另验收。已有线上webhook不自动修改，发布前需独立核对与授权。

### C3d / C3e 合并验收

- PR #40 head `10e8bfcdf4a3ae04531015ae7b4718ce12796945`，合并 `3d41a9c667a8f213d99edfb20cb5d806713f2c40`；CI [36634321850](https://github.com/dreamvm/one-hub/actions/runs/36634321850) / [36634322267](https://github.com/dreamvm/one-hub/actions/runs/36634322267) 全9项成功，三数据库各219事务叶子用例，smoke28 PASS。
- PR #41 head `fb0e45d879ff9647e4ebffe5f4c7c90d7ef4434f`，合并 `507255ef3159c7626c2e43f5998d1b9beb2f3b2a`；CI [36634842781](https://github.com/dreamvm/one-hub/actions/runs/36634842781) / [36634843205](https://github.com/dreamvm/one-hub/actions/runs/36634843205) 全9项成功，三数据库各230事务叶子用例，smoke28 PASS。
- 候选、合成和实际合并树一致。镜像分别为 `sha256:4b0688e7aac49fd85cfe48d5625152d97a5085924974278096d3ecd30e3e436b`、`sha256:fecd0199243a2de3f3ff04eac811868859e358bd2b566ba1e3c257ad9b553fa7`；仅runner本地加载，未发布。

### C3f 合并验收

- PR #42 head `0f9c247377cd3a8934346e2f371189aba01ac065`，合并 `b04145b00976128a763e303fefa3d059557b32da`；候选、合成与实际合并树一致。
- CI [36635726603](https://github.com/dreamvm/one-hub/actions/runs/36635726603) / [36635726938](https://github.com/dreamvm/one-hub/actions/runs/36635726938) 全9项通过，三数据库各245事务叶子用例、smoke28 PASS。
- runner镜像 `sha256:f992214fff3c8ca243abac59e6782c51049bd62880476727877cb7a5355edd1f` 未发布。

### OIDC1 候选

- 分支 `codex/oidc-claims-identity` 依赖已合并 PR #42；未知subject不再根据同名声明登录/改绑已有账号，必需与可选声明按类型检查。
- 旧实现正常绑定/注册通过，同名关联和错误类型失败；初始夹具先补齐生产初始化所需logger，再取得有效回归证明。
- 一次独立审阅指出SQL排序规则可能等同不同subject。改用正常临时表的NOCASE/RTRIM夹具后，精确匹配控制通过、大小写/尾空格变体错误登录已复现；添加原始subject精确比较后全部通过。
- 共33项真实本地签名OIDC子用例，包括登录cookie经认证中间件回读、注册开关、禁用状态、缺失/null资料、错误issuer/audience/签名/有效期。
- 相关包race、vet、主程序build、工作流/actionlint和格式检查通过；未改变数据库查询/schema，专项目前SQLite模拟比较规则，未宣称MySQL/PostgreSQL真实OIDC验收。精确候选CI待执行。
- 未绑定/解绑账号不再自动同名关联；安全绑定流程、issuer迁移与subject唯一性另行处理。RC8仍未创建，未发布部署。

### D1 健康探针候选

- 分支 `codex/compose-healthcheck` 依赖 PR #43。旧管道以awk退出状态为准，网络/HTTP/业务失败和空响应均可误报健康；正常与带空白JSON控制通过。
- 先检查wget退出状态，再检查success为true；使用POSIX字符类并保留Compose变量转义。7项测试从实际YAML提取并执行探针，修复前5项错误路径失败、修复后全通过。
- workflow策略测试、actionlint、diff格式通过；没有改应用代码或部署运行服务。精确候选CI与镜像另验收。

### D2 构建入口候选

- 分支 `codex/reproducible-local-build` 依赖 PR #44。移除普通build的tidy依赖与仅检查旧index的跳过规则，genui使用Yarn frozen lockfile且不改package.json；前端失败不再继续后端。
- Go构建加入readonly与trimpath；日期取提交日期，本地镜像任务不再包含推送。元数据通过环境变量传入命令，包含空格的工作目录有执行对照。
- 固定Task v3.53.1实际执行5类正常/故障/本地Docker命令夹具，旧实现均违反至少一项契约，修复后通过；策略测试/脚本语法/actionlint通过。
- Node22.20.0/Yarn1.22.22/Go1.25.14真实task build通过，go.mod/go.sum/package.json/yarn.lock前后哈希不变。构建体积提示仍保留，未提高阈值掩盖；依赖升级/前端体积另批处理。
- 同一提交/工作树与固定工具链连续两次真实构建二进制SHA256相同：`92071a16681bb5240966a7f7b8a25796c955a07107053a59590223bcceb3cfba`；这是本机重复性证明，不代表跨平台产物相同。

### OIDC1 / D1 合并验收

- PR #43 head `6c4719cedeee9d21a218ee649b8a16cff8daa539`，合并 `6cac316a1bd73800794c3dc72b9fc0ad35c45002`；CI [36674884050](https://github.com/dreamvm/one-hub/actions/runs/36674884050) / [36674884234](https://github.com/dreamvm/one-hub/actions/runs/36674884234) 全9项通过。
- PR #44 head `0108f373999bf0171c5be80debc86b1097e126e9`，合并 `16b3d41cc76180e94e85e82465c66b522537dc94`；CI [36675089885](https://github.com/dreamvm/one-hub/actions/runs/36675089885) / [36675090349](https://github.com/dreamvm/one-hub/actions/runs/36675090349) 全9项通过。
- 两者均三数据库各245事务叶子用例、smoke28 PASS，候选/合成/实际合并树一致；runner镜像分别为 `sha256:5bdeb03ce52a10aab73cbd5d8c973a4d744589fe50d204c52573f31b44611868`、`sha256:3d0914ab60bfc7f84d4c8343c80477528279cffc20f51640abb7952a0f26ba09`，未发布。

### D3 部署模板候选

- 分支 `codex/explicit-deployment-config` 依赖 PR #45。默认保持MySQL+Redis，数据路径不变；独立SQLite模板与MySQL/Redis叠加文件共享配置。
- 旧模板真实Compose正常控制通过，缺失显式参数仍解析、上游浮动镜像、公开DB和仅等待启动的回归失败；测试夹具先分离stderr弃用提示与JSON输出，再取得有效边界证据。
- 新模板要求Fork镜像digest和显式秘密，DB/Redis不公开端口、依赖健康。缺失/空参数、四种组合和探针故障对照本地通过；密钥非空不等于强度验收。
- 固定Compose 5.5.1二进制校验官方SHA256；CI增加四种模板的实际候选启动（本地image ID、临时数据卷、隔离网络）。本机无Docker，运行验收待准确候选CI，不能以配置解析替代。
- 文档去掉上游/固定秘密的Docker示例，说明已有密钥/数据库密码保留、SQLite配置文件省略SQL_DSN、镜像摘要身份与回滚边界。未改生产、未发布RC8。
- 一次独立审阅发现SIGTERM跳过临时Compose资源清理；父任务补充失败回归复现，沿用既有signal转异常模式。正常四模式/启动失败/信号清理3项通过；隔离测试关闭编码器下载和自动价格更新，真实业务网络未开放。

### D2 合并验收

- PR #45 head `025667aea168979dec0848b9e3f29002bae9207d`，合并 `6e8cf650518060770919bc268b7d410ac9994f62`，候选/合成/实际合并树一致。
- CI [36676186620](https://github.com/dreamvm/one-hub/actions/runs/36676186620) / [36676186794](https://github.com/dreamvm/one-hub/actions/runs/36676186794) 全9项通过，三数据库各245事务叶子用例、smoke28 PASS。
- runner镜像 `sha256:bc514ed1b8fd319fb00c270518edcc9ba2da949e2c2d64e09f5121d0830dc116` 未发布；RC8仍未创建。

### D4a Redis 故障验收候选

- 分支 `codex/redis-failure-smoke` 依赖 PR #46，只扩展隔离验收，不修改应用故障处理策略。
- 在真实MySQL+Redis候选中停止Redis、构造group缓存错误类型，验证请求拒绝、模拟上游计数不增、用户/令牌/预留账目不变；恢复后核对正常费用、次数和单一预留记录。
- 真实Redis旧零余额不覆盖正DB余额，旧高余额不授权零DB余额；恢复正余额继续单次结算。
- 所有操作限本次新建的合成容器/账号/数据库；本机Python语法及16项现有夹具通过，真实故障用例待准确候选CI，不记为已验收。
- 此批不覆盖Redis在已准入请求中途失效、认证轮换、网络延迟/分区或负载；这些仍需独立证据。
- 首次真实CI在故障前的正常控制发现测试假设错误：playground使用无限额度令牌，其自身quota计数按现有契约不更新。源码与既有模型用例核对后按捕获模式比较，新增有限/无限正常和错账拒绝控制；未改变应用账务。精确候选重新验收。

### D4b PostgreSQL 业务验收候选

- 分支 `codex/postgres-business-smoke` 依赖 PR #47。为既有真实镜像业务流程增加PostgreSQL18+Redis后端，使用与事务专项相同的固定PostgreSQL镜像摘要。
- 本次新建内部网络、数据卷和非root数据库/Redis；psql经TCP连接、SQL错误返回失败。沿用MySQL的协议/工具、普通权限、重启、撤销和Redis故障检查。
- 新增不可变镜像与隔离SQL夹具正常/拒绝控制；本地Python18项、语法与工作流检查通过。完整PostgreSQL镜像验收待CI，本机没有Docker。
- 不验证既有生产PostgreSQL数据升级或恢复，不将新库业务流程视为所有历史schema兼容证明。
- 工作流策略回归先发现固定步骤清单尚缺PostgreSQL；已扩充必须执行的后端与不可变镜像检查，保留失败即阻断的约束，再重新验证。
- 已同步PR #47的无限令牌正常对照修正，合并后Python20项与策略检查重新通过；候选CI重新运行。

### D3 合并验收

- PR #46 head `196be65f24bd55bd68d409d3973499f695cbd6bc`，合并 `bb926b1c2f7c1deeaed3f4539cdd5a5e36930b13`；候选/合成/实际合并树一致。
- CI [36677328868](https://github.com/dreamvm/one-hub/actions/runs/36677328868) / [36677328907](https://github.com/dreamvm/one-hub/actions/runs/36677328907) 全9项成功；三数据库各245事务叶子用例、smoke28 PASS、四种模板实际启动COMPOSE_OK。
- runner镜像 `sha256:0fee891ed996966eb0ddc83dc230f856035b9107c93477a6cb15640bbd7d4b64` 未发布。RC8仍未创建。

### E1 主题按钮候选

- 分支 `codex/theme-button-keyboard` 叠加于PR #48 `ab69627a`，合并顺序为 #47 → #48 → 本项；仅改变主题按钮与四种语言标签。
- 真实Chrome旧实现鼠标点击正常，但聚焦按钮后Enter不切换。将事件移到ButtonBase并添加本地化可访问名称，保留Redux与localStorage状态保存。
- 修复后真实浏览器Enter→深色、空格→浅色、点击→深色通过；1440×1000桌面、390×844移动视口完成页面身份、非空/无错误遮罩、控制台及截图检查。
- 临时SQLite中登录、创建合成渠道、刷新回读及移动端打开/取消编辑正常，移动页面无整体水平溢出。未调用渠道上游或生产服务。
- 使用已有Playwright与独立Chrome配置（Browser plugin not available）；临时Go overlay仅将测试监听地址收紧到127.0.0.1，未改项目main.go。56项前端测试与生产构建通过，lint零错误、33条既存警告。
- 当前仅Chrome与上述视口；其他页面、浏览器、历史数据未全量验收。既有深色编辑器标签对比度仍需独立检查，不作为本项修复完成。精确候选CI待执行。

### D4a / D4b 合并验收

- PR #47 head `2605af88edf5229eb00963f1b784dea6105c2bcd`，合并 `93e05122cd33ced2d30de276d13777fc04b5402d`；CI [36678973540](https://github.com/dreamvm/one-hub/actions/runs/36678973540) / [36678973726](https://github.com/dreamvm/one-hub/actions/runs/36678973726) 全9项成功，smoke32 PASS。
- PR #48 head `ab69627aab627c26937543ddfbfd34edc5d88b37`，合并 `0b6e5fee7fddf33ac57f237400a051ce5f496865`；CI [36679114678](https://github.com/dreamvm/one-hub/actions/runs/36679114678) / [36679114840](https://github.com/dreamvm/one-hub/actions/runs/36679114840) 全9项成功，smoke48 PASS，包含完整PostgreSQL业务与Redis故障检查。
- 两者均三数据库各245事务叶子用例、四种真实Compose启动通过，候选/合成/实际合并树一致。runner镜像分别为 `sha256:f93fa5e01b242c1e467e6f23a48c82d35f1043d60a82e49cc4659a5605995b2c`、`sha256:e247a081da24864529beaa9e48f029e29e0656de08bfa1d5f21c96af96766d27`，未发布。

### E2 深色标签候选

- 分支 `codex/chip-dark-contrast` 依赖PR #49。真实Chrome中默认填充Chip在深色主题为白字白底，对比度1:1；浅色正常控制为15.52:1。
- 仅按现有主题选择深色文字，背景、禁用样式和数据逻辑不变。真实编辑器修复后默认标签文字可读；深浅主题普通/悬停对比度及删除交互、桌面/移动视口回归通过。
- 前端56项测试、lint零错误/33条既存警告、生产构建通过；准确候选CI待验收。没有把单个默认Chip的测量扩大为全站WCAG认证。

### E3 属性声明候选与依赖检查恢复点

- 分支 `codex/frontend-prop-contracts` 依赖PR #50；为11个文件补充已有prop-types声明，不增加库、不改Hook依赖/数据逻辑/样式。
- 初次lint发现个人页的实际组件名为CustomTabPanel，已纠正声明目标后重跑；最终56项测试和构建通过，lint零错误，33条警告降到9条。没有关闭规则或提高阈值。
- 新构建真实Chrome验证仪表盘、个人设置、保存的合成渠道和390像素移动视口，无页面错误/控制台警告；仅覆盖这些流程，准确候选CI待执行。
- 剩余9条为7处Hook依赖和模型模态渲染回调的2处key声明；需分别验证数据更新/请求次数，不直接补依赖触发循环。
- 已完成只读依赖基线扫描：govulncheck v1.7.0 / Go1.25.14 / 官方库2026-09-28，13条公告带静态符号调用链；Yarn审计275条路径记录去重为144条公告。静态可达、初始化或泛型Error链不等于漏洞可利用；尚未把任何依赖公告登记为已修复。
- 图片解码、IDNA/文本处理的官方修复模块已在临时缓存核实可用于Go1.25；图片解码边界正在独立调查。前端最严重公告位于i18n CLI依赖链，需区分构建工具和浏览器产物。详细诊断保留在本地安全证据中，不向公共PR发布原始扫描日志。

### E1 / E2 合并验收

- PR #49 head `cdfd9d6c64d0b44e6b0c4bdacc8ade4f96a1738b`，合并 `e78cb8e94eea704a40c0f6398817fa8d5770ec55`；CI [36680160084](https://github.com/dreamvm/one-hub/actions/runs/36680160084) / [36680160286](https://github.com/dreamvm/one-hub/actions/runs/36680160286) 全9项成功。
- PR #50 head `9927f60adf3ab9b9f665bb8c2632782d2998fdc6`，合并 `9d8ae590f7126e115a4aec694b2fd4a0f00af0d1`；CI [36680928227](https://github.com/dreamvm/one-hub/actions/runs/36680928227) / [36680928525](https://github.com/dreamvm/one-hub/actions/runs/36680928525) 全9项成功。
- 两项各48业务PASS、四种Compose、三数据库各245事务叶子用例，候选/合成/实际合并树一致。runner镜像依次为 `sha256:9433514a36cf98cbf772bc684c1bd73b2ddf23764c8a708c3bd2353b651f600d`、`sha256:438333369435418e8f30837c46254e0daa557a7346a5f892000a6a1de6453306`，未发布。

### D5 容器工具链候选

- 分支 `codex/container-go-toolchain` 依赖PR #51。旧Dockerfile与CI日志证实构建镜像为Go1.25.0；runner和本机的1.25.14不代表最终候选。Go1.25.0版本对照扫描另命中37条标准库公告调用链，不能将此数量视为37条已验证可利用漏洞。
- 官方registry核实Go1.25.14多架构索引摘要，固定builder到该摘要；旧新amd64注解均为buildpack-deps:trixie-scm，但基础镜像digest不同，不宣称系统库完全相同。保留CGO、Linux静态链接与现有runtime。
- 构建显式设置GOTOOLCHAIN=local（旧官方镜像配置已继承此值，显式声明避免以后丢失），三处setup-go固定1.25.14。没有升级语言版本或改应用模块依赖。
- 新CI从不可变候选image ID创建未运行、无网络容器，提取/one-api，读取实际Go build metadata；严格核对Go版本、one-api主包、Linux/amd64/CGO，输出binary SHA256，失败阻断后续业务检查并清理容器。
- 旧模板的两个策略回归先失败，原“不发布”控制通过；新检查器用真实strip后的本机小程序保留正常控制，并拒绝旧版本期待、错误主包/OS/架构/CGO、非Go文件与缺文件。策略、race、vet、20项Python夹具、actionlint通过。
- fresh agent创建受会话线程数上限限制；复用两个未参与彼此工作的只读代理分别调查和独立审阅。审阅未提出可验证绕过或回归，父任务核对最终diff；不把该限制隐去或称为fresh review。
- 候选准备时本机无Docker，准确候选提取与三数据库/升级回退交由CI执行，最终结果见下方D5合并验收。仅amd64候选进入本次验收；arm64、未来发布产物、所有依赖公告和Go支持周期仍单独核对，未发布或部署。
- GO-2026-6222 已按源码和受控执行判为当前业务路径no_change：仅DecodeConfig，不进入完整VP8L Huffman分配；六项配置/合法WebP控制与媒体race通过。没有改依赖，也没有声称该包全部公告已消除。

### E3 合并验收

- PR #51 head `982405137c3c93a3068aa3b14610113ffcdf2ec7`，合并 `7e9fa7612c0c5c52c62fdc7d447729d7cc76f476`；候选/合成/实际合并树一致。
- CI [36681486287](https://github.com/dreamvm/one-hub/actions/runs/36681486287) / [36681486831](https://github.com/dreamvm/one-hub/actions/runs/36681486831) 全9项成功，48业务PASS、四种Compose、三数据库各245事务叶子用例。
- runner镜像 `sha256:227c5aa42cbb8266829b5ac11158c78bcb36b700bb944ba3721dbf3769ef7e00` 未发布；RC8仍未创建。

### D5 合并验收

- [PR #52](https://github.com/dreamvm/one-hub/pull/52) head `996569dc61da2875031eac24af054325abc63dc4`，实际合并 `a85842ce070c2601c2786b0a7c0d5ffb056ee62a`；候选、GitHub合成合并、本地从当前main计算的合并及实际合并树均为 `000eedeb196900d8c250dd5a90aeb500d67f7b86`。
- 准确候选CI [36683089028](https://github.com/dreamvm/one-hub/actions/runs/36683089028) / [36683089760](https://github.com/dreamvm/one-hub/actions/runs/36683089760) 全9项成功；核对日志确认48项业务PASS、四种Compose启动，以及SQLite/MySQL/PostgreSQL各245个事务叶子用例。
- 从runner候选 `sha256:72b66f2e1cfb4a5b9c4619e519b9f490d8d414a6ea7e978160d1eac57199d902` 提取的实际程序确认为 `Go=go1.25.14 main=one-api GOOS=linux GOARCH=amd64 CGO_ENABLED=1`，binary SHA256为 `952f4f8cbebcc37cf6b360f04c2098cc7dc718ff3ca18d9ea644683d27894dd9`。
- SQLite/MySQL/PostgreSQL业务、真实Redis故障、升级和两条回滚路径通过。该证据仅对应上述源码和隔离amd64镜像，不能替代生产或未来发布产物验收；镜像未推送，RC8未创建，未部署。

### D6 邮件依赖修复与验证边界

- [GO-2025-3988](https://pkg.go.dev/vuln/GO-2025-3988)涉及go-mail把解析后的原始地址写入SMTP信封命令。应用验证邮件、密码重置、额度提醒和通知共用`StmpConfig.Send`；公开入口使用的邮箱校验允许合法quoted local part，不能据此排除该问题。
- 本地STARTTLS模拟服务直接记录`StmpConfig.Send`产生的MAIL FROM/RCPT TO和DATA。旧0.6.2在发件人/收件人边界、空格、转义引号和反斜杠五项失败，普通地址、plus tag、显示名以及AUTH/RCPT/DATA失败控制通过。没有连接真实邮件服务器或证明某个真实MTA发生了重路由。
- 升级到0.7.2：0.7.1虽修复信封编码，却会丢失显示名；0.7.2的官方修复使用地址副本，正常显示名得到保留。必要模块闭包同时将x/text提升至0.29.0、x/sync提升至0.17.0；完整模块图还将x/tools提升至0.36.0、x/mod提升至0.27.0，并移除旧x/telemetry节点。后三项不在`go list -deps .`的主程序编译依赖中。没有改变应用SMTP配置或TLS策略。
- 升级后11项叶子用例race通过；8项地址/显示名控制和3项失败回执控制均检查真实协议结果。测试CA只在子进程中启用，生产信任链不变。专项使用`go test -race -count=1 ./common/stmp -run '^TestSMTPEnvelope'`；禁止无过滤执行会读取外部配置的旧`TestSend`。
- 相关model/types/providers/requester离线race、SMTP/通知/cache vet、策略检查和providers/relay构建通过；目标包govulncheck不再报告GO-2025-3988，其他依赖公告仍分别核实。全仓库准确提交CI和隔离镜像结果记录在本修复PR中，本段本地记录不替代远端验收。
- 新鲜只读调查与另一名新鲜只读审阅完成，未确认存活绕过或回归。回退此依赖会恢复旧地址编码问题；RC8仍仅预留，未发布或部署。

### D7 Bedrock事件流解析边界

- 独立分支`codex/bedrock-eventstream-dependency`基于D6候选`cd011d32b9713b8afdd5cd7af9f4a1be94a506c7`；顺序为D6验收合并后再合并本项。
- [GO-2026-5764](https://pkg.go.dev/vuln/GO-2026-5764)涉及EventStream未知头值类型panic。Bedrock上游响应在`Recv`启动的独立goroutine中解码，请求入口的恢复无法捕获；同一边界缺少`:exception-type`还会产生nil调用。来源是配置的上游响应，不把它表述为客户端可以直接注入任意上游帧。
- 先以真实`RequestStream/Recv`和内存响应体复现两类进程崩溃，其余10项正常/错误控制通过。升级eventstream至1.7.8及必要smithy-go至1.24.2；缺失异常类型在调用前明确返回错误，保持既有错误封装。
- 新增12组回归：覆盖全部10种合法头类型、连续Unicode事件、246种非法类型在首个/后续头的位置，以及异常、错误、JSON/base64、CRC和EOF控制。所有用例race通过，未调用真实模型。
- Bedrock vet、providers/relay构建、types/Gemini/Claude/requester race、工作流策略与actionlint通过；新增专项接入CI。新鲜只读独立审阅未发现具体绕过或回归，并另行通过专项race/vet/构建。
- 目标包govulncheck不再报告GO-2026-5764；其他公告保持分别核实。此次仅关闭两类解析崩溃，不宣称覆盖所有截断/流终止语义。回退会恢复崩溃路径；准确提交CI和隔离镜像验收以本批PR记录为准。

### 依赖可达性核实：PostgreSQL驱动

- GO-2026-5004：当前pgx 5.7.5仍在公告版本范围内，但本次检查的应用路径无需改动。`model/main.go`同时设置`PreferSimpleProtocol=true`与`PrepareStmt=true`，GORM普通/事务查询显式prepare，pgx优先使用已准备语句；没有发现关闭此设置或包含dollar-quoted占位符的运行时SQL。
- 临时PostgreSQL 18对照通过：当前配置在普通查询与事务中均保留SQL字面量和独立参数；关闭prepare的测试对照会复现旧驱动的字面量错误替换。只运行无害SELECT，临时数据库已关闭，未访问生产。
- 结论是当前受检路径`no_change`，不是依赖已修补，也不排除未来查询/配置变化后的风险；动态探针与完整依据保存在本地安全验收证据中。

### D6 合并验收

- [PR #54](https://github.com/dreamvm/one-hub/pull/54) head `cd011d32b9713b8afdd5cd7af9f4a1be94a506c7`，合并 `13541bb502e42f49c25cc229373ea2de0f612daa`；候选、GitHub合成、当前main计算和实际合并树均为`9b3fcdbb6037ff0942abce6aa0b0213c20990056`。
- CI [36693881691](https://github.com/dreamvm/one-hub/actions/runs/36693881691) / [36693882358](https://github.com/dreamvm/one-hub/actions/runs/36693882358) 全9项成功；日志确认48项业务PASS、四种Compose启动，以及SQLite/MySQL/PostgreSQL各245个事务叶子用例。
- 实际候选程序为Go1.25.14、one-api、linux/amd64、CGO1，SHA256 `1daf9f1a7db59cdbe87727c846e7ce41b8ae7d36dba83c4cddc0e21ab1302fcf`；runner镜像`sha256:d127e9d490c68bb9da3a273bf5a9fcaa5e6789b8325e7fb4dec9ce80e5650b56`仅本地加载，未发布。升级及两条回滚路径通过，未发送真实邮件或部署生产。

### 依赖可达性核实：Markdown与OIDC

- GO-2026-5208：当前gomarkdown版本的直接Smartypants调用可以越界/panic，但邮件正文的`Renderer.Text`先转义再处理；HTML节点不经过Smartypants。另一处标题处理需要`CompletePage`，应用未启用且邮件subject单独传给SMTP。
- 离线验证直接依赖的panic及多读一个哨兵字节；应用相同parser/renderer选项的15组输入全部完成，覆盖裸/不完整尖括号、实体、反斜杠、代码、HTML、分段和中文通知，正常链接/列表/格式化保持。未调用真实SMTP。结论为当前路径`no_change`，不是库已修补，也不代表HTML安全性整体验收。
- GO-2026-4945：当前go-jose/v4版本仍受JWE解密公告影响；应用OIDC使用`ParseSigned`和JWS签名验证，没有`ParseEncrypted/Decrypt/KeyUnwrap`调用。模块级全部符号标记不能证明JWE解密业务可达。
- 离线验证依赖在空wrapped key下panic；当前OIDC verifier拒绝正常/畸形compact JWE、JSON JWE、伪装有效claims的五段输入及附加分段，合法RS256签名身份通过。使用临时密钥和静态公钥，无真实OIDC连接。结论为当前路径`no_change`，不关闭issuer归属/subject唯一性等既有业务阻断项。
- 三项`no_change`均有父代理源码核对、独立只读调查和动态探针；证据保存在本地安全验收集合。未来查询模式、渲染选项或令牌解密能力变化时重新检查。其余x/image、x/text、x/net、gRPC、OTel及前端公告仍开放，不能将本批结果写成依赖已全部安全。

### D7 合并验收与下一依赖项

- [PR #55](https://github.com/dreamvm/one-hub/pull/55) head `88ada2a567affcde8c64722e652ab1dfede2ff20`，合并 `bda4c40acca99fe0a667721b46accb7d30b3cb77`；在D6已合并基线上重新计算，候选、合成、新计算及实际合并树均为`2b110d259cb5dc5587dd10fa05787693396ee241`。
- CI [36695527963](https://github.com/dreamvm/one-hub/actions/runs/36695527963) / [36695529036](https://github.com/dreamvm/one-hub/actions/runs/36695529036) 全9项成功；48项业务PASS、四种Compose和三数据库各245个事务叶子用例通过。
- 实际候选程序Go1.25.14、one-api、linux/amd64、CGO1，SHA256 `8f82651389e24391aa976fff69292706f70c3aa23578e4e0edc3e39ad066db99`；runner镜像`sha256:24dbdd008ea13b412c0ab4ab27242876832da352ea2398bc3b29acf56ffaadb2`未发布。未创建RC8、未部署。
- 下一项优先核实GO-2026-6348：VertexAI IAM客户端进入gRPC接收队列，SDK允许较大的接收消息；TLS身份验证不能替代分片内存边界。先做有界分片与正常响应对照，再评估1.83.1及其模块闭包，不能以真实OOM作为验收目标。GO-2026-6061涉及的xDS服务端RBAC及服务端流重置路径未在当前应用中发现，不能由此扩大为所有gRPC公告均不受影响。

### D8 gRPC 接收分片边界候选

- 基线为已合并`f5a77ef14509f11d04092d7e789fb50193a9f0ab`，该主分支CI [36698841110](https://github.com/dreamvm/one-hub/actions/runs/36698841110)成功；独立分支`codex/grpc-receive-buffer-boundary`处理[GO-2026-6348](https://pkg.go.dev/vuln/GO-2026-6348)。VertexAI聊天、原生Gemini/Claude和图片请求共用IAM取令牌路径；数据须来自通过TLS身份验证的IAM对端，普通请求用户不能直接提交该对端的响应帧。
- gRPC从1.73.0升级到官方修复版1.83.1，仅接受其必需MVS闭包；完整模块图157个节点变化，其中22个属于当前主程序编译依赖。Google API/IAM、auth、genproto、protobuf、OTel及x/*等版本变化是闭包约束，testify同步至1.11.1；不是157个库全部进入程序。Go声明规范化为1.25.0，实际工具链仍固定1.25.14。
- 独立临时依赖副本的64KiB接收队列探针先证明旧版在1/2/7字节分片下越过2048对象上限，正常16KiB分片对照通过；新版race下分别保留1024/576/103个对象，正常对照仍为4，内容和EOF顺序保持。此证据验证实际接收队列组件，不是完整IAM线上攻击或真实OOM演示；探针及旧新日志保存在本地安全验收集合。
- 新增7项离线IAM SDK兼容性回归并接入CI：真实SDK构造与内存gRPC连接，精确核对服务账号/Scope、token/expiry、大响应连续性、默认Unavailable重试、权限/认证错误、调用中取消与超时。旧新依赖均通过；GAX可能直接返回context错误，断言保留此语义。测试不调用Google，不改变应用凭据、TLS、代理、缓存或30秒后台上下文，也不宣称HTTP取消已传播到GetToken。
- 官方修复默认开启，但`GRPC_GO_EXPERIMENTAL_ENABLE_RECEIVE_BUFFER_COMPACTION=false`（忽略大小写）会关闭保护。独立进程负向对照已复现退化；仓库配置未设置该变量，生产环境未验收。运行时不得关闭此保护；本批不自动更改生产配置。
- providers/relay构建、VertexAI race/vet、核心/model、controller/middleware（含OIDC）、媒体、SMTP、Bedrock回归与vet、模块校验、工作流策略和actionlint通过；目标包govulncheck不再报告GO-2026-6348。独立审阅、准确候选CI及隔离镜像验收继续按流程执行，以本批PR最终记录为准。回退此提交将恢复旧依赖风险。
- x/text闭包只升至0.37.0，尚未达到GO-2026-5970修复版本0.39.0；其余依赖、OIDC业务归属和历史额度/支付阻断项分别保持开放。预留标签仍为`v0.14.27-dreamvm.1-rc.8`，未创建、未发布镜像、未部署。

### D8 合并验收

- [PR #57](https://github.com/dreamvm/one-hub/pull/57) head `2dc55b0736205aa3204c326ee5e63d30156aa26e`，合并 `75613fb0a977ccc127a883adb37674c27828d34c`；候选、GitHub合成、当前main计算及实际合并树均为`3729d8342a59e615f10bc3fdcaf88f6b6c1a5d41`。
- fresh只读独立审阅未发现具体绕过或回归，并独立通过离线IAM race/vet。准确候选CI [36701221389](https://github.com/dreamvm/one-hub/actions/runs/36701221389) / [36701221897](https://github.com/dreamvm/one-hub/actions/runs/36701221897) 全9项成功。
- 日志确认48项业务PASS、四种Compose启动、SQLite/MySQL/PostgreSQL各245个事务叶子用例、Redis故障、升级与两条回滚路径通过；前端56项测试通过，lint零错误/9条既存警告。
- 实际程序Go1.25.14、one-api、linux/amd64、CGO1，SHA256 `c7c7358fcd3b1b6d8ec1d67f9057d94481c37197a85ce4a4f97a2351ada5c67c`；runner镜像`sha256:6c35683d0415bef557494783a77404d7195455f5584efdaa6eacd2d1bac46021`仅加载在隔离环境，未发布。预留RC8仍未创建，未部署；生产不得关闭接收缓冲合并保护，真实运行配置另行验收。

### 依赖可达性核实：文本规范化

- GO-2026-5970：当前x/text 0.37.0仍包含有缺陷的`norm.Iter`；父任务及独立只读调查核对Darwin arm64、Linux amd64/CGO1实际主程序编译包，均未发现外部`norm.Iter`消费者。IDNA调用`norm.Form`方法，precis/afero使用transformer，cases使用属性查询；不能从共享属性函数的宽泛受影响符号集合推断迭代器业务可达。
- 有界探针验证库缺陷：NFC/NFKC、Init/InitString四种形式在非法UTF8夹具下均无法在128次迭代内结束，四项合法重音文本对照通过；临时0.39.0模块下八项均通过。没有实际挂死或OOM；仓库依赖未因此额外升级。
- 实际`mediaHostname`的4项正常对照及24项非法UTF8变体均有界返回；旧新模块的规范化结果/错误完全一致。部分非法字节会映射为ASCII punycode，不能写成全部拒绝。域名经同一规范化值执行DNS/IP检查及TLS，拨号固定到校验后的IP；现有IDNA/SNI正常控制race通过。
- 此项结论是当前受检应用路径`no_change`，不是依赖已修补或全部输入安全；未来新增Iter/排序消费者时重新检查。私有探针、旧新日志和两平台实际编译包证据已保留；下一项继续核实x/image剩余公告及前端依赖，再推进业务阻断项。

### 依赖可达性核实：WebP 尺寸读取

- 受检基线`dd14dc08f29bff7b3724edfa6e819e7260e57ce6`，x/image 0.28.0，Go1.25.14 Darwin arm64。[GO-2026-5061](https://pkg.go.dev/vuln/GO-2026-5061)与[GO-2026-4961](https://pkg.go.dev/vuln/GO-2026-4961)经父任务源码核对、独立只读调查和动态对照，结论仅为当前受检业务路径`no_change`。业务通过`GetImageSize`调用`image.DecodeConfig`读取尺寸，不进入完整像素解码；依赖本身仍受公告影响。
- 11类合成输入分别经inline Base64、直接HTTP及Worker传输，旧版和临时x/image 0.43.0各33个业务叶子用例按预期通过race；正常VP8/VP8L/VP8X+alpha与PNG、畸形/截断输入均有对照，HTTP响应体关闭及请求次数符合预期。既有媒体、TLS/代理/Worker和Base64回归通过。
- 完整解码阳性控制在旧版构造不匹配画布后像素访问panic，临时修复版拒绝；合法匹配画布对照均通过。该完整解码路径不由当前业务尺寸查询调用。旧尺寸API仍在三个入口接受`65536×32768`，临时修复版拒绝；相邻`65536×32767`两版均接受。这是验证行为差异，不是32位损坏图片panic复现，也不能写成旧版已拒绝超大尺寸。
- 未完整解码超大头，完整解码夹具小于一百万像素；没有读取真实远端图片、生产数据或调用付费模型。当前维护架构为64位，32位支持和未来完整解码消费者需重新分析；实际arm64镜像运行仍待验收。私有探针与旧新版本/媒体日志保留，仓库未因本项改变x/image依赖。
- D8闭包后的go-jose/v4为4.1.4，已达到GO-2026-4945修复版；上文Markdown/OIDC的`no_change`是此前受检基线结论。当前依赖扫描仍有其他公告，需继续逐项区分模块存在、包/符号可达和业务动态证据；OIDC issuer/subject业务约束及其余发布阻断项保持开放。
- 顶层Vite开发工具专项已完成合并验收（见D9）；其余Go与前端依赖继续逐项核对。预留RC8仍未创建标签，未发布镜像、未部署；本调查与台账补录不代替全部Go依赖或最终候选验收。

### WebP 台账补录合并核对

- [PR #59](https://github.com/dreamvm/one-hub/pull/59) head `b0d2bb58726fec3e35e906375908f9032968832a`，合并 `8c6ff760bbf12c268c10a1c24ae8ba920a827025`；候选、GitHub合成、合并前main计算及实际合并树均为`f6648d584dd50c14866675a6029ec7ab1f3ad9e9`。
- 准确候选CI [36726182894](https://github.com/dreamvm/one-hub/actions/runs/36726182894) / [36726183967](https://github.com/dreamvm/one-hub/actions/runs/36726183967) 全9项成功。隔离smoke日志为三后端41项检查（SQLite 9、MySQL+Redis 16、PostgreSQL+Redis 16）及7项升级/两条回滚检查，合计48个PASS，另有四种Compose启动通过。
- 本PR只补录已完成的WebP受检路径结论，没有改变x/image依赖；CI成功不把`no_change`改写为依赖已修复，也不关闭其余Go公告或实际arm64镜像验收。

### D9 Vite 合并验收

- 独立分支`codex/vite-dev-file-boundary`；[PR #60](https://github.com/dreamvm/one-hub/pull/60)最终head `377844081bc9f5c1d5b199242637fa25e0c9e31e`，实际合并 `3da2b6d3a525cce66e8091742006d34aacfd71d4`。在PR #59已合并基线上核对，候选、GitHub合成、合并前main计算及实际合并树均为`43f30e9cd8d8be311e3d3e4513c59d24dc32b65c`。
- 顶层Vite 7.1.11升至7.3.5，plugin-react 4.3.1升至4.7.0及必要锁文件闭包；Vitest自己的Vite 7.3.6没有被当作旧版复现对象。原始锁文件隔离副本中17个边界叶子失败、13个正常或拒绝控制通过；修复版30个叶子通过。测试使用合成文件、localhost与随机端口，覆盖HTTP query、WebSocket及sourcemap边界，不读取真实秘密。
- 新鲜只读独立调查与独立候选审阅完成，未确认存活绕过或兼容性回归；本地frozen安装、边界回归、UI、lint和build通过。准确候选CI [36729060363](https://github.com/dreamvm/one-hub/actions/runs/36729060363) / [36729060822](https://github.com/dreamvm/one-hub/actions/runs/36729060822) 全9项成功，Linux日志确认30个边界叶子（Node汇总31项包含父测试）、56项UI测试、lint零错误/9条既存警告，构建仍有既存大chunk提示。
- SQLite/MySQL/PostgreSQL各245个事务叶子用例通过。三后端smoke共41项检查（SQLite 9、MySQL+Redis 16、PostgreSQL+Redis 16），另有7项升级/两条回滚检查，合计48个PASS；四种Compose实际启动通过。Redis停止、错误类型、旧余额和恢复属于已覆盖的隔离场景，请求中途故障与网络分区仍分别开放。
- 从最终runner镜像`sha256:80e32a0ebf337d16c61a28e9549af1bdf74645dace51c673c55eda5a2e3347b4`提取的实际程序为`Go=go1.25.14 main=one-api GOOS=linux GOARCH=amd64 CGO_ENABLED=1`，binary SHA256 `570a11a580b502bfef0ad2d78c36b4c66ca6cbb9bed89802a9bfd76c666a0988`。镜像只在runner加载，未发布；未复制生产数据，真实模型调用为0。
- Windows ADS/短文件名和UNC editor修复只核实了上游版本/源码，未做Windows/NTFS原生或NTLM验收；其余浏览器运行依赖、开发工具公告与发布阻断项保持开放。撤销本批依赖提交会恢复旧版风险；RC8仍只预留，未创建标签、未发布镜像、未部署生产。
- 验收台账[PR #61](https://github.com/dreamvm/one-hub/pull/61) head `dfdb4043c5baf83ac4fd374973912b6e302a90b1`合并至`cce1a31041d67917a97841cc995975dcdfe8d2fa`，候选与实际合并树均为`5173fb9f82a51715714fe894fe66fe32e73bf49f`；准确候选CI [36733361810](https://github.com/dreamvm/one-hub/actions/runs/36733361810) / [36733362454](https://github.com/dreamvm/one-hub/actions/runs/36733362454) 全9项成功。此文档提交成为D10已核实基线，不替代后续源码候选验收。

### D10 Axios 浏览器依赖候选

- 分支`codex/axios-browser-config-boundary`在PR #61验收台账通过全9项CI并合并后更新至main基线`cce1a31041d67917a97841cc995975dcdfe8d2fa`。Axios 1.12.2固定升级至1.18.0，仅更新必要锁文件闭包，未改变业务API或错误拦截器。
- 独立子进程对浏览器发行产物验证合成前置污染；旧版四个无请求体方法在适配器前报validator TypeError，继承reviver改写JSON。五项失败均在候选消失，15项正常控制两版通过；总20个叶子，Node汇总21项包含XHR父测试。未证明应用污染来源，不能写成旧版已外发继承请求体。
- 另有真实API工厂4项控制，包含401清理与LoginCheckAPI拒绝；本地Node22.20/Yarn1.22.22 frozen安装、30项Vite边界、20项Axios边界/XHR、56项既有UI加4项API、lint零错误/9条既存警告和build通过，仍有大chunk提示。
- 新鲜只读调查与另一位候选审阅完成，未确认存活绕过或兼容性回归；审阅者独立复跑核心、XHR及API控制，并检查等价继承形状和Fetch变体。新鲜依赖审计已无Axios匹配记录，其余包公告保留，审计匹配不能代替应用可利用性判断。原生浏览器、完整页面及生产验收尚未完成；准确提交CI与合并事实见下方D10合并验收。RC8仍未创建，未发布或部署。

### D10 Axios 合并验收

- [PR #62](https://github.com/dreamvm/one-hub/pull/62)最终head `93d58cb9a7d7add43b3c22e448b0a588468493ad`，实际合并 `49ddd66ec3826d2b440ea23dd79104ba0680d0d1`。在PR #61已合并基线上核对，候选、GitHub合成、合并前main计算及实际合并树均为`985c2565a5c26223e7b8efee311441a3355a5c42`；独立审阅结论与旧新动态对照保持上述范围。
- 准确候选CI [36735528407](https://github.com/dreamvm/one-hub/actions/runs/36735528407) / [36735528944](https://github.com/dreamvm/one-hub/actions/runs/36735528944) 全9项成功。Linux前端日志确认30个Vite与20个Axios的Node原生runner叶子（各自汇总31/21项包含父测试），Vitest共60项（56项既有UI与4项真实API工厂控制），lint零错误/9条既存警告，build通过并保留既存大chunk提示。
- SQLite/MySQL/PostgreSQL各245个事务叶子用例通过。三后端smoke共41项检查（SQLite 9、MySQL+Redis 16、PostgreSQL+Redis 16），另有7项升级/两条回滚检查，合计48个PASS；四种Compose实际启动通过。现有隔离Redis停止、错误类型、旧余额与恢复检查不代替请求中途故障、网络分区或并发负载专项。
- 最终runner镜像`sha256:89304e82d84fad61cb38d96ec7e0adde1888fd580996023ecd6b0a72eaed761a`提取的实际程序为`Go=go1.25.14 main=one-api GOOS=linux GOARCH=amd64 CGO_ENABLED=1`，binary SHA256 `21024bb5067ca5bcf4426cc17352d3fae9c0ba82f329015e4d92237c095c274b`。镜像仅在隔离runner加载，未发布；未复制生产数据，真实模型调用为0。
- 升级后的新鲜审计仍有238条路径记录、111个Yarn advisory ID及85个唯一GHSA，Axios匹配为0。此计数是依赖路径匹配，不是238个已证实应用漏洞，也不代表全部前端依赖已安全。ReactRouter/Monaco当前只做源码路径核对，尚无动态边界验收；原生浏览器、Windows、实际arm64镜像及计费/身份/故障发布阻断项继续开放。
- 下一项核对PostCSS开发工具边界：当前8.5.6仍匹配4个GHSA、12条审计路径记录，拟评估8.5.23。只读调查及脱离业务的合成初探不等于Vite业务回归、候选升级、独立审阅、CI或合并完成。回退D10依赖提交会恢复旧库缺陷；RC8仍仅预留，未创建标签、未发布镜像、未部署。

### D11 PostCSS 编译器依赖候选

- 独立调查后，在 `codex/postcss-map-boundary` 将锁定 PostCSS 8.5.6 升至 8.5.23，并更新必需 nanoid 闭包至 3.3.19；不更换 Vite、不新增直接依赖或 resolutions。
- 旧版 7 个边界检查失败、9 个正常对照通过；新版 16 个叶子检查通过。测试覆盖真实顶层 Vite 生产构建/import 和独立库边界，普通 CSS、CSS Modules、SCSS、资源与合法 map 保持，见 [POSTCSS_FILE_BOUNDARY.md](POSTCSS_FILE_BOUNDARY.md)。
- Vite 自身的开发加载器仍会异步读取直接 CSS map 注释指向的合成文件；默认关闭开发 sourcemap 的探针未见内容进入返回 map，不能据此关闭所有 Vite 文件读取或其他配置的披露风险。Windows、符号链接 realpath、原生浏览器与完整端到端尚未验证。
- 本地 frozen 安装、30 个 Vite / 20 个 Axios / 16 个 PostCSS 叶子检查及 60 个 Vitest 检查、lint 零错误/9 条既存警告、build 通过。用户授权的新审计有 217 条路径记录、104 个 Yarn 公告 ID、78 个唯一 GHSA；PostCSS 的 4 个和 nanoid 的 3 个 GHSA 不再匹配，其他公告保留，不将审计计数当成应用可利用性结论。
- 新鲜只读候选审阅者独立复跑 16 项及相邻路径/受信任正常控制，未确认阻断项；分派消息包含父任务测试结果与范围解释，因此不是盲审，保留此流程限制。准确提交 CI、合并与镜像验收事实见下方。RC8 仍预留，未创建标签、发布或部署。

### D11 PostCSS 合并验收

- [PR #64](https://github.com/dreamvm/one-hub/pull/64) 最终 head `60aa55eab6270992dbd1426c9715531f30005351`，实际合并 `04ea23dbc3cf45a0f6d1f0477a0cd0c3dd755634`。候选、GitHub 合成、合并前 main 计算及实际合并树均为 `8ae8d22ab43ac126f4378b64572d8a85093719e2`。更新基线时只解决台账追加位置，已审阅源码、测试和依赖内容保持一致。
- 准确候选 CI [36742294716](https://github.com/dreamvm/one-hub/actions/runs/36742294716) / [36742295282](https://github.com/dreamvm/one-hub/actions/runs/36742295282) 全 9 项成功。Linux 前端通过 30 个 Vite、20 个 Axios、16 个 PostCSS 叶子检查与 60 个 Vitest 检查；Node 汇总分别为 31 和 38（包含父节点）。Lint 零错误/9 条既存警告，build 通过并保留大 chunk 提示。
- SQLite/MySQL/PostgreSQL 各 284 个测试节点、245 个事务叶子用例通过。三后端 smoke 共 41 个 PASS（9/16/16），另有 7 个升级/两条回滚 PASS，合计 48；四种 Compose 实际启动通过。未覆盖的请求中途故障、网络分区及并发负载不因此关闭。
- 最终 runner 镜像 `sha256:1e5350b05eee550c11ab5bd5a4c76eef0460d83b487976bbae67db36505d5408` 中实际程序为 `Go=go1.25.14 main=one-api GOOS=linux GOARCH=amd64 CGO_ENABLED=1`，binary SHA256 `2ed56f7aaf528acb063b511b77d8d529f884a2644b552cb922fda7e50ac734cf`。镜像仅在 runner 加载，未发布；无生产数据复制或真实付费模型调用。
- 已合并主分支的 PostCSS 后审计为 217 条路径、104 个 Yarn ID、78 个唯一 GHSA，其他依赖公告保持开放。Vite 独立异步 map 加载器、Windows 原生、符号链接 realpath、浏览器及生产端到端仍未完成；受检 PostCSS 路径通过不代表整个 Vite 文件读取边界关闭。只读审阅有上述非盲审上下文限制，不写成无偏差完成全部审阅规范。
- 前置 Axios 台账 [PR #63](https://github.com/dreamvm/one-hub/pull/63) head `8d19b57a6ab4aaa788e6640b9ecea1ee5bfcdb42` 合并至 `8d885bda002429de279faeba555ff236a57ddc6e`，候选及实际树 `2bc0610d4081dd57f615845bf61cdac899802a8a`；[36740013903](https://github.com/dreamvm/one-hub/actions/runs/36740013903) 与 [36740015255](https://github.com/dreamvm/one-hub/actions/runs/36740015255) 第 2 次尝试合计全 9 项成功。后者首次在 Go 模块代理 HTTP/2 下载错误处失败，随后对同一提交重跑；失败与成功证据分别保留。
- 下一项 Rollup 仅有本地候选 `codex/rollup-output-boundary` / `42988901fad12c44bdedfda1455bfe4ba4537037`：4.59.0 的 20 项与完整前端检查通过，但独立审阅被平台内容检查中断、无结论，未创建 PR、未完成候选 CI、未合并。其 214 条路径/77 个 GHSA 的本地审计不能替代已合并主分支数据；当前主分支 Rollup 仍为 4.53.3。此项保持待审阅，其余可独立工作继续。RC8 远端标签仍未创建，未发布或部署。

### D11 台账合并与 D12 React Router 当前路径

- PostCSS 台账 [PR #65](https://github.com/dreamvm/one-hub/pull/65) head `3e7bcef9e7b22b7cb9d752745c4b68203916e87f` 已合并至 `34d36a135d43235bb542a19c52eaafe69e2b2d60`。候选、GitHub 合成、本地计算及实际合并树均为 `947badba3a7659ef3a3ae254bf66a95cf2637ecd`；[36745622940](https://github.com/dreamvm/one-hub/actions/runs/36745622940) / [36745623855](https://github.com/dreamvm/one-hub/actions/runs/36745623855) 全 9 项成功，不以 PR #64 的 CI 代替。
- PR #65 隔离镜像实际提取程序为 Go1.25.14 / one-api / linux/amd64 / CGO_ENABLED=1，二进制 SHA256 `4ee19406a9629f9c125c0c863dfc0d50c632805d424e8e404b1ab96ba3bf0ac8`，镜像 ID `sha256:6835031b982b411fb13a4dd3c58de04daa4e8fc5ba3192c1b83a7b6857640d44`；SQLite/MySQL/PostgreSQL 41 条与升级/回滚 7 条 PASS、4 种 Compose 模板成功。镜像仅在 CI runner 加载，未发布。
- 在该前端源码与锁文件上完成真实 Chrome/BrowserRouter 合成账单检查，详见 [FRONTEND_BROWSER_BOUNDARIES.md](FRONTEND_BROWSER_BOUNDARIES.md)。桌面和移动视口共 24 个日期输入组合均保持同源；正常查看/返回/侧栏/前进后退、浅深色通过。异常日期仍有空白路由及坏编码警告，列入页面行为专项，不写成所有页面状态通过。
- React Router 6.21.3 / router 1.14.2 未升级，12 条审计路径保持开放；结论仅为受检业务路径 `no_change`。Monaco/DOMPurify 动态编辑器验收、Rollup 缺失审阅及其余依赖继续推进；这次局部浏览器证据不替代生产端到端、实际 arm64 镜像或最终 RC8 验收。未创建标签、发布或部署。

### D13 Babel 合并验收

- [PR #67](https://github.com/dreamvm/one-hub/pull/67) head `0095e8dfe30b6b66ea37129543e5963406275f1f`，实际合并 `7ee5307882d96eeb2c77350038be76552b3a6815`。候选、GitHub 合成、合并前 main 计算及实际合并树均为 `e0c4925b9bd535573f84c8b20e44e462f22483d9`。交接原目录的 Babel 分支与 5 个暂存文件保留，交付在独立工作树完成；不要再次提交原暂存候选或覆盖原目录。
- Babel 7.24.3 → 7.29.7 的既有旧新对照为 12 个边界失败、16 个控制通过 → 28 个叶子全部通过。全新只读候选审阅未提供原调查结论、修复理由或通过声明；审阅者独立重跑通过，并确认符号链接不受词法检查保护、少见 `..` 开头合法 map 文件名被丢弃的限制。未发现当前业务消费者或交付阻断；不称为完整文件系统沙箱。
- 本地 Node 22.20.0 / Yarn 1.22.22 冻结安装、30 个 Vite / 20 个 Axios / 16 个 PostCSS / 28 个 Babel 叶子、60 个 Vitest、lint/build 及 Go 1.25.14 工作流策略通过。准确候选 CI [36759790568](https://github.com/dreamvm/one-hub/actions/runs/36759790568) / [36759791654](https://github.com/dreamvm/one-hub/actions/runs/36759791654) 全 9 项成功；Linux Node 汇总 31 与 67 项包含父节点。Lint 零错误/9 条既存警告，build 保留大 chunk 提示。
- SQLite/MySQL/PostgreSQL 各 284 个测试节点、245 个事务叶子通过；三后端 smoke 9/16/16 条及升级/两条回滚 7 条，合计 48 个 PASS，4 种 Compose 实际启动通过。未覆盖的请求中途故障、网络分区及并发负载仍开放。
- 最终 runner 镜像 `sha256:3a0d7620cacc6f6c615a7fd839cd90dd02e2d137bff87be64f25de2c9d3dc383` 中实际程序为 `Go=go1.25.14 main=one-api GOOS=linux GOARCH=amd64 CGO_ENABLED=1`，binary SHA256 `b6fc9bf9e4f51e69dbb2577c5f230044e5d383421b6902817f006a145ef174ee`。镜像仅在隔离 runner 加载，未发布；无生产数据复制或付费模型调用。RC8 仍仅预留。
- 合并后 main `7ee53078` 的 [36761306821](https://github.com/dreamvm/one-hub/actions/runs/36761306821) 兼容性 CI 也已成功。该检查与候选的全 9 项分别记录，不替代彼此。

### 接续：Rollup 审阅门槛、最新审计与 Monaco 证据

- 原 Rollup 候选 `42988901` 保留；交付候选 `79316d4c6db9ffaae5d62e863facc188c7833487` / 草稿 PR #68 承接已合并的 PostCSS 与 Babel，台账冲突以最新验收记录为准，测试入口同时保留 Babel/Rollup。更新基线后的冻结安装、114 个依赖叶子、60 个 Vitest、lint/build 通过。全新只读独立审阅再次被平台内容检查中断，无结论；不得用父任务源码核对、本地通过或 CI 代替该门槛，不合并。
- 精确候选的手动 CI [36760618740](https://github.com/dreamvm/one-hub/actions/runs/36760618740) / [36760626211](https://github.com/dreamvm/one-hub/actions/runs/36760626211) 全 9 项成功；Linux Node 汇总 31/88（含父节点），三数据库各 284 个节点。草稿由 Babel 前置分支改为 main，候选树与当前 main 计算的合并树均为 `739d8148c17b447d085625387d614d89cd726723`；原分支不改写。此为 workflow_dispatch 的准确提交证据，不宣称 PR 事件的检查已运行；未来源码或合并树变化必须重验。
- Rollup 隔离 smoke 同样为 48 个业务/升级回滚 PASS 和 4 种 Compose 启动。最终 runner 镜像 `sha256:7794de952f88a15143ae2dd5ba27b8f16918dd522e89608efda59d3d684913b9` 的实际程序为 Go1.25.14 / one-api / linux/amd64 / CGO_ENABLED=1，binary SHA256 `2eed35d976d83bc2350298333ed8176b9bf4d68fc3bf47850bb06b82a219426d`；未发布。CI 通过不消除独立审阅门槛。
- 新鲜 Yarn 审计最初因依赖树外发被自动审批拒绝；用户随后明确授权当前任务及修复后的必要复查，仅向官方 npm/Yarn 服务发送公开依赖元数据。核实 Yarn 1.22.22 固定接收端 `registry.yarnpkg.com/-/npm/v1/security/audits`，字段为包名/版本/完整性哈希、依赖关系和 dev 标志，无源码、凭据或运行配置。公开候选 `79316d4c` 上新结果为 219 条路径、114 个 Yarn ID、88 个唯一 GHSA，Babel/Rollup 匹配为零，Axios 匹配增至 12 个；仍需逐项判断实际浏览器/Node 调用路径，不把匹配数量当作应用漏洞数。
- 恢复并核验了基线 `34d36a13` 的 Monaco 局部浏览器证据及 15 个资源哈希，见 [FRONTEND_BROWSER_BOUNDARIES.md](FRONTEND_BROWSER_BOUNDARIES.md)。加载、model 夹具后的格式化及模拟保存有证据；完整键盘输入和诊断 hover 可见性未通过，不能用 DOM 文本或 model API 代替。没有升级 Monaco/DOMPurify，真实 CDN、移动端与生产验收继续开放。

### 接续：台账合并与 Monaco 运行版本候选

- 上述台账 [PR #69](https://github.com/dreamvm/one-hub/pull/69) head `075d26b7fb1953e631f1c919a4c605f9bb25915f` 的 [36762293812](https://github.com/dreamvm/one-hub/actions/runs/36762293812) / [36762294422](https://github.com/dreamvm/one-hub/actions/runs/36762294422) 全 9 项成功；实际合并 `3a74e083f806c5440afe1cb6e1663496862c80a3`。候选、合成、计算与实际合并树均为 `fa55a4742f137ebd882f3583bb932a436a64352b`；合并后 [36764012954](https://github.com/dreamvm/one-hub/actions/runs/36764012954) 成功。
- 在该 main 上建立独立 `codex/monaco-runtime-boundary`，不依赖未合并的 Rollup。Monaco 精确锁定 0.57.0（内嵌 DOMPurify 3.4.15），共享本地 ESM/worker 配置统一三个 JSON 编辑器，避免 loader 独立加载旧 CDN；细节及可利用性限制见 [MONACO_RUNTIME_BOUNDARY.md](MONACO_RUNTIME_BOUNDARY.md)。
- 旧版实际请求 0.55.1 CDN；候选真实页面的旧 CDN 请求为零，JSON/core worker 为同源文件。三个入口正常键入、格式化、映射/列表错误拒绝、一次合成渠道保存以及深色模式的输入/撤销有新证据。零间隔自动化输入、真实剪贴板、hover 裁剪、移动端及生产流程不因此关闭。
- 本地 frozen 安装、94 个依赖叶子与 62 项 Vitest、lint 零错误/9 条既存警告及 build 通过。新鲜审计 205 条路径、72 个唯一 GHSA；DOMPurify 剩一条依赖 IN_PLACE/hook 的公告，受检调用没有该前提但保留跟踪。
- 全新独立只读审阅确认直接加载候选存在渠道列表提前加载 Monaco 的资源回归；父任务改为既有 Loadable/lazy 模式并重验加载时机及编辑器流程。按单次审阅周期，随后添加的懒加载边界未被独立复审，保留流程限制。准确候选 CI 与合并仍待完成；未创建标签、发布镜像、部署或付费调用。

### Monaco 合并验收

- [PR #70](https://github.com/dreamvm/one-hub/pull/70) head `6e84fdf0663dafba4af65a30840620e5a3d29b7c` 已合并为 `4603ed139c3be1353e99c7260b70dd97a314670b`。候选、合成、最新 main 计算与实际合并树均为 `0e49356a2e26252ee7b055057561edd499814344`。
- [36766394971](https://github.com/dreamvm/one-hub/actions/runs/36766394971) / [36766395689](https://github.com/dreamvm/one-hub/actions/runs/36766395689) 全 9 项成功，94 个依赖叶子、62 项 Vitest、lint/build、三数据库各 284 个节点、48 个业务及升级回滚 PASS 和 4 种 Compose 启动均有准确候选证据。镜像与实际程序身份见 [MONACO_RUNTIME_BOUNDARY.md](MONACO_RUNTIME_BOUNDARY.md#合并验收)。
- 单次审阅覆盖限制、余下 DOMPurify IN_PLACE 公告、hover 裁剪、移动与最终端到端仍开放。Rollup PR #68 保持独立审阅受限、未合并；不把此 Monaco 交付或后续构建链迁移算作 Rollup 候选审阅通过。RC8 未创建标签；没有发布或部署。
- 合并后准确 main 的 [36768041402](https://github.com/dreamvm/one-hub/actions/runs/36768041402) 已成功。

### Vite 独立 source map 候选

- 从 Monaco 已合并 main `4603ed13` 建立独立分支；采用 Vite 8.3.1 / plugin-react 5.2.0 / Vitest 4.1.11，上游同时控制依赖外部 map 和 sources 补读，保留原 JS 浏览器目标。范围及迁移影响见 [VITE_SOURCE_MAP_BOUNDARY.md](VITE_SOURCE_MAP_BOUNDARY.md)。
- 旧版 9 项边界失败、6 项正常对照通过，候选 15 项通过；本地全部 109 个依赖叶子、62 项 Vitest、lint（0 错误/9 既存 warning）及构建通过。实际静态页面三个 JSON 入口与深色模式、同源 worker、模拟保存及开发热更新有证据。
- 新鲜官方审计 191 条路径、69 个唯一 GHSA。候选独立审阅、准确 CI 与合并未完成；锁文件移除 Rollup 不等于未合并 PR #68 获得审阅结论。RC8 标签与发布步骤仍未执行。
- 新鲜独立候选审阅被平台内容检查中断，无审阅结论；未改写、绕路或重试。继续正常开发页面/worker 验收及准确候选 CI，保留为待审阅草稿，不合并。
- 后续 [草稿 PR #71](https://github.com/dreamvm/one-hub/pull/71) head `9c90385c665440e1016e6b9eb4c970b626763d46` 的 [36769982936](https://github.com/dreamvm/one-hub/actions/runs/36769982936) / [36769983549](https://github.com/dreamvm/one-hub/actions/runs/36769983549) 全 9 项成功；109 个依赖叶子、62 项 Vitest、三数据库各 284 个节点、48 个业务/升级回滚 PASS 和 4 种 Compose 启动有准确候选证据，实际镜像/程序身份见专项文档。独立审阅仍无结论，因此未合并，main 仍为 Vite 7.3.5。

### i18n 工具链首批静态分流

- [I18N_TOOLCHAIN_TRIAGE.md](I18N_TOOLCHAIN_TRIAGE.md) 逐条保留 19 条输入路径，18 条在当前调用缺少公告必要前提，1 条 `lodash-es.unset` 路径仍为待验证；未升级依赖，不能将静态不适用写成库已修补。
- 当前翻译 CLI 使用普通 JSON 解析，没有 LangChain load、LangSmith Hub/anonymizer 或流式 handler；浏览器仅导入静态语言文件。自动 tracing 是否在真实环境启用未读取，也未授权外发。
- 下一项继续合成语言文件差异/正常控制及其余 i18n 公告，再处理 Axios 1.18.0 新增 12 条公告；官方当前 Axios 1.20.0 的修复与兼容性尚未完成本项目验收。没有执行真实模型、凭据读取、标签、镜像发布或部署。

### 依赖路径后续核对与台账验收

- 上述台账 [PR #72](https://github.com/dreamvm/one-hub/pull/72) head `05319927743b0a8116f8815ce93ff6f7729e3300` 已合并为 `5f0a7fc8139757ec31bfb297806a7bd6ab9e5690`。候选、合成、本地计算和实际合并树均为 `15f9878523987aa50e2bf2343e0f8683afbd5c05`；[36771742772](https://github.com/dreamvm/one-hub/actions/runs/36771742772) / [36771743406](https://github.com/dreamvm/one-hub/actions/runs/36771743406) 全 9 项及合并后 [36773447676](https://github.com/dreamvm/one-hub/actions/runs/36773447676) 均通过。
- 该台账候选通过 94 个依赖叶子、62 项 Vitest、三数据库与 48 个隔离业务/升级回滚检查、4 种 Compose；最终镜像 `sha256:a1b7dc156714d1a7c4f17558f438a2e66b0e07a3c47cff981658f12319555294` 的程序为 Go1.25.14 / one-api / linux/amd64 / CGO1，binary SHA256 `d9317368ae075b9f60b6a93bbd3d6d702e9be7b0d446615f1f8540d1b94ed29e`。未发布。
- i18n 后续 56 条静态记录为 36 条当前路径不适用、20 条待核实，逐条保留在 [I18N_TOOLCHAIN_TRIAGE.md](I18N_TOOLCHAIN_TRIAGE.md)。007 的合成 JSON 对照未见原型标记删除，普通对象控制通过，但数组缩短控制失败；四份当前语言文件无数组。未据此宣布库安全或实施推测性补丁。
- [AXIOS_NEW_ADVISORIES.md](AXIOS_NEW_ADVISORIES.md) 记录新增 12 条公告的实际浏览器路径；必要的 Node、表单序列化、请求拦截器或 fetch/重定向边界不在当前产品调用中。结论仅为静态 no_change，保留 Axios 1.18.0 版本匹配，不声称已升级至 1.20.0 或新漏洞动态验收通过。
- Rollup/Vite 独立审阅仍受平台限制，无结论，草稿不合并；i18n 可选 Markdown/glob/debug 边界继续开放。下一独立专项为 Realtime 重复/缺失用量、异常结束与恢复规则，先调查真实路径和正常计费控制。RC8 仍未创建，正式发布阻断项未全部关闭。

### 依赖台账合并与 Realtime 响应收据候选

- [PR #73](https://github.com/dreamvm/one-hub/pull/73) head `3af16b9d9815550edfef05ac4b50ab8d5fbe07f7` 已合并为 `8105887756030a843064396cb278c7ffefd74625`；候选、GitHub 合成、本地计算及实际合并树均为 `bcaa0c7fa7ed47da6294ebbd98bd6ea908324d79`。[36774578516](https://github.com/dreamvm/one-hub/actions/runs/36774578516) / [36774578737](https://github.com/dreamvm/one-hub/actions/runs/36774578737) 全 9 项通过；合并后 main 检查仍待核对。
- PR #73 隔离 smoke 为 48 个业务/升级回滚 PASS、4 种 Compose 启动。最终 runner 镜像 `sha256:dc8844a58203ecbd12391175302f957c73c8d32db4007970008e6773b929128f` 的实际程序为 Go1.25.14 / one-api / linux/amd64 / CGO1，binary SHA256 `7afe1e5f9e8d1d6be14155e73b5767a72e611606a041f86ac41858241d70a558`；未发布。
- 独立分支 `codex/realtime-response-accounting` 在该 main 上处理同一连接内已标识响应的重复/冲突用量，详见 [REALTIME_RESPONSE_RECEIPTS.md](REALTIME_RESPONSE_RECEIPTS.md)。旧版 4 个失败、2 个正常对照通过；候选相关专项 53 个叶子通过，项目规定与受影响包 race、策略、vet、编译通过。
- 全新只读独立调查与候选审阅完成，审阅未发现当前范围内存活绕过或阻断回归。六文件源码/测试补丁 SHA-256 `f4cfb306723a487432a6c4dd15ea9d4a69481a6adfd2c3531ff49185240ab94b` 在文档基线更新后保持不变。准确候选 CI、PR 和合并待完成，本地通过不是交付完成。
- 缺失/null usage、匿名响应、跨连接/崩溃恢复、有歧义金额规则仍开放。首帧错误路径的正 extra/audio 费用另列下一独立修复；不与当前收据修复混为一项。没有标签、镜像发布、生产变更或真实付费调用。

### Realtime 响应收据合并与首帧收尾候选

- PR #73 合并后 main 的 [36776820608](https://github.com/dreamvm/one-hub/actions/runs/36776820608) 已通过，更新前节的待核对状态。
- 响应收据 [PR #74](https://github.com/dreamvm/one-hub/pull/74) head `ba4df83279e04e3c73e50f4d8876f80be2356e30` 已合并为 `b01e0ffc7a0032efab7dd8571bd679a3e5813b00`。候选、合成、本地计算及实际合并树均为 `23fd0ebbd9da03f67fe8894817e7a700d0103782`；[36777026553](https://github.com/dreamvm/one-hub/actions/runs/36777026553) / [36777027193](https://github.com/dreamvm/one-hub/actions/runs/36777027193) 全 9 项通过。合并后 main 检查待核对。
- PR #74 最终 runner 镜像 `sha256:9ca73b69bd7a402e1c3e281d867059517f40adc1ff9698a884bd75d264244819` 的实际程序为 Go1.25.14 / one-api / linux/amd64 / CGO1，binary SHA256 `89821ff73a1f1bb5b70263ab3d7b8ca5d05eb3d132d747155bd8858be3d39239`。48 个隔离业务/升级回滚 PASS 与 4 种 Compose 启动通过；未发布。
- 后续独立首帧修复见 [REALTIME_FIRST_USAGE.md](REALTIME_FIRST_USAGE.md)，从上述已合并 main 交付。旧版 9 个失败/3 个正常对照，候选 12 场景及 5 个内部表示控制通过；相关 39 个叶子、规定离线与受影响包 race、策略、vet、编译通过。首帧已报告明细用量时停止退款重试；无效计价保留预留，免费计费语义不变。
- 全新只读调查完成；新建候选审阅者遭工具线程数上限，使用未参与本项实现的既有审阅者执行一次独立只读审阅，复核旧新对照、模型与用量算术后未发现阻断项。上下文复用的流程限制保留，不声称 fresh-context 审阅。四文件补丁 SHA-256 `445f665d89cc90d9283e6e0eecaec25a1b276dbb4a2cc1c9c1d2c532407c4b32`，更新基线未改变补丁。
- 首帧候选的准确 CI、PR 与合并尚待完成；缺失用量、运行中崩溃恢复、历史歧义金额等未关闭。RC8 仍未创建标签，未发布或部署。

### Realtime 首帧修复合并与缺失用量恢复位置

- PR #74 合并后 main 的 [36778589455](https://github.com/dreamvm/one-hub/actions/runs/36778589455) 已通过。
- 首帧修复 [PR #75](https://github.com/dreamvm/one-hub/pull/75) head `4819089f9f514975a46d64d0d0c79220b287d500` 已合并为 `8351770c6eb044f45be0fc3eb3faf3aff27ba49f`。候选、合成、本地计算及实际合并树均为 `0dc9b65293f11f215a5f603b1b1d92b9adf1ad64`；[36778738593](https://github.com/dreamvm/one-hub/actions/runs/36778738593) / [36778739287](https://github.com/dreamvm/one-hub/actions/runs/36778739287) 全 9 项通过。合并后 main 检查仍待核对。
- 48 个隔离业务/升级回滚 PASS 和 4 种 Compose 启动通过。最终 runner 镜像 `sha256:e4935127c30e0397ecee14019a0e06c3bdc138c21e314cfee8ffe75f423ddd64` 的实际程序为 Go1.25.14 / one-api / linux/amd64 / CGO1，binary SHA256 `67ca3a8acf27592625810a9401ab9e4dda1bada6c9adc82ec8cc02e615c2c415`；未发布。审阅者上下文复用限制见前节，不因 CI 通过而删除。
- 下一独立分支 `codex/realtime-missing-usage`：源码与只读独立调查确认，供应商 `response.done` 缺失/null usage 会跳过计费回调，最终以零或部分累计生成确定终局。临时副本的首帧和已有正累计后缺失/null 共 4 个场景失败；显式 `{}` 零用量、完整用量、无工作握手 3 个对照通过。调查复用未参与本项的既有 agent 上下文，保留容量限制说明。
- 当前只保留失败回归与已知边界，尚无缺失用量修复。下一步是供应商专有未知标记、停止重试及持久待核对证据；不得把部分金额写成最终费用，也不得把未知记录交给自动终局恢复。通用异常断连、运行中崩溃窗口和历史核销仍需各自证据。

### Realtime 缺失用量候选

- PR #75 合并后 main [36780297419](https://github.com/dreamvm/one-hub/actions/runs/36780297419) 已成功，更新前节待核对状态。
- 已实现供应商专有缺失报告标记、停止重试及 `reconcile` 待核对状态，详见 [REALTIME_MISSING_USAGE.md](REALTIME_MISSING_USAGE.md)。保留预留和已知部分证据，不写成最终消费/退款，不进入自动终局恢复。
- fresh-context 独立审阅发现 response 整体缺失/null 的提前报错绕过；父任务复现 4 个失败、3 个控制通过后修正，原 `invalid_response` 保留。最终 94 个专项叶子、规定回归、vet、编译通过。审阅后修改未独立复审的单次周期限制见专项文档。
- 当前尚未创建 PR、未完成准确候选 CI 或合并。新增列的 MySQL/PostgreSQL 迁移、通用异常结束、运行中崩溃和历史核对仍待各自证据；本地通过不等于交付完成。RC8 仍只预留。
- 后续 [PR #76](https://github.com/dreamvm/one-hub/pull/76) 首候选 `f7690802` 的 CI 在 PostgreSQL 迁移夹具失败：同一池删列再加列留下旧 `SELECT *` 执行计划。SQLite/MySQL 及其余检查通过；已修正夹具为旧结构建立后新连接迁移，全部准确候选 CI 需重跑，未合并。

### Realtime 缺失用量合并与未完成响应候选

- PR #76 最终 head `bee84d6c4b5b784c75d7e50fc862337464d89c53` 已合并为 `51590891071dd8626ae13a9913285022a8ba79e3`；候选、合成、计算与实际合并树一致。准确候选 [36783144118](https://github.com/dreamvm/one-hub/actions/runs/36783144118) / [36783144323](https://github.com/dreamvm/one-hub/actions/runs/36783144323) 全 9 项通过，三数据库各 289 节点/249 事务叶子（含旧账本加列迁移）通过，48 个隔离业务/升级回滚 PASS、4 种 Compose 启动通过。镜像/程序身份详见 [REALTIME_MISSING_USAGE.md](REALTIME_MISSING_USAGE.md#合并验收)，未发布。
- 合并后 main [36784595991](https://github.com/dreamvm/one-hub/actions/runs/36784595991) 已成功。首轮 PostgreSQL 夹具失败及单次审阅后的修改范围限制继续保留，不能把最后成功反写成首轮成功。
- 下一独立分支 `codex/realtime-unfinished-response` 处理供应商已观察开始但未取得对应完整用量的收尾，见 [REALTIME_UNFINISHED_RESPONSES.md](REALTIME_UNFINISHED_RESPONSES.md)。旧版 5 个失败/4 个正常对照；候选 123 个专项叶子、规定回归、vet、编译通过；全新预调查和全新候选独立审阅完成，未发现受检范围内阻断项。快进已合并 main 后十文件源码补丁摘要保持不变。
- 此候选尚无准确 CI/PR/合并。开始事件缺失的非完整流、客户端已发送但供应商尚未可见的窗口、崩溃前证据和历史核销规则仍开放；不声称 Realtime 异常结束已全部关闭。RC8、正式发布、生产步骤仍未执行。

### 后续 Realtime 草稿保留与历史核对规则

- [PR #77](https://github.com/dreamvm/one-hub/pull/77) 已以 head `236d2b5b2d5f8ee820c7c4a3236019e321c2fed9` 合并为 `f5c1ad53ef3e2d4b078d55404fc6e4ac4098cca5`，候选/预合并/实际树均为 `52072836cc8de7d0bd054196c2b41f5f951fa110`。候选 [36821487199](https://github.com/dreamvm/one-hub/actions/runs/36821487199) / [36821487446](https://github.com/dreamvm/one-hub/actions/runs/36821487446) 全 9 项及合并后 main [36822827773](https://github.com/dreamvm/one-hub/actions/runs/36822827773) 成功。
- 输出进度 [PR #78](https://github.com/dreamvm/one-hub/pull/78) head `11a0e3762fc2d8a0b440a30cb7e631414e387616` 已通过 [36823355438](https://github.com/dreamvm/one-hub/actions/runs/36823355438) / [36823355908](https://github.com/dreamvm/one-hub/actions/runs/36823355908) 全 9 项；48 个隔离业务/升级回滚和 4 种 Compose 通过，未合并。后续无效报告审阅发现类型错误早退的残余表示，原进度审阅未覆盖，故 #78 改为草稿。
- 无效报告 [PR #79](https://github.com/dreamvm/one-hub/pull/79) head `26465acdbdeed55c5d98f2912442ef51fc5543e9` 依赖 #78。旧版 10 个失败/6 个控制，候选 281 个专项叶子及规定回归通过；全新独立审阅被平台内容检查中断，无完整结论。父任务基于源码修正中间报告指出的早退路径，既有回归重跑通过，但未重试受限审阅或新增变体，不能据此关闭审阅缺口。手动候选 CI [36824874581](https://github.com/dreamvm/one-hub/actions/runs/36824874581) / [36824879034](https://github.com/dreamvm/one-hub/actions/runs/36824879034) 已启动；草稿保持未合并。
- 本分支从已合并 main 独立整理 [HISTORICAL_ACCOUNTING_RECONCILIATION.md](HISTORICAL_ACCOUNTING_RECONCILIATION.md)，明确历史预留、支付、补偿的证据、归属和可处理状态。未读取生产账本，未执行或批准任何历史资金调整；真实逐笔核对仍开放。
- Rollup/Vite 原有平台审阅限制继续保留；不重复或绕过受限步骤。RC8 仍未建标签，未发布镜像、部署或执行真实付费调用。只暂停依赖缺失证据的交付，继续独立的规则和身份等整改工作。

### 历史规则交付与 OIDC 共享保存候选

- [PR #80](https://github.com/dreamvm/one-hub/pull/80) head `c91160a66a1d94a533843262b65bddafc825398f` 已合并为 `5fb760c608a86df0690534ba884279b79744068d`；候选、GitHub 合成、计算及实际树均为 `111dbde99bbc888e9eb43073fd347b81e419a396`。[36825235366](https://github.com/dreamvm/one-hub/actions/runs/36825235366) / [36825235601](https://github.com/dreamvm/one-hub/actions/runs/36825235601) 全 9 项通过；48 个业务/升级回滚 PASS，实际 runner 镜像 `sha256:736476d9c9b062da56cabc72722b356cc8a9a9e0b48666c098eb82d2a852cba7`，程序 Go1.25.14 / one-api / linux/amd64 / CGO1，binary SHA256 `938a6d084a358927daf9d7a5a1f0f5829963dcb0fde576487bccd83b8d66edd0`。合并后 main [36826859753](https://github.com/dreamvm/one-hub/actions/runs/36826859753) 正在运行；未读写生产账本。
- PR #79 head `26465acd` 的上述两次手动准确候选 CI 均已成功，48 个业务/升级回滚 PASS；镜像 `sha256:63e2b37c183307a628be11fbc768491049b54b4c55df0920ee8956bd5d04bcea`，程序 Go1.25.14 / one-api / linux/amd64 / CGO1，binary SHA256 `16a09b4e86bee6e167fce50f60fc2498dc96abb7f345b950e7b401beb22f674f`。CI 不补足独立审阅结论；#78/#79 继续草稿未合并，未重试平台受限审阅或新增变体。
- 独立候选 [OIDC_STALE_UPDATE_BOUNDARY.md](OIDC_STALE_UPDATE_BOUNDARY.md) 仅阻止普通 User.Update 旧快照恢复/覆盖已变更的 OIDC 绑定。旧版四个失败/两个正常控制，候选 44 个专项叶子、规定回归/vet/编译和新鲜独立审阅通过；资料/密码、明确解绑及新注册保留。管理员通用资料 JSON 中的 oidc_id 也被忽略；没有新增重新绑定接口。准确候选 CI/PR/合并待完成。
- 用户已明确选择 OIDC 保守迁移：历史 issuer 未知的记录保留待核实，不自动关联；不以当前配置或首次登录回填，也不任意挑选重复账号。issuer 持久化、subject 唯一性及并发注册仍待独立实现与验证；当前没有修改生产身份数据。RC8 未创建，发布阻断项仍开放。

### OIDC 前置修复交付与 issuer 候选

- PR #80 合并后 main `5fb760c6` 的 [36826859753](https://github.com/dreamvm/one-hub/actions/runs/36826859753) 已成功，取代上节正在运行的历史状态。
- [PR #81](https://github.com/dreamvm/one-hub/pull/81) head `86e6a6601a1e3076deba0e43c34a10f4c0c818a5` 已合并为 `416a52ea533131d1f64f29efb3764c9c775bb489`，候选/合成/计算/实际树 `b0383f636fb9efb8ec360a84beeac6f7f286628b`。准确候选 [36827130791](https://github.com/dreamvm/one-hub/actions/runs/36827130791) / [36827131193](https://github.com/dreamvm/one-hub/actions/runs/36827131193) 全 9 项及合并后 main [36828389683](https://github.com/dreamvm/one-hub/actions/runs/36828389683) 成功。48 个隔离业务/升级回滚 PASS，镜像与程序身份见 [OIDC_STALE_UPDATE_BOUNDARY.md](OIDC_STALE_UPDATE_BOUNDARY.md#合并验收)；未发布。
- 后续独立 [OIDC_IDENTITY_BOUNDARY.md](OIDC_IDENTITY_BOUNDARY.md) 候选保存已验证 issuer/精确 subject 和 nullable 唯一身份键，防止跨 issuer 归属混用和重复注册；历史未知归属保持原值，不自动关联。旧版五个失败/四个正常控制；候选本地 59 个专项叶子及完整规定回归/vet/编译通过。真实 MySQL/PostgreSQL 验收仍等待准确候选 CI。
- 全新只读独立审阅未报告具体问题；父任务之后的完整回归和确定性交错测试发现首轮查询与新注册之间的时序回归，已最小修正并重跑。按单次周期，后续修正未再独立复审，明确保留这一覆盖限制。尚未完成该候选 PR/CI/合并，不把 PR81 的绿色检查替代它。
- 旧程序回退会恢复 subject-only 登录，并不理解新身份键；即使可读取 schema，也不能宣称 OIDC 回退安全。生产升级/回滚与历史身份恢复仍需独立核实，当前没有生产读写。原目录 Babel 暂存工作继续保留；RC8 未建标签，发布阻断项仍开放。

### OIDC issuer 交付与请求中途 Redis 验收

- [PR #82](https://github.com/dreamvm/one-hub/pull/82) 最终 head `eb625210c639186450c709ac313ed571e67ad940` 已合并为 `4ba29b48f9629512ce7170a509a3383739387b07`，候选/合成/计算/实际树均为 `7c02f9b827c6e308bd5d1eae10339697a687755a`。[36829774827](https://github.com/dreamvm/one-hub/actions/runs/36829774827) / [36829775344](https://github.com/dreamvm/one-hub/actions/runs/36829775344) 全 9 项通过，包含三数据库 OIDC 归属/迁移/并发注册、48 个隔离业务/升级回滚 PASS 和四种 Compose 启动；实际镜像/程序身份见 [OIDC_IDENTITY_BOUNDARY.md](OIDC_IDENTITY_BOUNDARY.md#合并验收)。合并后 main [36831183355](https://github.com/dreamvm/one-hub/actions/runs/36831183355) 尚待核对。
- 初次 PostgreSQL 夹具失败及修正、独立审阅后的并发查找修正未再次复审，均如实保留；不将初次候选或最终全部补丁写成独立复审通过。保守历史迁移规则已实现，真实身份核实与恢复仍未执行。
- 下一独立 [请求中途 Redis 验收](REDIS_INFLIGHT_ACCEPTANCE.md) 仅扩展隔离 smoke：在 JSON 返回前或上游首个 SSE 内容块后，停止测试自己的 Redis，检查有限/无限令牌的一次结算与恢复。本地夹具和准确候选 CI 待最终验收；数据库故障、网络分区、并发负载仍开放。
- Rollup/Vite 及 Realtime 草稿的受限审阅步骤继续暂停，未重试或绕过。RC8 未创建标签，未发布镜像、部署或执行真实付费调用。

### Redis 中途故障交付与数据库恢复候选

- PR #82 合并后 main `4ba29b48` 的 [36831183355](https://github.com/dreamvm/one-hub/actions/runs/36831183355) 已成功，更新前节待核对状态。
- [PR #83](https://github.com/dreamvm/one-hub/pull/83) head `6ab65ba2ea7f56d5842112fa5714898012137ea9` 已合并为 `53be6eb04942814f812d756dfbd809bd553e1b66`，候选/合成/计算/实际树均为 `74f9bcb63e7731298121daabeb8e78d0f5809ed7`。[36831550015](https://github.com/dreamvm/one-hub/actions/runs/36831550015) / [36831550324](https://github.com/dreamvm/one-hub/actions/runs/36831550324) 全 9 项通过；两个数据库分别验证有限/无限令牌 × JSON/SSE 的 Redis 中途故障及单次结算恢复，共 56 个隔离业务/升级回滚 PASS、四种 Compose 启动通过。实际镜像/程序身份见 [REDIS_INFLIGHT_ACCEPTANCE.md](REDIS_INFLIGHT_ACCEPTANCE.md#合并验收)。合并后 main [36832999472](https://github.com/dreamvm/one-hub/actions/runs/36832999472) 待核对。
- 下一独立 [数据库中途故障候选](DATABASE_INFLIGHT_ACCEPTANCE.md) 保持网关进程存活，观察确定终局写入失败后恢复本次创建的数据库，核对现有恢复任务的一次结算。只增加验收，不修改应用逻辑；本地 Python/语法/策略通过，准确候选 CI 与真实数据库故障结果尚未完成。
- 意图尚未落库时同时崩溃、网络分区、并发负载和资源释放、真实历史数据及生产验收仍开放。RC8 未创建标签，没有发布、部署或真实付费调用。

### 数据库中途故障交付与网络分区候选

- PR #83 合并后 main `53be6eb0` 的 [36832999472](https://github.com/dreamvm/one-hub/actions/runs/36832999472) 已成功。
- [PR #84](https://github.com/dreamvm/one-hub/pull/84) head `29bc8064455d21455670e74edf9a1e1eb8e108ec` 已合并为 `9f0db48c6eb325f14417ae6645e007c0dfc42c92`，候选/合成/计算/实际树均为 `93debe9709620ad6bad54852c63ad990ec912efb`。[36833222590](https://github.com/dreamvm/one-hub/actions/runs/36833222590) / [36833223347](https://github.com/dreamvm/one-hub/actions/runs/36833223347) 全 9 项通过；两个数据库各四个实际终局写入失败与恢复场景通过，共 64 个业务/升级回滚 PASS、四种 Compose 启动通过，程序身份见 [DATABASE_INFLIGHT_ACCEPTANCE.md](DATABASE_INFLIGHT_ACCEPTANCE.md#合并验收)。合并后 main [36834793394](https://github.com/dreamvm/one-hub/actions/runs/36834793394) 待核对。
- 下一独立 [网络分区候选](NETWORK_PARTITION_ACCEPTANCE.md) 在测试依赖保持存活时断开本次内部网络，验证连接失败、进程连续、账务等待与恢复。目标限定本次随机前缀，恢复原 IP/别名；有限/无限令牌 × JSON/SSE × Redis/数据库矩阵待准确候选 CI。本地 25 项 Python、Go smoke/策略 race 和 vet 通过，不代表真实网络验收完成。
- 崩溃前未持久化用量、并发负载和资源释放、剩余页面与依赖、arm64 及生产事实仍需各自验收。受限审阅草稿继续未合并；没有 RC8 标签、镜像发布、生产变更或真实付费调用。

### 2026-10-01 PR #84 主分支与 PR #85 首轮失败补记

- PR #84 合并提交 `9f0db48c6eb325f14417ae6645e007c0dfc42c92` 的 [main CI 36834793394](https://github.com/dreamvm/one-hub/actions/runs/36834793394) 成功。
- PR #85 首轮候选 `647f7ffb` 8 项基础检查成功，真实镜像 smoke 在恢复网络原 IP 时因自动子网不支持显式 IP 失败。已修正为 Docker 选取后显式配置的本次内部子网；准确新候选 CI、实际网络矩阵与合并仍待完成。详见 [网络分区记录](NETWORK_PARTITION_ACCEPTANCE.md)。

### 2026-10-01 网络分区交付与指标上下文修复候选

- PR #85 修正候选 df235557 的九项检查成功，16 项实际网络分区、80 项业务/升级检查与四种 Compose 通过，合并 a95142550659965969b1306c91c4e7cb6aa1f488；候选/合成/计算/实际合并树一致。main CI 待核对，详细镜像身份和首轮失败见 [网络分区验收](NETWORK_PARTITION_ACCEPTANCE.md)。
- 并发预检确认 HTTP 指标异步闭包读取已回收 Gin Context，旧版 race 与计数失败；最小候选改为同步复制 method/path/status。正常对照、专项和相关回归通过；一次复用上下文的独立审阅发现重复测试累计值问题，已改精确增量并通过 -count=2。候选 CI、PR、合并仍待完成，见 [指标生命周期](METRICS_CONTEXT_LIFECYCLE.md)。
- 完整并发账务、资源释放和 SQLite 实际负载验收仍开放；临时模拟夹具及本地结果不等于交付。RC8 无标签，发布/部署/付费调用未执行。

### 2026-10-01 指标修复交付与三数据库并发候选

- PR #85 合并后 main CI 36838531843 已成功。
- PR #86 指标上下文修复候选 91b0e0bb 九项检查、80 项业务/升级与四种 Compose 通过，合并 5712dbb205b1544a130ee02b7ea1063058019296；四种树身份一致，实际程序身份见 [指标生命周期记录](METRICS_CONTEXT_LIFECYCLE.md)。main CI 待核对。
- 三数据库并发候选继承上述修复，覆盖各三轮32次、4worker、有限/无限 JSON/SSE、精确账务和上游/指标计数、有限资源排空。SQLite 为本次卷的固定只读查询，MySQL/PostgreSQL 增加活动会话核对。一次独立审阅发现失败后队列继续执行，已修正并通过异常/取消控制；本地38项Python和相关Go回归通过。准确候选镜像/CI、PR、合并仍待完成，详见 [并发验收](CONCURRENT_LOAD_ACCEPTANCE.md)。
- 首页网络失败后持续加载已在独立临时副本建立失败对照，尚未修改或交付；后续继续前端整改。RC8、正式版本、发布、部署及真实付费调用状态未改变。

### 2026-10-01 并发验收交付与首页加载候选

- PR #86 合并后 main CI36840850235成功。
- [PR #87](https://github.com/dreamvm/one-hub/pull/87) 最终f50b2756九项准确候选检查成功，三数据库九轮并发、89个业务/升级PASS及四种Compose通过，合并d190a5c45db4ccb81adde454447aa29266e48d5c，四种树一致；实际镜像和资源数值见[并发验收](CONCURRENT_LOAD_ACCEPTANCE.md#2026-10-01-合并验收)。main CI36843424536待核对。该有界验收不代表长期压测或生产容量。
- [首页加载候选](HOME_LOADING_LIFECYCLE.md) 修正已结束的失败请求仍无限加载，以及内容与错误译文混淆，保留正常/空配置。旧新回归、完整前端测试、构建和实际浏览器正常/失败/恢复检查通过；lint由9降到8条警告。独立审阅指出一项测试缺口，已以旧版失败/新版通过修正；未二次复审，准确候选CI/PR/合并尚未完成。
- 其余八条ESLint警告、异常日期空白路由、加载体积与关键页面端到端继续待办。受限草稿、真实历史数据归属、arm64和生产事实仍开放。RC8未建标签，未发布、部署或执行真实付费调用。

### 2026-10-01 首页交付与统计卡片候选

- PR #87 合并后 main CI36843424536成功。
- [PR #88](https://github.com/dreamvm/one-hub/pull/88) 首页加载修复fce0129d九项准确候选检查、89个业务/升级PASS和四种Compose通过，合并c784264f6b880aa2e05acceeed67a1368ec42b3a；四种树一致，实际镜像身份见[首页记录](HOME_LOADING_LIFECYCLE.md#2026-10-01-合并验收)。main CI36845885783待核对。
- 新[统计卡片候选](ANALYTICS_STATISTICS_LIFECYCLE.md) 修正三类失败后持续骨架屏，以新对象汇总成功响应并隔离过期结果。旧版2正常通过/3异常失败；新8项专项、完整78项Vitest及既有依赖回归、lint和构建通过；lint剩7条警告。一次复用上下文的独立审阅无具体发现，实际浏览器明暗/移动、成功/503/恢复、金额和原始配额/空数据已核对；尚未完成本项准确候选CI、PR和合并。
- 异常账单日期的原24组浏览器证据仍适用，相关源码未改变；新增组件回归2正常通过、6异常失败。账单修复、剩余警告、图表失败路径、移动日期布局和完整前端验收继续待办。RC8、受限草稿、arm64及生产事实等阻断项不变。

### 2026-10-01 统计卡片交付与账单日期候选

- PR #88 合并后 main CI36845885783 已成功。
- [PR #89](https://github.com/dreamvm/one-hub/pull/89) head1fc58da7九项准确候选检查、89个隔离业务/升级PASS与四种Compose成功，合并e71b1051fce5dd460e3854a6038c871dcee6cc25；候选/合成/计算/实际树一致。镜像及程序身份见[统计卡片记录](ANALYTICS_STATISTICS_LIFECYCLE.md#2026-10-01-合并验收)；main CI36848136097待核对。
- 独立[账单日期与路由候选](INVOICE_ROUTE_BOUNDARY.md) 对响应日期规范化，禁用无效日期入口，将异常详情路由恢复列表，未知路径显示现有404。旧版组件2正常通过/6异常失败、旧路由1正常通过/7异常失败；新版39项专项和完整117项Vitest、lint、构建通过。一次复用上下文独立审阅无具体问题，实际构建56组浏览器检查通过；本项准确候选CI/PR/合并仍待完成。
- 其余7条ESLint警告、图表失败/移动布局、详情返回译文、404无可用历史的返回行为、关键页面与加载体积仍待验收。受限草稿、真实历史归属、arm64与生产事实等阻断项不变；RC8未建标签，未发布、部署或真实付费调用。

### PR #90 首轮验收失败与修正

- 账单首轮df852422的117项Vitest断言通过，但Iconify在测试环境销毁后的异步回调产生未处理异常，兼容性frontend失败。已将新增两个账单测试中的装饰图标隔离为本地替身，应用源码不变；完整前端回归重跑通过，无未处理异常。此测试修改发生于独立审阅之后，未二次复审，准确新提交CI待完成。
- PR #89 合并提交e71b1051的main CI36848136097已成功，更新上节待核对状态。

### 2026-10-01 账单修复交付与模型排序候选

- [PR #90](https://github.com/dreamvm/one-hub/pull/90) 修正后b68c07d0全九项准确候选检查、89个隔离业务/升级PASS和四种Compose成功，合并eb6a46c7a0848d15d892bd91c4c332e48628b1ef；四种树一致。首轮Iconify测试异步回调失败与审阅后修正仍如实保留，镜像身份见[账单记录](INVOICE_ROUTE_BOUNDARY.md#2026-10-01-合并验收)。main CI36851033301待核对。
- [模型排序候选](SUPPORT_MODELS_ORDERING.md) 修正异步厂商元数据到达后的旧排序，并保留数字厂商名的正确ID顺序。旧版2正常通过/2时序失败；独立审阅发现数字键继承问题，新增正常控制先失败再修正，未二次复审。最终8专项、完整125项Vitest、lint和构建通过，警告剩6条；实际浏览器四组明暗/移动/数字与字母厂商检查通过。本项PR、准确候选CI与合并尚未完成。
- 用户组异步加载失败已建立1正常通过/1异常失败且有未处理拒绝的独立旧版证据，尚未修复。其余警告、页面行为、加载体积及发布阻断项继续开放；RC8未建标签，未发布、部署或真实付费调用。

### 2026-10-01 模型排序交付与用户分组候选

- PR #90 main CI36851033301成功。[PR #91](https://github.com/dreamvm/one-hub/pull/91) head1d3102eb九项准确候选检查、89个业务/升级PASS及四种Compose成功，合并c8d1ace99278cabc61b4f76ec9e2e2bb3c92a3db，四种树一致，程序身份见[模型排序记录](SUPPORT_MODELS_ORDERING.md#2026-10-01-合并验收)；main CI36853275946待核对。
- [用户分组候选](USER_GROUP_LOADING.md) 为现有Promise链补充拒绝处理，保持同步返回及登录协议。旧版1正常通过/1异步失败；新5专项、完整130项Vitest与依赖边界、lint、构建通过，警告剩5条。一次复用上下文独立审阅无具体发现，实际浏览器六组正常/503/重新访问恢复与明暗移动检查通过；本项准确候选CI、PR与合并尚未完成。
- 令牌列表旧请求覆盖新结果、过期错误与卸载后提示已在临时副本复现；正常两主题通过，修复仍未交付。其余警告、页面、加载体积、受限草稿、真实历史数据、arm64及生产事实等阻断项继续开放。RC8未建标签，未发布、部署或真实付费调用。

### 2026-10-01 用户分组交付与令牌列表候选

- PR #91 main CI36853275946已成功。[PR #92](https://github.com/dreamvm/one-hub/pull/92) head ea050744九项准确候选检查、89个业务/升级PASS及四种Compose成功，合并3835f1c67b47126e9de8ecc7b3750939b22b7a5e，四种树一致；程序身份见[用户分组记录](USER_GROUP_LOADING.md#2026-10-01-合并验收)，main CI36855902502待核对。
- [令牌列表候选](TOKEN_LIST_LIFECYCLE.md) 用effect生命周期隔离旧响应，保留普通/管理员筛选、排序分页参数。旧版2正常通过/3异常失败，新9专项及完整139项Vitest、依赖回归、lint和构建通过，警告剩4条；一次复用上下文独立审阅无具体发现，实际浏览器明暗移动的乱序、业务错误、搜索、分页及管理员筛选通过。本项准确候选CI、PR与合并尚未完成；共享拦截器全局HTTP错误/401副作用不在active保护范围。
- 运营设置保存失败误报成功、部分写入和刷新失败已在独立临时副本继续整改，尚未交付。其余警告、页面、加载体积、受限草稿、历史归属、arm64与生产事实继续开放；RC8未建标签，未发布、部署或真实付费调用。

### 2026-10-01 令牌列表交付与运营设置候选

- PR #92 main CI36855902502已成功。[PR #93](https://github.com/dreamvm/one-hub/pull/93) head c05d317f九项准确候选检查、89个业务/升级PASS及四种Compose成功，合并0f50ed877c91dd9b645c9005e00f388df1f052ff，四种树一致；程序身份见[令牌列表记录](TOKEN_LIST_LIFECYCLE.md#2026-10-01-合并验收)，main CI36858232247待核对。
- [运营设置候选](OPERATION_SETTING_SAVE.md) 避免写入失败误报成功，明确部分写入与刷新失败，整次保存管理loading且组JSON先校验。旧版正常控制通过、失败路径先失败；一轮复用上下文独立审阅发现真实刷新吞异常，父任务以真实Provider等六项失败建立证据后修正，新增StatusContext范围与后续修正未二次复审。最终23专项、完整162项Vitest及依赖回归、lint和构建通过，仍有4条警告；实际构建14组明暗移动合成保存检查通过。本项准确候选CI、PR和合并尚未完成。
- 价格单位多余依赖及ModelInfo选项props警告在独立临时副本处理中，未交付。运营设置初始读取生命周期、其他页面、体积、受限草稿、历史归属、arm64与生产事实继续开放。RC8未建标签，未发布、部署或真实付费调用。

### PR #94 首轮 CI 补记

- PR #93 合并后 main CI36858232247成功。PR #94首轮9c397a6e在隔离frontend中161项通过、多阶段保存测试超出默认5秒；另一套frontend成功，smoke未执行。仅优化该测试控件重复查询并设置15秒有限上限，保留业务断言，应用源码未变。修改未经第二轮独立复审；新提交验收仍待完成，不能记作已交付。

### 2026-10-01 运营设置保存交付与价格单位候选

- [PR #94](https://github.com/dreamvm/one-hub/pull/94) 修正head8e72a05f九项准确候选检查、89个业务/升级PASS、九轮并发及四种Compose成功，合并37f35b4d684bb088cd1c076acb07f2291a326682，四种树一致；程序身份见[运营设置记录](OPERATION_SETTING_SAVE.md#2026-10-01-合并验收)，main CI36861546856待核对。首轮测试超时和审阅后修正限制仍保留。

### 模型信息选项回调警告候选

- PR #94 main CI36861546856已成功。[PR #95](https://github.com/dreamvm/one-hub/pull/95) head cd225ef2九项准确候选检查、89个隔离业务/升级PASS、九轮并发及四种Compose通过，合并ca1b429c64c8fff6a80dc1f9463b434bcc85d143，四种树一致；程序身份见[模型价格记录](MODEL_PRICE_UNIT.md#2026-10-01-合并验收)，main CI36864331311待核对。
- 本项仅明确两个renderOption回调形参名称，清除两条props/key误判，事件及aria展开保持原样；旧目标lint失败、新目标通过，完整162项Vitest/依赖边界/lint/build通过，剩运营初始读取一条警告。一次复用上下文的独立只读审阅无具体发现，实际构建桌面浅色/手机深色验证鼠标、键盘、自定义输入和取消无写入。详见[模型信息记录](MODEL_INFO_OPTION_PROPS.md)。准确候选CI及合并未完成；其他生命周期/日期布局候选和发布门槛仍开放。

### 运营设置初始化读取候选

- PR #95 合并后main CI36864331311已成功。模型信息候选 [PR #96](https://github.com/dreamvm/one-hub/pull/96) 的e7f30627八项基础检查成功，隔离镜像验收仍运行；本项叠加于该提交，必须先交付#96，合并前核对最新main及树，不能把前置CI当作本项验收。
- 初始化读取函数稳定化并只合并返回字段，保留未返回的当前编辑和独立保存基线；卸载后忽略旧组件结果/错误并停止后续初始化读取。旧版2正常通过、3异常失败，新9读取/23保存专项与171项全套Vitest、依赖边界、格式/lint/build通过，叠加前置清理后ESLint零警告。一次复用上下文的独立只读审阅无具体发现，实际浏览器明暗主题共10组通过。已发送请求、全局API副作用及主动保存不属于本项guard；同名服务器字段仍覆盖编辑。见[初始读取记录](OPERATION_SETTING_LOAD.md)。尚无本项准确候选CI或合并，发布门槛不变。

### 统计概览图表请求生命周期候选

- [PR #96](https://github.com/dreamvm/one-hub/pull/96) head e7f30627九项准确候选检查、89个业务/升级PASS、九轮并发及四种Compose成功，合并e391ba683928f3958ba666477e39408884996188，四种树一致；镜像/程序身份见[模型信息记录](MODEL_INFO_OPTION_PROPS.md#2026-10-01-合并验收)。main CI36867428349待核对。
- 运营初始读取 [PR #97](https://github.com/dreamvm/one-hub/pull/97) head2651bd0c先叠加#96再改main；因初始基线不是main，手动运行准确候选[36866226110](https://github.com/dreamvm/one-hub/actions/runs/36866226110)/[36866231207](https://github.com/dreamvm/one-hub/actions/runs/36866231207)，八项基础检查成功、隔离smoke仍运行；不声称自动PR事件已验收。
- 本项叠加#97，查询序号防止过时结果和卸载副作用，失败/空结果清除旧图表并结束加载，全部数据转换完成后才更新；旧2正常通过/5异常失败，新11专项及182项全套Vitest/依赖边界/格式/零警告lint/build通过。一次复用上下文的独立只读审阅无具体发现，桌面浅色/手机深色真实ApexCharts共14组通过。源码/测试与已验收副本逐字一致，准确候选CI和合并仍待完成，日期/Grid/标题及其他发布门槛仍开放。见[统计图表记录](ANALYTICS_OVERVIEW_LIFECYCLE.md)。

### 日期输入响应式布局候选

- PR #96 main CI36867428349已成功。[PR #97](https://github.com/dreamvm/one-hub/pull/97) head2651bd0c的手动准确候选九项检查、89个业务/升级PASS、九轮并发及四种Compose成功，合并bbca75aacaea59c5e03d7dd156a5b0355a03ccb3；候选、最新main计算及实际树一致，旧叠加合成父节点未刷新但树相同，见[初始读取验收](OPERATION_SETTING_LOAD.md#2026-10-01-合并验收)。main CI36868758519待核对。
- [PR #98](https://github.com/dreamvm/one-hub/pull/98) head60883302承接统计图表生命周期，准确手动CI36867907042/36867916459运行中；本项叠加该提交，须先交付#98。
- 日期输入旧390px文字空间不足导致截断；改为小于1200px纵排、宽屏横排。初版600px切换经一次复用上下文独立只读审阅，无具体发现但要求实际断点验收；实测600/900仍截断后改1200，审阅后修正无第二轮复审，限制明确保留。最终182项全套/依赖边界/格式/零警告lint/build及9个宽度、4组明暗主题真实日期交互通过，日期协议不变。详见[日期布局记录](DATE_RANGE_RESPONSIVE.md)。尚无本项准确候选CI或合并；其他候选与发布门槛仍开放。
- [模型价格单位候选](MODEL_PRICE_UNIT.md) 仅移除筛选memo未读取的unit依赖，换算公式不变。旧目标lint警告、新目标零警告，一次复用上下文独立审阅无具体发现；完整162项Vitest及依赖边界、lint和构建通过，警告剩3条，实际明暗移动的K/M、按次价格、组倍率与搜索通过。本项准确CI、PR、合并仍待完成。
- ModelInfo选项警告与运营设置读取生命周期在独立临时副本已完成本地回归/审阅/浏览器，尚未交付；叠加后可达到lint零警告，不代表主分支已零警告。统计概览图表读取候选旧2正常通过/5异常失败，最小修正及182项完整回归/独立审阅通过，实际ApexCharts浏览器仍待完成；日期布局、Grid开发期警告等尚未处理。体积、其他页面、受限草稿、历史归属、arm64与生产事实继续开放；RC8未建标签，未发布、部署或真实付费调用。


### 统计筛选网格子项候选

- PR #97 合并后 main CI36868758519 已成功。PR #98 九项准确候选检查、89个业务/升级 PASS、九波并发及四种 Compose 成功，已合并 9c4c3cd906e65f2b497904620a0d641b5cae0499，候选/最新 main 计算/实际树一致；旧合成父节点限制及镜像程序身份见 [统计图表验收](ANALYTICS_OVERVIEW_LIFECYCLE.md#2026-10-01-合并验收)。main CI36870649003 已成功。
- PR #99 head d2a15495 已改基 main，准确手动 CI36869291859/36869300132 的八项基础检查成功，隔离 smoke 仍运行；本项叠加 #99，必须先交付前置，不能复用前置 CI。
- 本项仅给筛选 Grid 增加 item，旧11项正常通过但出现两条 PropTypes 警告，新182项全套/依赖边界/格式/零警告 lint/build 通过；一次复用上下文、非 fresh 的独立只读审阅无具体发现。九种宽度、四组日期交互及14组统计图表实际浏览器通过，24px间距变化已测量。详见 [Grid 记录](ANALYTICS_GRID_LAYOUT.md)。尚未完成本项准确候选 CI、PR、合并；标题、体积、其他页面及发布门槛仍开放。


### 统计图表已查询日期标题候选

- Grid [PR #100](https://github.com/dreamvm/one-hub/pull/100) head018b7b17 手动准确候选 CI36871413655/36871421547 运行中。本项叠加该提交，须先交付前置；本项源码/测试与已验收临时副本逐字一致。
- 旧版将未提交日期草稿显示为现有图表期间，有效旧回归3正常通过/2失败；新增独立已查询日期状态，发起请求时更新，标题不再随草稿变化。5标题专项及20相关专项、187完整Vitest/依赖边界/格式/零警告lint/build通过；一次复用上下文、非fresh独立只读审阅无具体发现，实际明暗移动浏览器8组通过。见[标题记录](ANALYTICS_PERIOD_HEADING.md)。本项准确CI/PR/合并仍未完成；未改变日期合法性或范围政策。

- 日期布局 [PR #99](https://github.com/dreamvm/one-hub/pull/99) 九项准确候选检查、89业务/升级PASS、九波并发及四种Compose成功，合并aed35474decd106090dc56c0a02719cb8cd58a82；候选/最新main计算/实际树一致，旧合成父节点限制和镜像程序身份见[日期验收](DATE_RANGE_RESPONSIVE.md#2026-10-01-合并验收)。main CI36871762848待核对。PR #100已改基main，head018b7b17不变。


### 语言图标入口体积候选

- PR #99 合并后main CI36871762848已成功。Grid PR #100 head018b7b17准确候选八项基础检查成功、镜像smoke运行中；标题[PR #101](https://github.com/dreamvm/one-hub/pull/101) head23292f41准确手动CI36872001960/36872009378运行中。本项叠加#101，须依次交付前置再改main，不能引用它们的CI替代本项。
- 语言菜单仅需四个国家图标，将全量默认对象改为四个具名导入及映射；同基线入口减少224,291原始/51,116gzip字节，实际统计页请求脚本同样减少224,291未压缩字节。一次复用上下文、非fresh独立只读审阅无具体发现；四语言、明暗移动旧新各8组正常控制通过，SVG完全一致，无页面/控制台错误警告。最新叠加源码重跑187项全套/依赖边界/零警告lint/build及8组浏览器通过。详见[图标体积记录](LANGUAGE_FLAGS_BUNDLE.md)。本项准确CI/PR/合并尚待完成；不代表LCP或所有大chunk优化完成。


### 原生 arm64 镜像验收准备

- Grid PR #100 九项准确候选检查及89业务/升级PASS、九波并发、四种Compose通过，合并86fea947506186ce9cf800cc3f19152bce3d508f；候选/最新main计算/实际树一致，旧合成父节点限制和镜像程序身份见[Grid验收](ANALYTICS_GRID_LAYOUT.md#2026-10-01-合并验收)。main CI36874138917待核对。标题PR #101已改基main；图标PR #102仍依赖#101，均保持冻结head。
- 本项叠加图标PR #102的f33f9706，扩展smoke为原生amd64/arm64两架构，共享同一套三数据库、故障并发、升级回滚和四种模板验收；核对镜像及最终binary架构，不发布镜像。官方固定镜像均含arm64，新增Linux arm64 Compose固定校验资产。旧架构门槛失败、两个原有控制通过；候选19项工作流检查及38项Python离线smoke、actionlint通过。一次复用上下文独立审阅无功能阻断，发现测试导入分组后修正，无第二轮复审。详见[arm64记录](ARM64_IMAGE_ACCEPTANCE.md)。本项准确CI、真实arm64镜像、PR和合并尚未完成；生产和发布门槛仍开放。

### 2026-10-01 双架构交付与真实后端浏览器验收

- PR #100 合并后 main CI36874138917 已成功。标题 [PR #101](https://github.com/dreamvm/one-hub/pull/101) 九项准确候选检查及89业务/升级PASS、九波并发、四种Compose成功，合并c65c53f39e789447b1e7fce08f359a33875cda14，main CI36874831130成功；树及镜像程序证据见[标题记录](ANALYTICS_PERIOD_HEADING.md#2026-10-01-合并验收)。
- 图标 [PR #102](https://github.com/dreamvm/one-hub/pull/102) 九项准确候选检查及相同专项验收成功，合并7860d0d3db79f7bfdb8b04587df56efe50ff4847，main CI36875419258成功；树及镜像程序证据见[图标记录](LANGUAGE_FLAGS_BUNDLE.md#2026-10-01-合并验收)。ESLint当前零警告，入口同基线减少224,291原始/51,116gzip字节；不代表全部动态chunk或LCP已优化。
- 原生双架构 [PR #103](https://github.com/dreamvm/one-hub/pull/103) head23b5c263的十项准确候选检查成功；arm64和amd64各自89业务/升级PASS、九波并发、四种Compose成功，并读取最终程序身份。合并0229f0aaf4a6f00720b402467ae317f47248735e，main CI36877499093成功。详见[arm64实际验收](ARM64_IMAGE_ACCEPTANCE.md#2026-10-01-合并验收)，不再将本项列为“实际arm64未执行”。
- [真实本地后端浏览器](FRONTEND_REAL_BACKEND_ACCEPTANCE.md)已完成实际登录、令牌增删改与持久化、设置保存刷新、空数据页及移动暗色异常日期恢复。15分钟夹具到时FAIL和最终汇总脚本失败保留；退出登录、正常有数据页面及完整端到端继续开放。新增证据不改应用代码，不将部分操作通过写成整体通过。
- 受限审阅草稿 #68/#71/#78/#79仍未获结论，不重试或绕过；真实历史账目/身份归属、生产版本/配置/历史凭据/备份恢复点以及RC8最终验收继续开放。生产目标信息待用户提供，仅暂停依赖该信息的检查。RC8未建标签，未发布镜像、部署或真实付费调用。
