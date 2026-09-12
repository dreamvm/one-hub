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
4. 上游自带的镜像/Release 工作流需要发布凭据和仓库变量；未配置前不要依赖它们发布。
5. 未确认新修复前，不把生产私有文件或整个生产目录复制进公开仓库。
