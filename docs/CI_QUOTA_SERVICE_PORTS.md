# 额度数据库 CI 服务端口维护

2026-10-15，基线为 PR146 的实际 main 合并提交
`d83f6f0ba0f80a1b3acf11789f2a954e35b51b16`。本候选只调整 CI 和测试夹具，
不修改应用代码、数据库迁移、依赖版本或发布入口。

## 问题与修改

[main CI38690131460](https://github.com/dreamvm/one-hub/actions/runs/38690131460)
首次运行在 checkout 前启动 PostgreSQL 服务时失败，日志明确为固定主机端口 `35432`
已被使用。仅重跑失败任务后，第二次运行四项成功，逐项 checkout 已核对为实际 main。
成功重跑保留了旧配置，不能据此宣称固定端口问题已消失。

本候选让 MySQL/PostgreSQL 服务分别使用 `127.0.0.1::3306`、`127.0.0.1::5432`，
由 Docker 分配主机端口；测试步骤通过 `job.services.<service>.ports['<port>']`
取得实际端口。表达式的端口键使用字符串，已通过固定版 actionlint 检查。

`model/quota-transaction_test.go` 仅在显式选择外部测试数据库时读取对应端口变量，
保留原有回环主机、虚构账号和数据库。变量不存在时沿用本地默认端口；配置无效时
明确失败。没有增加读取应用 DSN 的入口。配置回归同时检查服务映射和测试步骤的连接配置。

镜像摘要及 PostgreSQL 18 的默认数据目录不变。此前一次普通 PostgreSQL 单独启动
观察到服务 ready，仅证明该次启动成功；没有完成连接或整组额度测试的证明。
不据此猜测早期未取得完整日志的 PostgreSQL 失败原因，也不增加未经证实的兼容参数。

## 实际检查

以下结果对应本候选的三个代码文件；后续文档整理没有改动它们。

| 检查 | 结果与边界 |
| --- | --- |
| `go test -mod=readonly -count=1 -v ./.github/tests -run '^TestQuotaDatabase'` | 通过，4 个 PASS 节点（含父测试和子项） |
| `ONEHUB_QUOTA_TEST_DB=sqlite go test -mod=readonly -race -count=1 -timeout=90s -v ./model -run '^TestQuotaTransaction'` | 通过，323 个 PASS 节点，无失败或跳过；不覆盖 MySQL/PostgreSQL 连接 |
| `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12` | 修正字符串端口键后通过；早期数字键失败记录保留 |
| `go vet -mod=readonly ./model ./.github/tests` | 通过，不执行测试或启动服务 |
| 格式及 `git diff --check` | 通过 |
| 完整 `./.github/tests` | 未通过：HTTP/2 四个子项监听回环地址时收到 `socket: operation not permitted`，连同父测试共 5 个失败节点；已停止，不重试 |
| MySQL/PostgreSQL 普通常规启动与连接验收 | 未完成：两服务初始化均实际报告 `No space left on device`，没有连接或额度测试通过证明；所属容器、网络、匿名卷已清理，没有重跑 |

本候选的普通检查没有执行刻意占用端口的复现，也未恢复暂停的依赖安全验证。现有安全匹配与受限记录继续
保留为未关闭；本候选不把它们标为已修复、已验证或风险已接受。

## 交付边界

候选保持未合并。普通维护分支推送不命中当前工作流的 push 分支规则；暂不创建会自动
重跑上述已停止套件的 PR，也不手动触发该套件。没有削弱、跳过或改写现有 CI 门槛。
完整候选 CI、外部数据库连接验收和审阅仍待完成，既有 main CI 不算本候选 CI。

后续合并仍需专项批准。回退本候选只恢复工作流端口与测试夹具配置，不涉及应用或数据回退。
生产核对、备份与恢复由本地在原服务器处理；本记录不包含私有生产摘要或生产文件。
标签、镜像发布、生产部署、凭据轮换和真实付费均未执行，整体发布门槛仍开放。
