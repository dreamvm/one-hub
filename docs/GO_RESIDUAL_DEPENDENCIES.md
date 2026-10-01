# 剩余 Go 模块匹配的调用范围

2026-10-01，源码核对4ad956fc；本次文档以#115合并08bc01e2为基线，二者内容树一致。处理既有go-audit-current.json中未在台账直接索引的29条
模块级输入，保留每条输入及原始公告；不是29个新增缺陷，也没有删除原快照中的包/符号记录。
此前八个公告的结论见[两条模块补充](GO_MODULE_FOLLOWUP.md)和[NEXT_RELEASE.md](NEXT_RELEASE.md)。

按codex-security:triage-finding只作静态分流，没有测试、程序运行、漏洞探针、依赖升级或生产访问。
本批29条均为维护版Linux amd64/arm64主程序范围内`not_actionable`，高静态置信度，
不进入修复排序队列。**依赖版本仍匹配公告，这不是库已修补或审计清零。**

## 输入与证据

固定Go1.25.14、CGO=1、readonly离线元数据：amd64 1044包、arm64 1043包。
元数据读取使用已有前端构建的隔离副本，560份Go源码和模块文件与候选逐字一致；
两架构实际GoFiles/CgoFiles、导入者和受影响API消费者均核对，完整元数据保存为私有证据。
29份官方vuln.go.dev JSON在本日重新读取，与保存的OSV内容均一致。
没有适用SECURITY.md；范围依据维护镜像、main入口、CLI及源码事实，不据此推定生产配置可信。

## 每条输入的静态结论

下表每行保留一条原模块级输入。所有行的结论仅为上述维护构建范围内`not_actionable`。

