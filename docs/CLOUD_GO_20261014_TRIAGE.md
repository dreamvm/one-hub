# 2026-10-14 官方 Go 输入的行为核对

本轮已恢复正式代理下载，并独立取得 PR136 实际双架构程序。基线源码 d684d49c 的51模块/25包/19调用、程序各50公告保留原始JSON；程序匹配是符号证据，不是业务调用证明。数据库更新时间2026-10-08T22:31:09Z。

Go1.26.9和x/net0.60维护候选的新源码匹配19模块/10包/5调用；所有19输入逐项保留。以下只对维护版Linux amd64/arm64主程序的已核对源码行为作静态分流，未运行漏洞载荷、生产探针或真实模型调用。固定CGO=1的两架构依赖元数据另存私有证据。最终候选身份和实际程序扫描须在独立验收记录更新，不从本表推断已发布。

| 公告 | 当前主程序反证 / 边界 |
| --- | --- |
| GO-2022-0635 | 受影响 s3/s3crypto 未进入两个架构主程序依赖图；普通S3客户端不是该加密包 |
| GO-2022-0646 | 同一加密包未编入，保留独立公告 |
| GO-2025-3900 | Viper配置入口使用ReadInConfig与typed Get，无v2 mapstructure Unmarshal/UnmarshalKey/UnmarshalExact消费者；配置codec不替代v2 decode路径 |
| GO-2026-4503 | MySQL驱动认证调用ScalarBaseMult，未调用受影响MultiScalarMult；编入模块不代表调用该API |
| GO-2026-4771 | pgx是数据库客户端，Frontend.Receive解码服务端响应，不选择Backend侧Bind.Decode |
| GO-2026-4772 | 同一客户端路径不选择Backend侧FunctionCall.Decode，独立保留 |
| GO-2026-4815 | x/image/tiff未进入两个架构依赖图 |
| GO-2026-4961 | WebP配置入口存在，但公告限定32位尺寸溢出；amd64/arm64均64位，不扩大到其他架构 |
| GO-2026-4962 | x/image/font/sfnt未编入 |
| GO-2026-5004 | 项目确实使用PreferSimpleProtocol=true；不能靠默认协议排除。核对Raw/Exec及GORM查询来源，无dollar-quoted SQL字面量；参数值与固定SQL结构分离。未来增加此字面量或可输入SQL时必须重评 |
| GO-2026-5031 | x/image/bmp未编入 |
| GO-2026-5032 | x/image/tiff未编入 |
| GO-2026-5061 | 两个应用入口仅image.DecodeConfig。WebP configOnly在VP8帧头/VP8X尺寸处返回，不进入受影响完整图像解码与alpha尺寸组合 |
| GO-2026-5062 | x/image/tiff未编入 |
| GO-2026-5066 | x/image/tiff未编入 |
| GO-2026-5208 | 邮件Markdown调用html.NewRenderer与CommonFlags确实启用Smartypants，但Text先EscapeHTML、链接标题先EscapeHTML，原始不闭合小于号不能到smartLeftAngle。应用未直接调用SPRenderer.Process，RendererOptions.Title为空且未启用CompletePage，不进入原始页面Title的Process入口；新renderer使用/自定义AST输入需重评 |
| GO-2026-5932 | 未维护的x/crypto/openpgp未编入；升级crypto不宣称该包变为安全 |
| GO-2026-6222 | WebP configOnly在VP8L时只DecodeConfig，在VP8X含alpha时先返回尺寸，不能到完整VP8L Huffman组解码；64KB/16KB读取限制不是此结论的依据 |
| GO-2026-6443 | gRPC模块用于Google客户端，应用没有NewServer/Serve；公告的缺authority/Host服务器入口未提供，不靠上游可信排除 |

上表19项为此范围的静态not_actionable，不代表库已修补、零匹配或任意部署安全。五个调用匹配有具体控制流/架构/SQL/转义反证，不能把govulncheck路径当成可利用确认。增加32位、全图解码、dollar SQL、Markdown原始renderer输入、gRPC服务端或新的模块入口均需重新核对。

新工具链已经移除对应stdlib版本匹配，x/net及其必要crypto/text维护移除对应模块版本匹配；应用Range防御另在PR138。所有初次失败与后续通过分别保存。生产新恢复点、凭据轮换、未知issuer、历史账务与真实付费仍遵循原台账，不能从合成演练或静态表补结论。
