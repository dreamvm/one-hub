# 用户媒体下载策略

共享入口为 `common/image.RequestFile`，包括完整下载与尺寸探测。
Gemini、Claude、Ollama、OpenRouter 等调用方共用此入口；PDF 和内联 data URL 保持原有用途。
Midjourney 图片读取使用同一目标策略的 `RequestPublicFile` 原始字节入口，且继承客户端取消。

## 网关直接下载

- 仅接受 HTTP(S)，目标必须为公网单播地址。回环、私网、链路本地、共享地址、保留和文档地址、
  IPv6 区域标识及常见地址转换范围均拒绝；IPv4 映射 IPv6 按其 IPv4 地址检查。
- 域名解析的全部结果必须符合策略，混合公网与非公网结果整体拒绝。
  国际化域名先按 IDNA 规范化，再检查、解析与校验证书。
  连接使用本次已检查的数字 IP，保留原 Host、HTTPS SNI、证书验证和 URL 路径/查询。
- 每次重定向重新检查并固定目的地址；同名下一次解析转为非公网也拒绝。
- 每次下载最多 15 秒，响应读取最多 20 MiB；尺寸探测读取所需头部后关闭响应。
  不在下载错误中附带包含签名或凭据的原始 URL。

公网判定不能发现部署环境把公网地址特殊路由到内部服务的情况。部署仍须用出口网络策略限制可达服务。

## 管理员配置的代理

`ChatImageRequestProxy` 仍支持 HTTP、HTTPS、SOCKS5 和 SOCKS5H 代理及用户名密码。
管理员配置的代理端点属于可信基础设施，可以位于本机或内网；用户媒体目标仍须满足公网策略。

- SOCKS 目标使用已检查的数字 IP，包含 SOCKS5H 模式；域名在网关解析。
- HTTP(S) 代理对 HTTP 与 HTTPS 媒体均使用 CONNECT 到数字 IP，在隧道内保留原 Host/SNI。
  代理必须允许媒体对应端口的 CONNECT，HTTP 通常为 80，HTTPS 通常为 443。
  仅允许 CONNECT 443 的代理不能下载 HTTP 媒体；失败时返回错误，不回退到重新解析域名的转发模式或直连。
- HTTPS 代理证书按代理主机名校验，媒体 HTTPS 证书按媒体主机名校验。
- 每次请求使用独立连接，避免跨目标或代理配置复用连接造成校验与实际连接不一致。

Midjourney 的原始图片入口以前不使用 Worker，继续返回完整二进制图片，不把 Worker JSON 或截断头部作为图片。
此入口优先使用 `ChatImageRequestProxy`；为空时逐跳按当前 URL 选择 HTTP_PROXY/HTTPS_PROXY/NO_PROXY 环境代理。
代理目标仍固定到已检查的 IP，不会因拒绝而回退直连。其他共享媒体入口的代理和 Worker 协议保持原规则。

## Cloudflare Worker 委托

配置 `CFWorkerImageUrl` 后，保留既有 POST 协议：`action`、`api_key`、`url`。
发送前在网关检查用户目标，且不跟随 Worker 端点的重定向，避免转发 API key 到其他端点。

Worker 是管理员显式信任的远端下载服务；其代码不在本仓库。当前协议无法传递可验证的固定 IP，
也不能证明远端重定向或 DNS 解析结果。Worker 必须自行执行相同的公网、重定向及连接校验。
网关前置检查不能替代 Worker 侧防护，本次验收不宣称修复未知 Worker 实现中的 SSRF。
未配置 Worker 的默认路径由网关完整执行下载策略；不因 Worker 错误改为直接下载。

## 离线验收与回滚

```sh
go test -race -count=1 ./common/image -run '^(TestMedia.*|TestParseBase64File)$'
go vet ./common/image
```

测试仅连接本机模拟服务；公网 IP 是协议校验用的虚构目的，实际拨号被测试夹具替换。
覆盖正常图片、尺寸、PDF、内联数据、签名查询、代理认证与 TLS、重定向、DNS 变化、超时及响应关闭。
旧 `image_test.go` 还包含外网集成测试，因此不要取消上述过滤条件。

没有 schema 或配置迁移；回退代码会恢复旧的无限制下载行为，不应作为目标拒绝错误的常规处理方式。
先核对媒体地址是否公网、代理 CONNECT 端口策略与 Worker 自身策略。
