# 下一版本计划

更新：2026-09-30。计划基线：`76fc8238e2d187a81239d8e8126289efe9460464`。
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
| 后续 C1 / C2 / C3 | 未报告用量核对、数据库支付幂等与绑定 | 缓存与批量计费不放行耗尽额度；并发不超支；多实例重复回调只入账一次；故障可重试 | 待实施，阻断正式版 |
| D1 | Compose健康探针退出状态 | 正常状态成功，网络/HTTP/业务失败返回非零 | PR #44 已合并，9项CI及7项本地回归通过 |
| D2 | 锁定依赖、当前前端、失败传播及本地镜像构建 | 实际Task正常/故障对照，依赖输入哈希不变 | PR #45 已合并，全9项CI通过 |
| D3 | Fork镜像、显式秘密、私有数据库及可选依赖 | 默认保留MySQL+Redis，SQLite组合不引入依赖 | 本地配置与独立审阅完成，实际容器CI待验收 |
| D4a | 真实Redis停止/错误类型/旧余额与恢复 | 拒绝请求零上游/零账务，恢复后正常单次结算 | 验收脚本已准备，真实容器结果待CI |
| D4b | PostgreSQL候选镜像业务与Redis故障 | 登录/中英文JSON与SSE/工具/持久化/撤销及账务故障 | 脚本与离线夹具准备，准确候选容器CI待执行 |
| 后续 D | 依赖/端到端/负载验收 | 固定 Fork 镜像；健康失败正确退出；三数据库及相关故障路径通过 | 待实施，阻断正式版 |
| 后续 E | 前端既存警告与加载体积 | 行为回归通过、深浅主题可用、性能变化有依据 | 待实施，按影响安排 |

rc.6 只覆盖身份与凭据；rc.7 增加媒体与 Midjourney 边界。
C/D 仍未完成，不把 RC7 的批次验收作为整体安全整改完成或正式生产就绪证明。
未确认项继续核查：OIDC 用户名声明约束、支付宝重启回调、
支付金额/币种/渠道绑定与延迟支付状态。证据不足时不登记为“已修复”。

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
