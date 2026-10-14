# PR135后的云端接续验收

2026-10-14，当前main `b783f95a0a96fd1301dd0713231763193c6c83f0`（PR136）。
PR135和PR136分别获得专项合并批准，实际合并树均与各自准确候选一致。
PR135 main CI38511264078四项成功；PR136 [main CI38516829968](https://github.com/dreamvm/one-hub/actions/runs/38516829968)
四项成功，逐任务实际checkout均核对为该合并提交。没有标签、镜像发布或部署。
RC8整体尚未验收，不能把历史本机证据视为已迁移。

## 官方Go审计的网络缺口

最新核对云端配置revision34为running/enforced，restricted网络的effective_allowed_hosts未包含
`vuln.go.dev`。通过已有代理访问 `https://vuln.go.dev/index/db.json` 返回CONNECT 403；
没有改用镜像、直连或其他运行地点绕过该目的地拒绝。
目前环境连接器只提供状态读取，没有配置写入接口。维护者已授权必要最小配置调整，
但授权不等于工具具备修改能力。

需要在本聊天所选one-hub云环境的配置编辑页面，进入网络访问设置的域名白名单
（Allowed hosts），为官方Go审计加入 `vuln.go.dev`，保存并应用到当前环境。
具体客户端菜单名称尚未取得截图或界面证据；不假定修改机器内策略文件会修改平台策略。
完成后须重核当前enforced配置及官方端点，再执行有界审计。

govulncheck1.7.0已在临时工具目录安装，编译工具链Go1.25.14。
尚未完成本轮官方Go依赖/调用链及实际双架构程序审计；不沿用#130的37公告旧快照作为新结果。
源码审计要保留调用路径、输入和信任边界；binary/symbol匹配也不等于业务可利用。

2026-10-14官方npm元数据重核：js-yaml的 `v3-legacy` 为3.15.2、latest为5.4.3，
picomatch latest为4.0.7；与此前接续记录没有新版本变化。
仅为发布元数据读取，不是兼容或漏洞验证；旧YAML v3/matcher候选的兼容残留继续保留，
没有安装、应用或重新运行受限专项。最新官方JS审计175条与PR135候选零新增零移除，
不是审计清零或新的全量triage。

## 默认关闭的原生程序证据导出已交付

原生CI此前只留下程序身份日志，没有可下载的实际程序；云端Docker镜像列表为空，
本机builder仅支持amd64，不能声称已取得当前arm64程序。
PR136只补充证据运输：在原有全部业务、升级/回退和四种Compose检查成功后，
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
PR136准确候选 `4bf131868ae8d4431ddff2f508a453e7e615fa31` 经独立非作者审阅
（复用上下文，非fresh）和十项有效CI后获专项批准，合并为上述当前main。
实际父提交为原main `3d256b3cd135e46ee7dfea471f3ac624c57b60a8` 与准确候选，
树均为 `40089f21630525287117ef1c04a10ca1af03994e`。
[Compatibility38513324064](https://github.com/dreamvm/one-hub/actions/runs/38513324064)
四项实际checkout同树合成提交53351eb1；
[Isolated38513334998](https://github.com/dreamvm/one-hub/actions/runs/38513334998)
六项实际checkout准确候选，两个原生架构各92项PASS、四种Compose及导出成功。
首轮38513324069被标签触发的替代运行取消，不计通过。
现有push:false及只读contents权限保留；本次合并批准不适用于后续台账或其他PR。

两个程序ZIP产物为amd64 ID11872290329（74163453字节）、arm64 ID11871940915
（70599773字节），2026-10-21过期。大小超过download_file工具32MiB上限；
GitHub工具另提供官方整文件临时地址，通过现行代理GET实测HTTP403，未取得云端文件字节。
当前临时地址域名为 `sdmntprwestus3.oaiusercontent.com`，未在现行白名单中。
只有支持的环境配置流程可以解决目的地限制，不拆分、改名或直连。
**GitHub导出成功不是云端下载校验、Go审计或最终候选验收成功。**
合并后main工作流只有四项检查，不把候选原生镜像写成合并提交重新构建。

## 实际定制RC5及其他剩余验收

已定位仓库[实际RC5演练](ACTUAL_RC5_UPGRADE_REHEARSAL.md)和
[#130产物记录](POST_130_ARTIFACT_ACCEPTANCE.md)：旧版本为
`v0.14.27-dreamvm.1-rc.5-gemini49-fix1`，旧config ID为
`a50cc418252e0309e7b15b427279801eb1ef303e50e9cb26dc824469e148beac`，
本机OCI manifest/index身份为
`60ed6a6c94ae117f270c10543b756ef3978aa9de1f1b279e412ced46fa6f9115`。
历史镜像保留在原Apple Silicon Docker Desktop，维护者已从不可变manifest ID重新导出，
在本机逐项校验config/layer SHA256及linux/amd64。旧标签导出失败，不依赖该标签；
新tar不冒充历史原始归档。镜像、两份历史证据zip及校验清单已完成Dropbox私有上传，
四份upload状态均completed；本机登录网页确认仅本人可见，上传后共享链接列表为空。
云端已分别核对四份文件的元数据、大小和身份，没有公开归档、私有路径或临时地址。
私有 `artifacts/post130-evidence.zip` 历史SHA256为
`0a0eb16f3dca2a44decc91734d79daa3f1b5a20fc13f22d5e6e59a5954d03f3e`，
文件已在本机找到、哈希核实并私有上传，前次“未找到”不再是当前状态。

云端用支持的单次临时下载地址、现有代理/TLS对286字节校验清单做一次完整GET，
返回HTTP403，尚未取得文件字节。临时主机属于 `dl.dropboxusercontent.com` 的不同子域；
若平台支持该子域规则，只为此下载用途配置；否则核对并允许实际完整主机名。
规则语法及新地址主机不能假定不变，不开放整个Dropbox或任意网络。
单次地址不能HEAD/Range预检，配置生效后重新取得地址，整文件下载后再校验。
两个下载403响应仅有状态、text/plain及server:envoy，未取得更细拒绝原因；
白名单同时缺少对应域名，配置后仍须重试，不能提前宣称问题已解决。

**已私有上传、云端元数据核对、实际字节校验、镜像加载和新候选演练分别记录。**
目前后面三项均未完成；原2026-10-08依赖triage是否在证据包内也未实际读取核实，
不能由归档名称或历史哈希反推其内容已迁移。

下载恢复后先校验归档SHA256、镜像config/manifest/platform，再对最终候选重验；
现有上游v0.14.27 CI夹具继续独立记录，不能代替定制RC5。
最终候选关键页面/体积、异常恢复、双架构实际产物与真实RC5升级/回退/备份恢复仍待完成。
历史Babel/Rollup等不重新从头整改，YAML v3/matcher残留、受限审阅和单项例外继续保留。
生产新恢复点、凭据完整性、未知issuer及历史账务不以合成数据猜测。
新PR合并、标签、镜像发布、生产部署及真实付费调用各自保留专项审批。