| 公告 | 受影响包/行为 | 当前反证 |
| --- | --- | --- |
| [GO-2026-5005](https://pkg.go.dev/vuln/GO-2026-5005) | x/crypto/ssh/agent | 全部受影响包均未进入两个架构的主程序依赖图 |
| [GO-2026-5006](https://pkg.go.dev/vuln/GO-2026-5006) | x/crypto/ssh/agent | 全部受影响包均未进入两个架构的主程序依赖图 |
| [GO-2026-5013](https://pkg.go.dev/vuln/GO-2026-5013) | x/crypto/ssh | 全部受影响包均未进入两个架构的主程序依赖图 |
| [GO-2026-5014](https://pkg.go.dev/vuln/GO-2026-5014) | x/crypto/ssh | 全部受影响包均未进入两个架构的主程序依赖图 |
| [GO-2026-5015](https://pkg.go.dev/vuln/GO-2026-5015) | x/crypto/ssh | 全部受影响包均未进入两个架构的主程序依赖图 |
| [GO-2026-5016](https://pkg.go.dev/vuln/GO-2026-5016) | x/crypto/ssh | 全部受影响包均未进入两个架构的主程序依赖图 |
| [GO-2026-5017](https://pkg.go.dev/vuln/GO-2026-5017) | x/crypto/ssh | 全部受影响包均未进入两个架构的主程序依赖图 |
| [GO-2026-5018](https://pkg.go.dev/vuln/GO-2026-5018) | x/crypto/ssh | 全部受影响包均未进入两个架构的主程序依赖图 |
| [GO-2026-5019](https://pkg.go.dev/vuln/GO-2026-5019) | x/crypto/ssh | 全部受影响包均未进入两个架构的主程序依赖图 |
| [GO-2026-5020](https://pkg.go.dev/vuln/GO-2026-5020) | x/crypto/ssh | 全部受影响包均未进入两个架构的主程序依赖图 |
| [GO-2026-5021](https://pkg.go.dev/vuln/GO-2026-5021) | x/crypto/ssh/knownhosts | 全部受影响包均未进入两个架构的主程序依赖图 |
| [GO-2026-5023](https://pkg.go.dev/vuln/GO-2026-5023) | x/crypto/ssh | 全部受影响包均未进入两个架构的主程序依赖图 |
| [GO-2026-5033](https://pkg.go.dev/vuln/GO-2026-5033) | x/crypto/ssh/agent | 全部受影响包均未进入两个架构的主程序依赖图 |
| [GO-2026-5932](https://pkg.go.dev/vuln/GO-2026-5932) | x/crypto/openpgp, x/crypto/openpgp/packet, x/crypto/openpgp/armor, x/crypto/openpgp/clearsign, x/crypto/openpgp/errors, x/crypto/openpgp/elgamal, x/crypto/openpgp/s2k | 全部受影响包均未进入两个架构的主程序依赖图 |
| [GO-2026-6303](https://pkg.go.dev/vuln/GO-2026-6303) | x/crypto/ssh | 全部受影响包均未进入两个架构的主程序依赖图 |
| [GO-2026-6354](https://pkg.go.dev/vuln/GO-2026-6354) | x/crypto/ssh | 全部受影响包均未进入两个架构的主程序依赖图 |
| [GO-2026-6355](https://pkg.go.dev/vuln/GO-2026-6355) | x/crypto/ssh | 全部受影响包均未进入两个架构的主程序依赖图 |
| [GO-2025-3900](https://pkg.go.dev/vuln/GO-2025-3900) | go-viper/mapstructure/v2 | 库编入，Viper读配置和typed Get不进入该结构解码错误路径；没有Unmarshal调用 |
| [GO-2026-4503](https://pkg.go.dev/vuln/GO-2026-4503) | filippo.io/edwards25519 | 库编入，但只有MySQL认证调用ScalarBaseMult；没有MultiScalarMult消费者 |
| [GO-2026-4815](https://pkg.go.dev/vuln/GO-2026-4815) | x/image/tiff | 全部受影响包均未进入两个架构的主程序依赖图 |
| [GO-2026-4962](https://pkg.go.dev/vuln/GO-2026-4962) | x/image/font/sfnt | 全部受影响包均未进入两个架构的主程序依赖图 |
| [GO-2026-5031](https://pkg.go.dev/vuln/GO-2026-5031) | x/image/bmp | 全部受影响包均未进入两个架构的主程序依赖图 |
| [GO-2026-5032](https://pkg.go.dev/vuln/GO-2026-5032) | x/image/tiff | 全部受影响包均未进入两个架构的主程序依赖图 |
| [GO-2026-5062](https://pkg.go.dev/vuln/GO-2026-5062) | x/image/tiff | 全部受影响包均未进入两个架构的主程序依赖图 |
| [GO-2026-5066](https://pkg.go.dev/vuln/GO-2026-5066) | x/image/tiff | 全部受影响包均未进入两个架构的主程序依赖图 |
| [GO-2022-0635](https://pkg.go.dev/vuln/GO-2022-0635) | aws-sdk-go/service/s3/s3crypto | 全部受影响包均未进入两个架构的主程序依赖图 |
| [GO-2022-0646](https://pkg.go.dev/vuln/GO-2022-0646) | aws-sdk-go/service/s3/s3crypto | 全部受影响包均未进入两个架构的主程序依赖图 |
| [GO-2026-4771](https://pkg.go.dev/vuln/GO-2026-4771) | pgx/pgproto3 | 库编入，但应用是数据库客户端；Frontend.Receive不选择这两个服务器侧解码器 |
| [GO-2026-4772](https://pkg.go.dev/vuln/GO-2026-4772) | pgx/pgproto3 | 库编入，但应用是数据库客户端；Frontend.Receive不选择这两个服务器侧解码器 |

## 库已编入但受影响行为未调用的四项

- **GO-2025-3900**：go-viper/mapstructure/v2 2.3.0仍存在错误中包含输入值的行为。
  唯一编入的外部导入者为Viper1.20.1；应用cli/flag.go:52–55通过ReadInConfig读取配置，
  后续使用typed Get。ReadInConfig走配置codec，Get使用cast；Viper的Unmarshal、
  UnmarshalKey、UnmarshalExact及decode hook才进入该v2解码路径，交付调用没有这些入口。
  Epay导入的是另一个mitchellh/mapstructure模块，本结论不替它处理其他历史公告。
- **GO-2026-4503**：edwards25519 1.1.0的Point.MultiScalarMult仍有初始接收者状态缺陷。
  唯一外部导入者MySQL驱动1.9.3的auth.go:237–265使用ScalarBaseMult及标量运算；
  两架构的编入源码没有MultiScalarMult消费者。标准库internal包是另一实现，不能混同。
- **GO-2026-4771/4772**：pgx5.7.5的Bind.Decode/FunctionCall.Decode仍有缺陷；Backend.Receive
  会选择它们。model/main.go:69–80的PostgreSQL入口是客户端，pgconn配置构造NewFrontend。
  Frontend.Receive只选择服务端响应，包括独立的BindComplete/FunctionCallResponse；未知类型
  在Decode前返回错误，没有应用NewBackend消费者。结论不靠数据库可信或PreferSimpleProtocol。

## 剩余边界

25条包未编入与4条API未调用是不同反证；不能把有包依赖图等同于没有调用，也不能把模块存在
等同于漏洞成立。每条结构化结果保留源、必要路径、反证、范围及缺口，未合并重复输入。
没有验证真实生产二进制/配置、其他程序或未来新导入；构建、插件、API或调用变化必须重新分流。

本批与此前八项一起为现有37个Go模块级公告提供处置索引，原始61条匹配仍保留。
这不是最终候选的新全量审计，也不关闭剩余前端公告、受限审阅、历史归属、生产事实和发布门槛。
文档候选独立审阅、准确CI及合并验收另行记录，不由此静态结果替代。

一次非作者、复用上下文且非fresh-context的独立只读文档审阅无具体发现，核对29条输入、
双架构元数据、上述四项源码语义及37/61计数；未运行测试、程序、PoC或新triage。
