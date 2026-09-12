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

## 回归测试

使用 Go 1.25.x。在仓库根目录运行：

```sh
go test -race -count=1 ./types ./providers/gemini
go build ./providers/... ./relay/...
go vet ./types ./providers/gemini
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

新增测试在上游基线先复现签名丢失和多工具 ID 重复，再迁入修复。
GitHub Actions 的 `Gemini compatibility` 工作流只测试和编译，不发布镜像、不部署。

不要把 `go test ./...` 作为无需准备的离线测试命令：上游现有部分测试会调用
邮件、存储、通知或外部图片服务。需要单独准备受控环境后再执行。

## 当前边界

- 本补丁只处理函数调用 part 上的签名，不宣称覆盖所有新模型或全部 Gemini 协议。
- 不包含 Interactions API、文本/图片 part 的签名，或跨多个上游事件拼接增量函数参数的完整支持。
- 不处理工具 Schema 的 `strict`/`additionalProperties` 等其他兼容性问题。
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
端口仅绑定 runner 的 `127.0.0.1`；容器不挂载 Docker socket，不使用主机网络或生产配置。
模拟上游以同一镜像运行独立静态测试程序，返回虚构 JSON/SSE，绝不调用真实模型或执行工具。

检查范围：

- 启动、SQLite 空库迁移、版本注入、内嵌前端资源、证书包。
- 使用容器名称解析上游 DNS；HTTP/校验证书的 HTTPS；中文 JSON/SSE 对话及用量。
- Gemini 双工具 ID、中文参数、各自签名、工具结果的两轮往返；损坏签名负向检查。
- 普通用户对话/工具与管理接口权限；重启后密码、用户、令牌和渠道仍可使用。
- 无论成功或失败，移除本次创建的容器、数据库卷和网络；不删除其他资源。

离线测试设置 `DISABLE_TOKEN_ENCODERS=true`，避免启动时联网下载编码文件；这不是生产配置建议。
测试 HTTPS 使用临时自签测试证书，未关闭证书校验；不等于验证真实供应商证书链。
目前不覆盖 MySQL/PostgreSQL、Redis、真实模型、Open WebUI/Open Terminal 和 Word 文件生成，
也不覆盖默认在线 tokenizer 初始化；这些需要后续独立验收。模拟测试通过不能替代真实联调。
