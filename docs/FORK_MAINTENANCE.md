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
- 本轮只验证源码回归和相关后端包，不构建完整前端/Docker 镜像，也不做线上端到端验收。

## 后续维护和发布

1. 每项修复使用独立分支和 PR，先写能复现问题的测试。
2. 同步上游前比较差异，保留本 Fork 的签名回归用例。
3. 合并源码不等于部署；上线前还需独立构建、备份、隔离联调及回滚验证。
4. 镜像发布只走下方的手动 GHCR 流程，不再依赖 Docker Hub 变量或私人发布令牌。
5. 未确认新修复前，不把生产私有文件或整个生产目录复制进公开仓库。

## 手动镜像构建与发布

合并 PR、推送 main、创建版本标签均不会自动发布镜像。
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

本次只准备流程并执行语法、策略和代码回归检查，未手动触发镜像构建，
未创建版本标签、镜像或 Release，未更改线上服务。首次完整镜像构建及部署验证仍待后续执行。
