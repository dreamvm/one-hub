# PR135后的云端接续验收

2026-10-15最新状态：[PR147](https://github.com/dreamvm/one-hub/pull/147) 准确批准 head `95a001d7d9289b8c8de5afd7bd7a67df6562c9dd` 已实际合并为 main `96a83b68d53eb9c43017dabad91a019870953e23`；parents、tree 与批准候选/受测合成提交的关系已核对。[合并后 main CI38708363722](https://github.com/dreamvm/one-hub/actions/runs/38708363722) attempt 1 四项成功，逐任务原始日志 checkout 均为实际 merge。端口维护已合并，原失败和精确空间清理证据保留。实际 main 完整构建在显式关闭自动 VCS stamping、单独记录源码身份的限定条件下通过；默认构建的链接工作树元数据失败没有改记为通过。详见[本轮普通发布准备](POST147_RELEASE_READINESS.md)。依赖安全验证继续暂停、未关闭且未接受风险；生产备份及恢复副本只留原服务器，由本地处理。整体发布门槛开放，未创建标签、发布镜像或部署。下方保留各阶段当时状态。

历史：2026-10-15 PR146阶段状态：PR146准确候选 `f53b5c429a16412e4b221954af52a0c2dab7a4b4` 获专项批准后，实际合并为main `d83f6f0ba0f80a1b3acf11789f2a954e35b51b16`；parents为原main `2771c876d8e09f7123cb83c4aca9174b6784c73c` 和批准head，tree `77582e53747c4b50bb37807396346fcff221d146` 与候选及受测合成 `7cad97b9613bc1abd6824ebf6a43d513200d2f22` 相同。[实际main CI38690131460](https://github.com/dreamvm/one-hub/actions/runs/38690131460)第二次运行四项成功，各任务实际checkout已核对；首次固定服务端口冲突保留。后续[普通CI端口维护候选](CI_QUOTA_SERVICE_PORTS.md)尚未合并；新增授权后完成精确临时缓存清理及正式localhost权限预检，双数据库正常连接、三数据库race和完整普通策略套件通过，非作者静态审阅完成。候选CI按PR准确head、实际checkout及受测树另记，既有main CI不代替它。依赖安全验证已暂停，PR146既有前端107路径/39 GHSA、Go19模块/10包/5调用匹配仅为历史快照，不是本轮新审计或清零。生产证据继续私有保存，生产操作由本地负责；整体发布门槛仍开放，未发布或部署。下方保留各阶段当时状态。

历史：2026-10-15 PR145阶段状态：PR145获准确候选专项批准后实际合并为main `2771c876d8e09f7123cb83c4aca9174b6784c73c`；parents为原main `4c2f31844c5a992879b9ffd83d21cb3cd0d04907` 和批准head `f27d71327a559a6b9bb5b73ea62dae327a419e81`，tree `3a95958741308dbc73a838e4aa314bba26a5f167` 与候选及受测合成97d8d362相同。[实际main CI38684549031](https://github.com/dreamvm/one-hub/actions/runs/38684549031)四项成功，逐任务checkout均核对为真实merge。仅调用PR145合并接口，PR142自动标记merged、143/144仍为已覆盖的历史叠加草稿，没有重复合并。新的Browserslist维护与历史/浏览器补验见[本轮接续](POST145_ACCEPTANCE_20261015.md)，新PR等待独立合并批准；未发布、部署、轮换、导出或真实付费。生产具体证据仅在私有验收记录维护。下方保留各阶段当时状态。

## 2026-10-14 PR137真实合并与发布前后续整改

历史：2026-10-14最新状态：PR137准确候选a4d9aeba已获专项批准并合并为main `4c2f31844c5a992879b9ffd83d21cb3cd0d04907`；实际树与受测候选相同，[合并后main CI38598613933](https://github.com/dreamvm/one-hub/actions/runs/38598613933)四项成功且逐任务checkout核对。PR141覆盖PR138/139/140的关系不变，没有重复合并。后续三个独立依赖维护PR142/143/144保持待合并；组合锁树实际审计113路径/53 Yarn ID/41 GHSA，原175输入逐项保留、零新增。三项本地回归与fresh独立审阅完成，组合候选自身CI另核。四项历史范围已补手工独立审阅和限定动态对照，不把原中断/单项例外改写成原专项扫描通过。flatted、Lodash、Immutable等不安全候选没有应用。详见[本轮整改](RELEASE_REMAINING_20261014.md)与[生产只读复核方案](PRODUCTION_READONLY_RECHECK.md)。后续PR合并仍需专项批准；标签、镜像发布、部署、凭据轮换、生产写入及真实付费未获批准；整体发布门槛仍开放。

批准head为 `a4d9aeba90015b642b5eb4e3d03a130d94268728`，实际merge `4c2f31844c5a992879b9ffd83d21cb3cd0d04907` 的parents为 `4cecf90c86ca0ab5ce0a5d43e57f79c04153a5e7` 与批准head；tree `7faac64fba5d95b0dc54ca6bff80c3d15d7f5784` 与候选及受测合成d0659bbf一致。合并前的38592635505/38592635520十项CI和合并后的38598613933四项main CI分别核对。旧程序/实际RC5演练仍绑定76c5657a；新的组合产物与隔离补验另记，不改写旧证据的来源SHA。下方是各阶段当时状态。

## 历史：2026-10-14 PR141实际合并与main验收

维护者已专项批准PR141准确候选 `76c5657a9ef1eec9cf677b921eca17a90067567a`。合并前重核head、base/main、6提交/21文件范围未变；十项有效CI均成功，被替代cancelled运行38544767567排除。独立只读复核无阻断，沿用此前各修复及最终候选独立审阅；GitHub正式review列表为空，未将只读审阅称为GitHub审批。

2026-10-14T15:39:26Z实际合并为当时main `4cecf90c86ca0ab5ce0a5d43e57f79c04153a5e7`；父提交是原main `b783f95a0a96fd1301dd0713231763193c6c83f0` 和批准候选 `76c5657a9ef1eec9cf677b921eca17a90067567a`。实际合并/候选/受测合成 `192e3c94963f897aac2527faa0ef45966ff4a707` 的树均为 `ad7866b8f9916279c21bf180ebcafa2b95df672f`，Git逐字节diff为空。[main CI38591239167](https://github.com/dreamvm/one-hub/actions/runs/38591239167)四项均成功，逐任务日志实际checkout重核为4cecf90c。main工作流不重建原生镜像；现有ZIP、manifest和实际RC5演练仍绑定原76c5657a源码，不更换其来源SHA或冒称发布digest。

| 原独立PR | 集成覆盖与当前GitHub状态 |
| --- | --- |
| PR138 / 2e987a35644567bfa846e211ea387087113a970c | Range修复cherry-pick为0ed40a50ff215dda151692ac826526d0b6bb63bb；两个受影响文件内容相同；原PR仍open/draft，保留历史 |
| PR139 / c6813c647275a431f20b9b36c3856fe457fa083b | head为集成候选祖先；GitHub在PR141合并后自动标记closed/merged，非独立再次合并 |
| PR140 / 0843087f96d6befadce912b7bd70e8ababbcce52 | head为集成候选祖先并含PR139；同样自动标记closed/merged，非独立再次合并 |

该阶段只调用PR141合并接口。PR137当时同步最新main以保留历史文档提交，与当前main比较仅NEXT_RELEASE、RC8_ACCEPTANCE_STATUS和本文件；保持未合并，其专项合并批准尚未取得。旧文档候选a823bcd0及CI38548485708/38548485745保留历史，新文档候选审阅和十项CI在PR137另记。标签、镜像发布、生产部署、凭据轮换、真实付费和生产写入无本次授权。

## 历史：PR136合并后、网络恢复前

当时main `b783f95a0a96fd1301dd0713231763193c6c83f0`（PR136）。
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


## 2026-10-14 新环境独立恢复与准确候选验收

本轮是新的one-hub实例，未继承旧任务/workspace/work。源配置版本cecfgver_6acf472f842c8195986faccc68280df2、observed_revision5/spec_revision5、observations_current=true、restricted/enforced；包管理器预设及八条额外规则保留，含vuln.go.dev、*.oaiusercontent.com及Dropbox两条。启动revision3/unknown不冒充enforced。正式命令网络授权，平台代理与TLS未禁用，没有分片、公开私有材料或镜像替代官方漏洞库。

Go官方db.json35bytes，Git远端查询成功；重新通过GitHub连接器取临时地址，centralus/westus两个区域的实际ZIP完整下载。PR136两ZIP与用户固定大小/SHA256、CRC、程序SHA、manifest、实际buildinfo全部吻合，源码4bf13186、Go1.25.14、Linux/对应架构、CGO1。四份Dropbox恢复及三归档固定大小/哈希核对通过；旧RC5全部7blob、5layer尺寸/diff_ids、config a50cc418、manifest60ed6a6c及linux/amd64独立核对并加载。未复制生产数据库、配置或挂载，未假称旧559266bytes私有快照已迁移。当前工具无read_thread能力，两条历史任务未发消息；使用用户交接与授权远端恢复、独立重验。

### 独立整改及冻结运行时候选

- [PR138](https://github.com/dreamvm/one-hub/pull/138)，2e987a35：web/fallback静态Range解析前单头/1024bytes/16范围限制。旧实现失败、正常完整/单/多范围及API控制、边界/race/vet与两轮独立只读审阅通过。自身Compatibility38542326791和Smoke38542326711成功。
- [PR139](https://github.com/dreamvm/one-hub/pull/139)，c6813c64：Go1.26.9官方归档/固定builder摘要、go.mod最低版本、CI、实际程序身份/export约束同步。标准库有界201范围旧失败、新通过；原go.mod兼容默认被正常双范围控制检出。新编译器重建govulncheck1.7.0；旧扫描器失败保留。注入localhost媒体测试隔离runner环境变量，未变更平台代理。自身Compatibility38543629362/Smoke38543629340成功。
- [PR140](https://github.com/dreamvm/one-hub/pull/140)，0843087f：依赖PR139，x/net0.60及必要crypto0.57/sys0.48/text0.42/sync0.23；sync由text要求。本地原始H2响应重复/冲突CL和Connection旧失败、新通过；维持原网关uppercase-first代理环境契约，explicit、lowercase fallback、NO_PROXY、CGI零dial与redirect回归通过。独立只读审阅无阻断；自身Smoke38545021742含四项可复用前置检查及双原生架构成功。其base为main以运行所需CI，不能越过依赖合并批准。
- 集成[PR141](https://github.com/dreamvm/one-hub/pull/141)，冻结运行时候选 `76c5657a9ef1eec9cf677b921eca17a90067567a`。十项CI成功：[Compatibility38544767525](https://github.com/dreamvm/one-hub/actions/runs/38544767525)、[Smoke38544774325](https://github.com/dreamvm/one-hub/actions/runs/38544774325)。四个Compatibility任务真实checkout合成192e3c94，六个Smoke任务checkout76c5657a；两树相同 `ad7866b8f9916279c21bf180ebcafa2b95df672f`，逐任务日志已重读。初始164运行被opt-in label触发的165替代而cancelled，不记为失败或通过。

独立修复各保留提交及依赖；PR141已按顶部记录合并，PR138–140修复均被覆盖，PR137仍未合并。此台账文档提交不替换76c5657a的程序与演练身份。

### 双架构实际程序和官方审计

两个原生架构各92项业务/升级PASS、三后端共九轮32并发和四种Compose成功；没有镜像推送。按opt-in现有路径从同一最终CI本地镜像提取程序，完整下载、ZIP digest/大小/CRC、固定两个文件、manifest、日志程序SHA和实际buildinfo逐个吻合：Go1.26.9、one-api、Linux/对应架构、CGO1。

| 架构 | artifact | ZIP bytes / SHA256 | 实际程序 SHA256 / CI本地image ID |
| --- | --- | --- | --- |
| amd64 | 11884828058 | 74425964 / 934660d329fe62f80bddc941eda1ae17c39c8431ad203da045266e1594e1a722 | 2525e9e83a49b4daa9d3ca2896173a3fd9ffd73734684f32da9bb73e4937a011 / a3ab0f2bede84a67c2d3249e2b2e0a581369f5b5b58dc2b155c4638ec88707cd |
| arm64 | 11884678341 | 70473132 / c11dbcad0f1d1b8b9d6a7113cb4817208ee7ad88dc6537a768d67003b265ee63 | d38058c5e73033534fc4850c3f18e82b5a304cac9e2f714cdff27ee1ecbb8957 / 5f181e0dadbd8919f0034c7360911232069e3b38e06137ca8fb052e6c07c4262 |

官方DB更新时间2026-10-08T22:31:09Z。基线源码51模块/25包/19调用、PR136程序各50公告全部原始输入保留。新源码19模块/10包/5调用，两个实际新程序各19符号公告、集合相同；新程序JSON SHA分别2fc27e73ac01eae73fe62c2b18e56e3c053d6977fb95d72535f30dad7080ffdb、f4a57b5d975363dca15ea3ef6619d5ee2a7260e5f88f3cc8f16e0ced5e7e6559。正常JSON退出不代表审计通过。

19输入在当前Linux amd64/arm64主程序范围静态not_actionable，经独立证据审阅；五个调用反证分别为32位限制、固定SQL无dollar-quoted literal（项目确实SimpleProtocol）、WebP configOnly早返回和Markdown原始Title为空/未启用CompletePage且文本转义。不是库已修补或零匹配；改变架构/入口/SQL/renderer/gRPC服务器须重评。详细输入与反证见[固定候选分流表](https://github.com/dreamvm/one-hub/blob/76c5657a9ef1eec9cf677b921eca17a90067567a/docs/CLOUD_GO_20261014_TRIAGE.md)。

### 本地真实RC5与页面联合检查

以冻结提交git archive、新前端、Go1.26.9/CGO1静态编译，在本任务独立构造运行镜像，提取程序SHA f0e2b93f974518b49623f119f3cc8902dc9b19e8e7da6bac5fdbc80b8c3316f4。最终本地image ID95728e0705c23acf6de144305e433e4e790d382fd9215d27a00126f0852e746b。此程序与CI产物编译路径/参数不同，不能写成同一整镜像或registry digest；源码SHA相同并独立核对程序身份。

启动命令在每次运行前验证精确SHA标签对应上述不可变ID；三个数据库后端分别13/36/36PASS，共九轮32并发，覆盖JSON/SSE、权限、故障前拒绝/中途结算、网络分区、Redis缓存权威和恢复、资源释放。不是三架构或生产HA。实际旧RC5到该同一候选的7项全部PASS/exit0：停写合成备份及SHA、原记录/新写入、仅镜像回退保留迁移后数据、全schema/data备份恢复、恢复后旧版读回、临时备份及资源清理。真实模型调用0、production_data_copied=false；另行Docker盘点本轮前缀容器/卷/网络均空。恢复会丢弃恢复点后写入，旧RC5会重新开放Realtime，不能自动降级或猜测生产账务。

本地首次运行镜像缺tzdata导致PostgreSQL Asia/Shanghai失败，补齐运行夹具并重新执行全部三个后端和RC5，失败保留。归档源码初次VCS stamping失败后显式buildvcs=false；不是把失败当通过。实际发布仍使用原Dockerfile/官方CI路线，未发布这一本地运行镜像。

新前端dev/dependency边界、26文件234UI、lint/build通过。初次全并发一项5秒超时，maxWorkers2后完整通过；两次结果保留。官方前端审计175路径/80个Yarn公告ID，低13/中57/高103/严重2，不清零；YAML v3/matcher及原受限审阅继续保留。最终构建279files/18395053原始bytes/5320768逐文件gzip，非首次页面下载量，不从总量推断LCP；前端代码/锁文件未被本批改变。

Chromium以仅loopback透明转发连接拥有的内部Docker真实程序，业务API无替身，外部浏览器请求拒绝。实际表单登录、有限令牌新建与刷新读回、analytics/invoice/model_price/setting/playground受限页面main存在、390px令牌页无全页溢出、UI注销清空用户/self401及受限页重定向共10检查通过，pageerror0。不是各页全部交互、普通用户/有数据图表/真实付款/完整编辑器/暗色或跨浏览器验收。桌面与移动截图已目视核对，表格自身横向滚动保留。

原浏览器直接内部地址403、首次loopback环境请求变化、后续资源429均保留。Playwright拦截禁用缓存并反复全页加载，实际默认180静态资源限额被消耗；仅临时夹具设置GLOBAL_WEB_RATE_LIMIT=5000后功能检查通过，不修改应用或生产设置，也不证明默认容量。保留被拒外部图标和匿名/注销self401控制台错误，不宣称全程零错误。夹具浏览器/监听器/容器/临时数据库已清理；临时请求中断后的forwarder BrokenPipe不冒充应用错误。

### 仍开放的发布/生产边界

本轮普通云端整改、官方源码和真实程序审计、准确候选隔离演练及CI证据已补齐，PR141已专项获批合并；不据此宣布整体RC8可发布。前端残留范围、四项历史受限审阅与单项例外、生产新恢复点/凭据轮换完整性、升级前最新历史账务和OIDC快照、实际生产digest与维护窗口、真实付费/跨浏览器及默认前端容量未由上述合成证据关闭。PR137合并、标签、镜像发布、生产部署、凭据轮换、真实付费与生产写入仍须各自授权。

### 本轮可独立完成的发布准备核实

- 当前网络observations_current=true、spec/observed_revision20、restricted/enforced；八条额外规则及源配置版本不变。历史revision5完整下载核验继续有效，没有禁用平台代理或TLS。
- 远端标签重新读取：维护标签仍以RC7为最新，指向d0f180451f770a7b01f41e52b4e14aeeb64dd924；RC8和正式标签均未创建。计划号继续预留，不因main合并自动发布。
- `.github/workflows/docker-image.yml`仅workflow_dispatch，从默认分支解析已存在且为main祖先的不可变标签；publish默认false，登录仅publish=true，镜像标签仅完整版本和源码SHA、不推latest。手动镜像任务具有packages:write能力，但本轮未调用；isolated smoke仍contents:read/push:false。双架构手动构建需显式选择linux/amd64,linux/arm64，默认仅amd64。
- 正式版本注入VERSION后是新的程序/镜像身份，必须核对实际版本、Go1.26.9/Linux/架构/CGO1、来源SHA、程序和最终registry digest，并验收实际生成镜像。main四项CI及候选本地image ID不代替这一证据；publish=false也不会导出可供运行的整镜像。
- PR141两个原生产物元数据已重核未过期：arm64于2026-10-21T10:07:01Z、amd64于2026-10-21T10:08:32Z到期；本任务已有完整下载并独立校验的副本。七天保留不是持久发布或跨任务迁移；若更换环境，须重新取得完整材料或获授权私有接续，不能假称旧工作目录继承。
- 生产只读证据仍是2026-10-02快照，本轮未连接或写入站点。候选合成恢复点不能代替新生产备份；升级前需要按已有规则重核最新历史账务/OIDC、凭据完整性、新备份与恢复校验、镜像digest、停写决定、恢复点后数据损失边界及维护窗口。旧镜像回退会重新开放Realtime，须另行决策。
