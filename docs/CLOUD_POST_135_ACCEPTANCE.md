# PR135后的云端接续验收

2026-10-14，基线main `3d256b3cd135e46ee7dfea471f3ac624c57b60a8`。
PR135合并树与准确候选一致，main CI38511264078四项成功；没有标签、镜像发布或部署。
RC8整体尚未验收，不能把历史本机证据视为已迁移。

## 官方Go审计的网络缺口

云端配置revision31为running/enforced，restricted网络的effective_allowed_hosts未包含
`vuln.go.dev`。通过已有代理访问 `https://vuln.go.dev/index/db.json` 返回CONNECT 403；
没有改用镜像、直连或其他运行地点绕过该目的地拒绝。
目前环境连接器只提供状态读取，没有配置写入接口。维护者已授权必要最小配置调整，
但授权不等于工具具备修改能力。

需要在本聊天所选one-hub云环境的配置编辑页面，进入网络访问设置的域名白名单
（Allowed hosts），只加入 `vuln.go.dev`，保存并应用到当前环境。
具体客户端菜单名称尚未取得截图或界面证据；不假定修改机器内策略文件会修改平台策略。
完成后须重核当前enforced配置及官方端点，再执行有界审计。

govulncheck1.7.0已在临时工具目录安装，编译工具链Go1.25.14。
尚未完成本轮官方Go依赖/调用链及实际双架构程序审计；不沿用#130的37公告旧快照作为新结果。
源码审计要保留调用路径、输入和信任边界；binary/symbol匹配也不等于业务可利用。

## 默认关闭的原生程序证据导出候选

原生CI此前只留下程序身份日志，没有可下载的实际程序；云端Docker镜像列表为空，
本机builder仅支持amd64，不能声称已取得当前arm64程序。
本候选只补充可审阅的证据运输：在原有全部业务、升级/回退和四种Compose检查成功后，
才可保留从最终本地镜像提取的同一 `/one-api` 与身份manifest。

普通PR默认不导出。手动工作流的 `export_binary` 默认false；明确添加
`binary-audit-export` PR标签也可触发opt-in验收，移除该标签后后续运行不导出。
每个产物按准确源码SHA、架构、run_attempt命名，仅保留7天。
导出前复核checkout、既有编译身份和程序SHA256，复制上限256MiB，身份记录上限4KiB；
只上传两个固定文件，不打包配置、数据库、mock、日志或运行目录，不执行提取程序。
manifest记录源码SHA、本地image ID、Go版本、Linux架构、CGO及程序SHA256；
本地image ID不是registry digest，导出程序不是发布容器镜像。

下载后仍须核对所属CI任务/源码、manifest及程序实际build info和哈希。
两个原生程序用于后续官方binary审计，不能把合成字节单测当作实际程序验收。
新候选仍需准确候选CI和审阅，其合并须另行批准。现有push:false及只读contents权限保留。

## 实际定制RC5及其他剩余验收

已定位仓库[实际RC5演练](ACTUAL_RC5_UPGRADE_REHEARSAL.md)和
[#130产物记录](POST_130_ARTIFACT_ACCEPTANCE.md)：旧版本为
`v0.14.27-dreamvm.1-rc.5-gemini49-fix1`，旧config ID为
`a50cc418252e0309e7b15b427279801eb1ef303e50e9cb26dc824469e148beac`，
本机OCI manifest/index身份为
`60ed6a6c94ae117f270c10543b756ef3978aa9de1f1b279e412ced46fa6f9115`。
历史镜像保留在原Apple Silicon Docker Desktop；文档没有云端可用的归档路径。
私有 `artifacts/post130-evidence.zip` 历史SHA256为
`0a0eb16f3dca2a44decc91734d79daa3f1b5a20fc13f22d5e6e59a5954d03f3e`，本次未找到该文件。
旧镜像归档、私有日志和原依赖triage均未确认迁移，不能由历史哈希反推拥有实际材料。

需取得不含配置/挂载/数据库的实际旧镜像归档及可用位置，再核对身份后对最终候选重验；
现有上游v0.14.27 CI夹具继续独立记录，不能代替定制RC5。
最终候选关键页面/体积、异常恢复、双架构实际产物与真实RC5升级/回退/备份恢复仍待完成。
历史Babel/Rollup等不重新从头整改，YAML v3/matcher残留、受限审阅和单项例外继续保留。
生产新恢复点、凭据完整性、未知issuer及历史账务不以合成数据猜测。
新PR合并、标签、镜像发布、生产部署及真实付费调用各自保留专项审批。
