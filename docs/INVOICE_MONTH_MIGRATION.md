# 月账单日期字段跨数据库迁移

## 问题与最小修复

启用 `USER_INVOICE_MONTH` 后，启动迁移使用 `StatisticsMonth` 建表。旧模型强制
`Date` 为 `datetime`；PostgreSQL 不接受此类型，实际隔离建表返回
`SQLSTATE 42704`。原账单详情测试手写 `DATE` 表，不能证明生产模型可迁移。

移除日期字段的固定 SQL 类型，让 GORM 的数据库方言映射 `time.Time`；保留时间值、
复合主键和账单计算规则。没有改为只保存日期，避免截断旧 datetime 的时间部分。
同文件另一处字符串拼接空格由 gofmt 调整。详情夹具改用生产模型迁移，新增日统计到
月账单的实际生成回归，覆盖月初/月末、跨渠道汇总、用户和月份隔离以及重复生成不翻倍。

## 2026-10-01 本地证据

本地验证基线 `d4f48aacb1b76d0c044aea89c608e51aa177aefe`（文档 PR #110）；候选在独立临时副本验证。
交付分支从该 PR 的实际合并 `da936380c20e89e82006d69e4d69e10279d5c4f3` 建立，两者内容树一致；
三份 Go 文件复制后逐字核对一致。
Go 1.25.14，离线依赖，合成账户和数据；PostgreSQL 为本机 18.4，随机回环端口、临时集群，
限制连接数、内存和进程时限，运行后关闭。CI PostgreSQL 镜像版本仍为 18.0，须另行验证。

- 旧模型：SQLite 的 4 个生成/迁移子例通过；PostgreSQL 生产模型建表失败，后续生成不执行。
- 新模型：带 race 的账单详情及生成测试，SQLite 7 个子例、PostgreSQL 6 个子例通过。
  SQLite 另验证旧表非零时分秒及其他字段完整保留。MySQL 旧表迁移与生成尚待准确候选 CI。
- 规定的五个离线回归包 race、相关 vet、providers/relay 编译通过。
- 策略测试补传固定 Task 3.53.1、Compose 5.5.1 后通过；此前未传工具的结果不作为工具相关验收。
- 一次未参与实现的独立只读审阅无具体发现；复用既有上下文，非 fresh-context。
  审阅核对锁定驱动映射、差异、测试和文档，并独立运行 gofmt 检查；未重复运行数据库测试。
  SQLite 使用 datetime，PostgreSQL 使用 timestamptz，现代 MySQL 默认 datetime(3)；
  MySQL 类型精度变化须由旧表数据保留控制和候选 CI 验证。
- 首次 PostgreSQL 夹具因临时 Unix socket 路径过长未启动，改为仅回环 TCP 后才得到有效结果；
  不将该次启动失败计为回归结果。运行器成功退出不替代每个 Go 测试日志的结果。

## 验收范围与回滚

本项不是对真实历史账单的修复或重算；没有连接生产数据库。未证明完整应用启动、
消费日志到日统计再到月账单的端到端链路、全部时区或历史自定义数据库结构。
本地没有 MySQL 实例；准确候选 CI、独立审阅、合并及合并后检查分别记录，未完成不能写为通过。

应用回退会重新引入 PostgreSQL 的错误类型声明；已有表也不能据此保证旧版迁移成功。
真实升级/回滚须先核对数据库结构和备份恢复点，不自动删除账单或恢复生产数据。
不改变 RC8、标签、镜像发布、部署和真实付费调用的审批边界。
## 2026-10-01 合并验收

- [PR #111](https://github.com/dreamvm/one-hub/pull/111) head `1e08ea38db09e0a7e3fc347247b85c6113caa05f` 的
  [Compatibility](https://github.com/dreamvm/one-hub/actions/runs/36903168568) 和
  [Isolated](https://github.com/dreamvm/one-hub/actions/runs/36903169887) 十项检查通过。
  三数据库专项实际检出该 head：SQLite/MySQL/PostgreSQL 的迁移、生成和重复生成均通过；
  SQLite/MySQL 的旧 datetime 非零时间数据保留通过。
- Compatibility 前端实际检出 `79713bba3fd56e0c627be78d553a006bcbb5de5b`，父节点 `da936380`/`1e08ea38`；
  双架构检出候选。候选、合成、最新 main 计算及实际合并树均为 `e1898e43255737a4580d59b222d17e883e1e1b5e`。
  实际合并 `0ec84ed60d93e8cbf8ecd74413eae74f1c1cc0a9`，父节点与合成一致；
  [合并后 main CI](https://github.com/dreamvm/one-hub/actions/runs/36905699246) 已通过。
- 两架构各89项业务/升级PASS、九轮并发、四种Compose通过；最终程序均为Go1.25.14、one-api、Linux、对应架构、CGO=1。

| 架构 | CI 本地 image ID (sha256) | 最终程序 SHA-256 |
|---|---|---|
| amd64 | a85038296ef754326746a4d53483b168ea33de47eb22f673065f7218f0a372e6 | 0bc50f3988eefa4457bad7dddeaf96cdabd9a3b15473b2d15c674a8590feab8f |
| arm64 | df15369a6f6684273a4a8859b1dbd287d311fa30829d7d828327423830beb872 | a04f77457fc0097343106f1a3f73b97e5180c2f1c7f3963f5a78656c3f6c7a19 |

[真实浏览器补充](FRONTEND_REAL_BACKEND_ACCEPTANCE.md#2026-10-01-消费日志到生成账单的真实页面补充)
已验证合成存储日志生成日统计与月账单、普通用户列表详情和刷新；不替代全部时区、真实资金来源或生产验收。
