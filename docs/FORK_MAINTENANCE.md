# dreamvm/one-hub 维护说明

本 Fork 以 `MartialBE/one-hub` 的 `main` 为基础。首批修复基线为
`387f8bf16ed0d601fdede7ade378adb10aa1a35a`，保留上游代码、历史和 Apache-2.0 许可证。

## 首批迁入的修复

原修复基于上游 `v0.14.27`，本分支只迁入可复用的源码变更：

- 在 OpenAI 兼容工具调用中保留可选的 `extra_content` 原始 JSON。
- 将 Gemini 函数调用所在 part 的 `thoughtSignature` 放入
  `extra_content.google.thought_signature`，同时覆盖非流式和流式转换。
- 读取下一轮 assistant 工具调用中的该字段，还原为 Gemini 的 `thoughtSignature`。
- 拆分工具流时，各工具使用自身的 ID 和元数据；仅在首个分片携带元数据。
- 保留原有 `GeminiFunctionCall.ToOpenAITool` 方法，避免不必要的调用接口变更。
- 修正 Dockerfile 的版本注入目标为 `one-api/common/config.Version`，版本仍从 `VERSION` 读取。

思考签名按不透明数据处理，不解码、不重写，也不制造替代签名。
参考：[Google Gemini thinking 文档](https://ai.google.dev/gemini-api/docs/thinking)。

没有迁入生产配置、密钥、用户数据、渠道 ID、固定模型列表、部署脚本、
机器专用的 Node 内存限制，或硬编码的 `v0.14.27` 构建版本。
Open WebUI 的客户端签名保留补丁属于另一个项目，不包含在本仓库中。

## Gemini 工具结果角色修复

OpenAI 兼容请求的 `tool` / 旧式 `function` 消息转换为 Gemini
`functionResponse` 时，使用 `user` 角色，不再发送 `function`。
连续工具结果仍合并到一个结果 turn，保持顺序、函数名、内容及原有调用签名；
普通用户消息不会因为同为 `user` 角色而与工具结果混合。
名称缺失或空白时尝试通过 `tool_call_id` 匹配；无法解析时返回本地 HTTP 400，
不再解引用空指针或向上游发送空函数名。

新增回归先在修复前复现了错误角色、缺失名称异常，以及模拟上游错误放行的情况。
模拟上游现在检查 `functionCall` 的 `model` 角色与 `functionResponse` 的 `user` 角色，
并将真实转换器输出序列化后送入模拟处理器，检查 JSON/SSE 两条路径。
这只是受控工具调用契约的检查，不是完整 Gemini 协议验证器；
源码测试通过不等于已发布新镜像或完成 Open WebUI 浏览器端到端验收。

## Gemini 工具定义和调用控制

OpenAI 工具参数使用 Gemini 专用声明的 `parametersJsonSchema` 传送，保留嵌套的
`additionalProperties`、`examples`、引用与参数名称，不修改请求原对象、不降级删除约束。
这要求上游实现该原生字段；旧网关若不支持，需要升级上游适配，不能以无限制删字段代替。
函数参数必须是 JSON 对象；损坏或非对象参数返回本地 400。

支持 `auto` / `none` / `required` / 指定函数及旧式 `function_call` 控制。
`strict=true` 只在 `ANY`（required/指定函数）或禁止调用时接受；不把 auto 偷换为强制调用。
Gemini 此适配器不能保证禁止并行，因此显式 `parallel_tool_calls=false`（除 none）返回 400。
自定义函数与搜索、代码执行、URL 工具均保留，由实际上游验证模型是否支持组合。
none 不注入内置工具。工具执行和权限仍由客户端负责。

共享 Chat/Responses 的并行开关改为可空布尔值，保留未设置、false、true 三种状态。
新增检查解析仅由 Gemini/Claude 适配按需使用，不改写其他供应商的 Schema。

## 流式完成与断流检查

Gemini 按候选答案缓冲跨事件的完整函数调用，统一编号，并在正常 STOP 时只发出一次
工具结束信号；文字与工具并存时不会丢弃文字。原生函数 ID 在调用与结果之间保留，
无参数调用使用 `{}`。`partialArgs` / `willContinue` 属于另一种平台增量协议，当前明确报错，
不会将未完成参数误当作空参数执行；不宣称已经兼容 Vertex 的所有增量调用模式。

直接 Gemini/Claude Chat 转换路径启用可选 EOF 校验：Gemini 必须收到候选结束原因，
Claude 必须完整关闭内容块并收到 `message_delta` 和 `message_stop`。连接提前结束时返回
不完整响应错误。未选择该校验的其他供应商流读取行为保持不变。
协议错误与上述断流错误输出 OpenAI 风格 JSON error 事件，不输出不能解析的纯文本；
内部仍可识别原始错误类型。Gemini 接受 SSE 的 `data:` 有/无空格形式，解析失败立即结束读取。

## Claude 工具往返

Chat 兼容路径分别管理 Claude 内容块索引与 OpenAI 工具索引，一条回答始终使用
同一个响应 ID 和 choice 0。工具参数在独立缓冲区累积，完成时校验 JSON 对象；
空参数输出 `{}`，截断、乱序或未知工具增量不会作为成功工具结果继续。
非流式回答的多个文本/工具块合并为一条 choice，保留说明文字。

原始 assistant 内容以 `extra_content.anthropic.content` 保留，并同时放在首个工具的
`extra_content` 中供已有工具元数据客户端使用。流式元数据在最终 delta 才完整，
客户端必须合并该末尾元数据并在下一轮回传；只保存首分片或 reasoning 文本不够。
服务端检查回传调用 ID、名称、参数和说明文字是否与元数据一致，拒绝过期编辑数据。
签名及 redacted data 不解码、不伪造、不持久化到日志。

非流式内容块在未编辑时保留上游原始 JSON，包括空 thinking/text、caller 及未知扩展字段；
修改 Go 对象后不回放过期原文。该保留也适用于原生 Messages 响应，不会改变云平台请求封装。
已知内容类型中的新字段可往返，不意味着兼容路径能执行未来未知类型的服务端工具。

连续工具结果合并为一个 user turn；检查缺失、重复和未知结果 ID，保留错误标记，
支持文本、原生 image/document、OpenAI image_url 和 inline PDF file 结果。
Word/PPT 本身不是直接可视输入，需要客户端先转换为 PDF/图片或提取文本。
strict、none 和显式禁止并行现在传入 Claude，旧式 function_call 只支持无签名单工具。
原生 Messages 的工具示例、延迟加载、allowed_callers、adaptive thinking/display、
output_config 和 context_management 字段有独立序列化测试，包含 Vertex/Bedrock 封装。
仅厂商 `anthropic-beta` 头可从调用方转发，已配置渠道头优先，不转发任意认证头。

不宣称支持所有服务端工具和未来未知块；兼容路径遇到未知内容明确报错并建议原生接口。
这些是离线契约测试，不是云平台真实凭据验收，也不能代替 Open WebUI 客户端验收。

## 回归测试

使用 Go 1.25.x。在仓库根目录运行：

```sh
go test -race -count=1 ./types ./providers/gemini ./providers/claude ./common/requester
go test -race -count=1 ./.github/smoke/...
go build ./providers/... ./relay/...
go vet ./types ./providers/gemini ./providers/claude ./common/requester
go test -count=1 ./.github/tests
go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12
```

测试使用虚构的签名和本地 JSON/SSE 数据，不调用真实模型、不读取服务配置、
不发送邮件、不写生产数据库，也不执行工具命令。

覆盖范围：

- 工具元数据 JSON 往返，以及非 Google 的未知扩展字段。
- 非流式、流式签名往返；客户端重组分片后再提交工具结果。
- 单工具、多工具、仅第一个工具带签名、不同工具各带独立签名。
- 无签名工具、旧式 `function_call`、空或不适用的可选扩展内容。
- Gemini SSE 解析、工具分片、结束事件和用量保留。
- 工具结果的 `user` 角色、连续多结果合并、普通用户 turn 隔离及无法解析名称时的 HTTP 400。

新增测试在上游基线先复现签名丢失和多工具 ID 重复，再迁入修复。
GitHub Actions 的 `Gemini compatibility` 工作流只测试和编译，不发布镜像、不部署。

不要把 `go test ./...` 作为无需准备的离线测试命令：上游现有部分测试会调用
邮件、存储、通知或外部图片服务。需要单独准备受控环境后再执行。

## 当前边界

- 本维护版处理函数调用 part 上的签名与工具结果角色转换，不宣称覆盖所有新模型或全部 Gemini 协议。
- 不包含 Interactions API、文本/图片 part 的签名，或跨多个上游事件拼接增量函数参数的完整支持。
- Gemini Schema/strict 的明确支持边界见上文，不保证所有中转供应商实现相同字段。
- 不修改计费、重试、渠道选择或权限规则；也不代表此前流式中断的所有根因已修复。
- 首批修复最初只验证源码回归和相关后端包；后续构建和隔离验收记录见下方，不代表线上端到端验收。

## 后续维护和发布

1. 每项修复使用独立分支和 PR，先写能复现问题的测试。
2. 同步上游前比较差异，保留本 Fork 的签名回归用例。
3. 合并源码不等于部署；上线前还需独立构建、备份、隔离联调及回滚验证。
4. 镜像发布只走下方的手动 GHCR 流程，不再依赖 Docker Hub 变量或私人发布令牌。
5. 未确认新修复前，不把生产私有文件或整个生产目录复制进公开仓库。

## 手动镜像构建与发布

合并 PR、推送 main、对本维护版提交创建版本标签均不会自动发布镜像。
不要给原始上游旧提交新打发布标签：GitHub 可能读取旧提交中的自动发布定义，
而不是当前 main 的配置。待发布标签应指向包含本维护版工作流和测试的已合并提交。
旧的 Linux/macOS/Windows 二进制 Release 工作流已移出执行目录，归档在
`.github/legacy-workflows/` 供参考，GitHub 不会执行该目录中的文件。
如需重新提供二进制发行，必须另行适配与验证，不能直接把旧文件移回执行目录。

镜像目标是 `ghcr.io/dreamvm/one-hub`。工作流使用当前仓库的 `GITHUB_TOKEN`，
不需要配置 Docker Hub 账号或额外 PAT。发布权限仅授予镜像构建任务；测试任务保持只读。

需要构建或发布时：

1. 对已经合并、经过审阅的提交创建版本标签，格式为 `vX.Y.Z` 或 `vX.Y.Z-suffix`。
   例如 `v0.14.27-dreamvm.1` 只是格式示例，本次并未创建这个标签。
2. 在 Actions → **Manual GHCR image** → **Run workflow** 中选择默认分支 `main`。
3. 填写已有的 `release_tag`。工作流检查标签格式、存在性，并确认其提交属于触发时 main 的历史。
4. 初次保持 `publish=false`：执行测试并构建镜像，不登录 GHCR、不上传镜像。
   默认只构建 `linux/amd64`；需要多架构时选择 `linux/amd64,linux/arm64`。
5. 构建和隔离联调通过、确认需要发布后，再手动以 `publish=true` 运行。

标签解析为固定提交 SHA 后，测试和镜像构建都使用同一个 SHA，避免测试与构建不同版本。
镜像任务通过 `needs` 依赖测试成功；不绕过失败或取消的测试。发布不自动更新 `latest`，
只生成版本标签和 `sha-完整提交` 标签，并在运行摘要中记录来源提交和镜像 digest。
版本标签不要移动或复用；部署时建议固定已验证的镜像 digest。

`publish=false` 的构建主要用于验证可构建性，没有将镜像作为可下载产物导出；
隔离联调仍需另行准备本地构建或经批准的镜像发布。首次 GHCR 发布后还应检查包的访问权限。

### 首次完整构建记录

2026-09-12 对 `61ef57550fa98dd31eb9b342c2f0f7d55a221889` 创建测试标签
`v0.14.27-dreamvm.1-buildtest.1`，以 `publish=false` 完成 `linux/amd64` 全镜像构建。
[构建记录](https://github.com/dreamvm/one-hub/actions/runs/34683724436)包含回归测试、
前端打包、Go 主程序编译和最终镜像层；未上传镜像、未创建 Release、未部署。
静态链接 glibc、前端依赖和 chunk 大小提示不是运行验收通过的证明。

### 隔离镜像运行测试

`Isolated image smoke` 在 PR 或手动触发时先通过源码回归，再构建同一提交的 amd64 镜像，
仅 `load=true` 加载到 GitHub 临时 runner，固定 `push=false`，无登录仓库或部署步骤。
`.github/smoke/run.py` 创建随机命名的内部 Docker 网络、临时 SQLite 卷和两个受限容器。
不向宿主机发布任何端口；测试请求通过内部网络中的受限 HTTP 探针发送。
容器不挂载 Docker socket，不使用主机网络或生产配置。
模拟上游以同一镜像运行独立静态测试程序，返回虚构 JSON/SSE，绝不调用真实模型或执行工具。

检查范围：

- 启动、SQLite 空库迁移、版本注入、内嵌前端资源、证书包。
- 使用容器名称解析上游 DNS；HTTP/校验证书的 HTTPS；中文 JSON/SSE 对话及用量。
- Gemini 双工具 ID、中文参数、各自签名、工具结果的两轮往返；损坏签名负向检查。
- 普通用户对话/工具与管理接口权限；重启后密码、用户、令牌和渠道仍可使用。
- 无论成功或失败，移除本次创建的容器、数据库卷和网络；不删除其他资源。

离线测试设置 `DISABLE_TOKEN_ENCODERS=true`，避免启动时联网下载编码文件；这不是生产配置建议。
测试 HTTPS 使用临时自签测试证书，未关闭证书校验；不等于验证真实供应商证书链。
CI 顺序运行 SQLite、MySQL/Redis 以及下方的升级/回滚演练，任一路径失败都会使任务失败。
不覆盖 PostgreSQL、真实模型、Open WebUI/Open Terminal 和 Word 文件生成，
也不覆盖默认在线 tokenizer 初始化；这些需要后续独立验收。模拟测试通过不能替代真实联调。

### Claude 隔离往返验收

模拟上游新增 `/v1/messages`，以严格的虚构签名和内容块检查真实网关转换后的第二轮请求。
SQLite/MySQL 路径均新增 Claude 渠道，验证 JSON 和 SSE 下的思考、文字、双工具、中文参数分片、
合并结果 turn 和普通用户两轮调用。客户端测试重组器必须保留最终 delta 的元数据。
独立负向测试确认修改签名会失败。所有签名和工具名均为测试数据，不访问文档或真实厂商。
旧版升级脚本仍使用原来的三条渠道，避免用旧版本不支持的新协议作为迁移前置条件。

### MySQL 与 Redis 隔离验收

同一 `.github/smoke/run.py` 现支持 `--backend mysql-redis`，默认仍为 `sqlite`。
需要显式传入已经在本机预加载、核实过的 MySQL 与 Redis **镜像 ID**，
即 `--mysql-image sha256:<64位哈希>` 和 `--redis-image sha256:<64位哈希>`。
不接受可变标签，不连接现有数据库，不自动拉取数据库镜像。
本测试要求两种官方镜像的服务用户 UID/GID 均为 `999:999`，使用前应核实。

该路径创建独立 MySQL 数据卷、随机测试密码，以及同一个内部网络中的受限服务。
所有容器无宿主机端口、丢弃 Linux capabilities，且有 CPU/内存限制。
MySQL 限制为 1 GiB/1 CPU，网关 512 MiB/1 CPU；Redis 与模拟上游各 128 MiB/0.25 CPU。
Redis 仅作临时缓存，禁用 AOF/快照。测试验证：

- MySQL 空库迁移和用户、渠道的实际持久化，确保未退回 SQLite。
- Redis 产生真实缓存键和命中，不把仅配置连接字符串视为验收成功。
- 原有 JSON/SSE、双工具签名、普通用户权限检查。
- 重启 MySQL、清空式重启 Redis 和重启网关后，密码、用户、令牌、渠道仍可使用，缓存可以重建。
- 在 Redis 缓存预热后删除普通用户令牌，确认旧令牌立即被拒绝。
- 成功或失败均只清理本次随机命名的容器、数据卷和网络。

这不等于验证旧版生产数据迁移、数据库高可用、缓存断线降级或真实模型兼容性。
GitHub 工作流自动运行此路径。MySQL 8.2.0、Redis 8.8.0 和旧应用 v0.14.27 的公开镜像
均在工作流中固定 registry digest，按 `linux/amd64` 拉取，再解析为本地不可变镜像 ID。
这一步只准备测试镜像；实际容器继续使用 `--pull=never`，测试网络禁止外连。
依赖拉取或任一检查失败均不会被忽略，不需要生产凭据；每条路径设有时间上限。
更新依赖 digest 时必须重新核对镜像版本、服务用户 UID/GID，并完整运行验收。

#### 2026-09-12 独立服务器验收

对已合并应用提交 `3885faae7b4edec2392cd3b9ae3d874e82987f9b` 构建本地 amd64 测试镜像，
以本地分支 `test/mysql-redis-smoke` 的增强脚本执行；SQLite 和 MySQL 8.2.0 + Redis 8.8.0
两条路径均退出 0。上述持久化、缓存命中、令牌撤销、权限和模拟工具调用检查全部通过。
构建仅对临时构建副本注入 smoke 版本和编译资源设置，未改应用业务代码。
临时容器、数据卷和网络已清理，保留测试镜像；未发布镜像、未修改生产部署或调用真实模型。
运行日志和机器专用配置保存在仓库外，不将生产配置或服务器凭据提交到公开仓库。

### 旧版升级与回滚演练

`.github/smoke/upgrade.py` 使用四个已预加载的不可变镜像 ID（旧应用、新应用、MySQL、Redis），
在随机命名的内部网络中创建全新测试库和虚构账户。参数为 `--old-image`、`--old-version`、
`--candidate-image`、`--candidate-version`、`--mysql-image`、`--redis-image`、`--mock-binary`。
不接收生产 DSN、已有数据库容器或生产数据目录；网关均无宿主机端口。

演练先用旧版 API 创建用户、渠道、有效令牌及禁用令牌，验证基本对话和权限后停止测试写入。
随后用逻辑备份和 SHA-256 校验固定升级前状态，保持测试应用密钥不变并在版本切换时清空测试 Redis：

1. 升级到维护版，比较原有用户/密码/余额/令牌/渠道记录，测试旧凭据、工具签名往返和新增写入。
2. 只切回旧镜像，验证旧程序在已升级数据库上的基本功能与新增记录保留。
3. 恢复升级前备份，要求完整 schema/data dump 与原备份逐字一致，再以旧镜像验证登录、令牌、渠道和缓存。

恢复只允许作用于本轮登记的新测试数据库；备份校验失败时不得执行 SQL 恢复。
全部临时容器、数据卷、网络、虚构数据备份在结束时清理。

2026-09-12：旧版 `v0.14.27` 到应用提交 `3885faae7b4edec2392cd3b9ae3d874e82987f9b` 的
MySQL 8.2.0 + Redis 8.8.0 演练通过。新增 `channels.allow_extra_body` 字段，未移除旧字段；
两种回滚方式在本次虚构样本上均通过，恢复备份后升级期间新增的测试用户按预期消失。
首轮测试密码超出旧版 20 字符限制，修正测试数据后完成全程；没有修改应用业务代码。

这不是任意版本可直接降级的保证，也不替代真实生产数据、支付/任务状态或高负载验证。
备份回滚会撤销备份之后的写入，正式发布必须安排停止写入的维护窗口和明确回滚点。
该演练已接入 `Isolated image smoke`，在两个后端的启动与持久化检查通过后自动执行。
新应用始终来自本轮已测试的同一提交 SHA；旧镜像固定摘要，版本由启动后的 API 实测。
GitHub 运行摘要记录各阶段结果，不上传数据库备份；没有镜像发布、生产部署或真实模型调用步骤。
