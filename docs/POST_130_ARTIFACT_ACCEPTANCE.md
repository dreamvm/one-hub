# #130 后续产物与实际 RC5 演练

2026-10-04。已合并 main `557103c95f2dd0acffb26000c2d4cbc62dd2c524`，
与已验收候选 `dde3a597430bce01b0ab786f0b3b9c6012e82b38` 源码树相同。
本记录补齐该候选的本机双架构程序审计与实际 RC5 升级回退，不是最终 RC8 或部署验收。

## 产物身份与审计

使用候选的 git archive 和原 Dockerfile，仅在构建副本注入与 CI 相同的
`v0.0.0-smoke-dde3a597430bce01b0ab786f0b3b9c6012e82b38`。
Node22.20 / Yarn1.22.22 冻结安装、Go1.25.14；逐架构构建各20分钟上限，本机加载、不推送。
从镜像提取 `/one-api`，确认 Linux、对应架构、CGO=1，两个程序 SHA256 均与
[准确候选 CI](https://github.com/dreamvm/one-hub/actions/runs/37182952080)相同。

| 架构 | 本机 build index SHA256 | 平台 manifest SHA256 | 程序 SHA256 |
| --- | --- | --- | --- |
| amd64 | 37f477f6e5125fdd9549647f988435b35d8676fbf03dae90579a0b02eda54286 | d32559b007c2d6f52abe83dcbe42c4010f9aa8de664ae93312926b20d94c954c | ca82c10c65340cccd3fd331a5fb66254014b8a5d6faac94cd85029d770b996c8 |
| arm64 | 35f820c87f381f17c6c7937f9c1103212c84d8c641812b12e62bf6c60a085ad3 | f878c5472f9f29a5d1b8263a8c84786b359adf9387e03f41240cb9da0d9eb3d7 | 104d6c6144d4ac2e58c87b92fae9cb10169df4770285aa15850594dd80ae3b63 |

本机 index/manifest 与 CI image ID 含义不同；程序相同不等于整镜像相同，也不是 registry digest。
govulncheck1.7.0 二进制 SHA256 为
`899544ebe123095757b7a49801c4919eaaa38c75f5aa4fe1f8e6f047f15b33ed`。
以 binary/symbol 模式、180秒上限从官方 `https://vuln.go.dev` 获取公告，未上传程序或源码。
服务返回数据库时间为 `2026-10-01T20:24:15Z`；两次均正常退出，两个架构各匹配37个公告ID，
相互一致，相对 #128 对应架构快照零新增、零移除。
符号存在不是源码调用链，也不证明业务可利用；不将正常退出或零新增写成依赖清零。

首次 amd64 构建因 Docker 24GB磁盘空间不足而失败，保留原始 ENOSPC 日志。
先前按ID及24小时筛选的清理未释放足够空间；逐项核对缓存归属后，仅回收本项目隔离builder
的可重建缓存，保留全部镜像、容器、卷及项目文件，随后同源码重建通过。
没有把失败构建计为通过；既有 peer、chunk、Dockerfile 大小写提示保留。

## 实际 RC5 升级与回退

使用已保留的实际旧版 `v0.14.27-dreamvm.1-rc.5-gemini49-fix1` 镜像，
本机不可变索引 `60ed6a6c94ae117f270c10543b756ef3978aa9de1f1b279e412ced46fa6f9115`，
不是 CI 的上游 v0.14.27 夹具。MySQL/Redis沿用[此前记录](ACTUAL_RC5_UPGRADE_REHEARSAL.md)
的固定镜像，并重新核对amd64平台身份。
复用的本地模拟上游确认Go1.25.14、Linux/amd64、静态编译；其源码及go.mod/go.sum与当前候选无差异。

沿用当前候选原 `.github/smoke/upgrade.py`，未改变断言。只用合成账号/数据、内部Docker网络，
保留逐容器资源限制、命令超时和20分钟外层上限。Apple Silicon上的amd64仿真不代表原生性能。
7项检查通过、进程退出0：旧版登录/令牌/JSON/SSE/权限，停止写入后的合成备份与校验，
升级后的原记录和新写入，使用迁移后数据库的镜像回退，完整schema/data备份恢复，
恢复后旧版凭据/权限/Redis，以及临时备份和资源清理。
另行读取Docker状态确认本轮前缀的容器、卷、网络均为空；真实模型调用0、生产数据复制false。

备份恢复会移除恢复点之后的写入；合成兼容不证明任意历史数据可降级。
旧RC5回退可能重新开放Realtime，仍须遵守[首版范围与回退约束](REALTIME_RELEASE_SCOPE.md)。
本轮未连接或修改服务器，也没有新建服务器恢复点。

## 四条新增 braces 输入

公告 [GHSA-vfj7-8cjw-p6xm](https://github.com/advisories/GHSA-vfj7-8cjw-p6xm)
覆盖braces<=3.0.3的递归栈耗尽，当前公告无修复版本。
先规范化全部四条输入，再内联静态核对；无新深层嵌套探针或运行时验证。
以下四条分别保留，结论均为当前维护入口的 `not_actionable`、高静态置信度：

| 输入路径 | 实际反证 |
| --- | --- |
| sass → chokidar → braces | Vite使用Sass JavaScript编译API；sass.node.js不加载chokidar，项目脚本不启动Sass watch CLI |
| eslint-config-react-app → @typescript-eslint/parser → typescript-estree → globby → fast-glob → micromatch → braces | 实际web/.eslintrc优先于package.json配置，选择Babel parser，未启用react-app的TS分支 |
| eslint-config-react-app → @typescript-eslint/eslint-plugin → type-utils → typescript-estree → globby → fast-glob → micromatch → braces | 同一配置反证，独立保留该输入；没有project glob来源 |
| eslint-config-react-app → @typescript-eslint/eslint-plugin → type-utils → utils → typescript-estree → globby → fast-glob → micromatch → braces | 同一配置反证，独立保留该输入；没有project glob来源 |

表中的parser、eslint-plugin、type-utils、utils、typescript-estree均为@typescript-eslint作用域包。
额外核对resolveProjectList无project时在调用globby前返回；react-app自带配置也未设置project。
没有适用SECURITY.md，边界依据实际脚本、配置、最终镜像和维护者说明的“仅API中转、无上传构建”。
不凭devDependencies标签排除风险；长度限制不是深度保护，也未将disableGlobbing当作通用免疫。
源码证据、完整路径及每项限制保存于独立的 `braces-20261004-triage.json`。

库仍受影响，前端审计仍为173条；上述结论不是库修补，不覆盖任意自定义工具配置、未来上传构建
或其他依赖图路径。启用自定义TS project glob或Sass watch前重评，继续跟踪上游修复。

## 证据和发布边界

本机补充证据包 `artifacts/post130-evidence.zip`，SHA256
`0a0eb16f3dca2a44decc91734d79daa3f1b5a20fc13f22d5e6e59a5954d03f3e`，
保留失败/成功构建、双架构身份和审计、对照、7项演练日志及执行入口，不含生产配置或数据库。
#130合并和main四项CI另见[交付记录](AXIOS_120_MAINTENANCE.md#2026-10-04-合并验收)。

本次是已执行证据的文档整理，没有新的独立事实审阅结论；自身文档PR/CI和合并状态须另列，
不能用#130的CI代替。YAML v3及matcher残留、Realtime底层缺口仍按各自范围保留。
最终发布版本/产物、新恢复点、历史凭据轮换及专项标签/发布/部署授权仍未完成。
