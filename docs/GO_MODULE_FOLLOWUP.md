# 两条 Go 模块匹配的静态补充

2026-10-01，受检应用候选6bcc849d。仅处理既有go-audit-current.json中的两条模块级记录，
不是新全库扫描；逐项保留原编号，没有升级依赖或执行动态漏洞验证。

| 公告 | 当前静态结论 | 证据与边界 |
| --- | --- | --- |
| [GO-2026-5942](https://pkg.go.dev/vuln/GO-2026-5942) | 维护构建范围内not_actionable，高静态置信度 | x/net0.55.0仍在范围，但Linux amd64/arm64主程序依赖图没有外部模块的dns/dnsmessage；实际包含的是Go1.25.14的vendor副本。官方stdlib范围从Go1.26开始，固定工具链没有受影响的SVCB/HTTPS解析入口 |
| [GO-2026-6443](https://pkg.go.dev/vuln/GO-2026-6443) | 维护构建范围内not_actionable，高静态置信度 | gRPC1.83.1仍有库缺陷；需要xDS服务端路由读取空authority。两个主程序依赖图不含grpc/xds或internal/xds/server。业务直接使用IAM客户端，主监听为Gin HTTP；测试中的bufconn服务端不进入交付程序 |

静态核对入口包括go.mod、Dockerfile:14、main.go:127–149和providers/vertexai/base.go:125–138。
独立记录按triage-finding/v0保留来源、边界、反证及证明缺口；两项均不进入修复优先级队列。
没有适用SECURITY.md，不据此推定配置可信；结论依据受影响路径未进入维护构建。

使用固定Go1.25.14、CGO=1、离线readonly的go list元数据，amd64为1044包、arm64为1043包。
初次在无web/build的工作区读取因embed缺文件失败；成功记录来自已有构建的隔离副本，
560份Go源码/模块输入与候选逐字一致。没有运行测试、程序、攻击演示或真实DNS请求。
两份包图和逐项结构化结果已保存为私有证据。

上述结论不等于库已修补、全部DNS解析不存在或Go审计清零，也不证明生产运行版本。
新增相关解析包、xDS服务端、工具链或构建方式变化时须重查；其余Go/JS记录继续分别核对。

29条其余模块记录的后续静态结论见[剩余Go输入](GO_RESIDUAL_DEPENDENCIES.md)，仅覆盖维护构建。
