# 下一版本计划

更新：2026-09-29。计划基线：`76fc8238e2d187a81239d8e8126289efe9460464`。
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
| rc.8 / C1a | 令牌对象的当前授权状态 | 已持久化的额度/状态变化不被旧 token 缓存覆盖；正常令牌与额度恢复可用 | 本地回归与独立审阅通过；详情见 [TOKEN_AUTHORITY.md](TOKEN_AUTHORITY.md)，完整 C1 仍开放 |
| 后续 C1b / C2 / C3 | 有限令牌预扣与生命周期、原子预留、数据库支付幂等 | 缓存与批量计费不放行耗尽额度；并发不超支；多实例重复回调只入账一次；故障可重试 | 待实施，阻断正式版 |
| 后续 D | Fork 部署模板、健康检查、构建入口、PostgreSQL/依赖/端到端验收 | 固定 Fork 镜像；健康失败正确退出；三数据库及相关故障路径通过 | 待实施，阻断正式版 |
| 后续 E | 前端既存警告与加载体积 | 行为回归通过、深浅主题可用、性能变化有依据 | 待实施，按影响安排 |

rc.6 只覆盖身份与凭据；rc.7 增加媒体与 Midjourney 边界。
C/D 仍未完成，不把 RC7 的批次验收作为整体安全整改完成或正式生产就绪证明。
未确认项继续核查：OIDC 用户名声明约束、兑换码锁的实际 SQL、支付宝重启回调、
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
