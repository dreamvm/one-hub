# Vite维护入口的模式输入记录

2026-10-02，补齐[原十二条Vite路径](FRONTEND_MATCHER_YAML_BOUNDARIES.md)的来源证据。
**本批是输入清单和正常构建观察，不关闭公告、修补候选或发布阻断项。**
[4.0.7候选](VITE_MATCHER_CANDIDATE_REVIEW.md)仍因已确认的残留和兼容回归保留TEMP，
没有应用、运行时提交、PR、准确候选CI或合并，也没有第二轮安全候选审阅。

## 基线与证据层级

实际main为0e0acea7e6920448a94257c8e7b87722c1d02e78；观察使用#122候选79e9e9b1的
独立Git归档副本，其web树与实际main均为f2ea80b7f05492f3b4e82a4356df594fbb750f9f。
package SHA256为41d2fd6dda846936242f837c199e73b6607031e6197b0e798cc0a0ad8b224bda，
lock为6b7c1d5d020ab518e98f184bd67465dcfbf0eda063630b6875fc1c4892d2c29d。
使用当前共享v4依赖副本，没有使用保留中的picomatch或YAML v3候选。

根Vite7.3.5、Vitest/vite-node各自的Vite7.3.6图路径与实际picomatch4.0.3继续分开。
三个受检Vite config.js字节相同，SHA256为c2fcfa51206dc1f8d22120ef4a82af8b9acdc02f4476520b6a0897d12a724ba3；
版本元数据仍分别核对。Vitest直接picomatch4.0.7/tinyglobby0.2.17不替代嵌套Vite路径的4.0.3/0.2.15。

| 证据层级 | 实际记录 | 能证明的范围 |
| --- | --- | --- |
| 安装元数据目录清单 | 2130份物理package.json；1073安装布局根、1057包内辅助文件；JSON错误0 | 指定依赖副本的字段和来源，不证明Vite实际读取或调用 |
| 正常构建packageCache | 439份已解析元数据，范围/读取缺口0 | 本次构建加载元数据，不证明每个字段分支均执行 |
| 模块观察 | 19673个模块、39346条pre/post记录，记录上限未触发 | 两个观察点的哈希及字面标记，不覆盖所有插件间的中间源码 |
| 实际编译观察 | 12条按模式、选项、调用栈去重的记录，共26次closure-makeRe调用 | 本次已观测的默认deny及sideEffects过滤，不是所有API调用的完整追踪 |

packageCache内两个array sideEffects提供方为country-flag-icons1.5.19的CSS模式，
和highlight.js11.10.0的es/common.js、lib/common.js、CSS、SCSS模式。
其元数据SHA256分别为5f979d8b2cf213b30463d747076dd83d8cb9355adac4137d7aee4f0b6ab80f5a和
db481b556af8e62ba850175b4ae07c82d2e86ff38c165c6e5463104d93c5b1a7。
Vite在1699–1709行把无斜线项加globstar前缀，其余以包目录解析；1503–1525行的string filter
编译模式，32605/32817行实际查询hasSideEffects。观察到CSS/SCSS及包目录内common.js模式，
不把相同CSS模式的每次调用全部归给某一个包。

## 固定维护入口与可选输入

以下同时保留提供方、消费条件和证据层级；匹配对象与模式提供方各自记录。

| 路径 | 当前提供值和条件 | 当前证据及限制 |
| --- | --- | --- |
| React/core string filter与hook filter | React默认include为RegExp；CommonJS include、dynamicImportVars exclude为node_modules RegExp；assetsInclude未配置 | 配置及源码，普通build已记录；未来string配置、插件和cwd仍可改变输入 |
| fs.deny | 固定.env、.env.*、crt/pem模式和.git模式，Vite补basename前缀 | 四项编译已观察；HTTP路径是匹配对象，host=true保留，未证明请求者提供模式 |
| sideEffects | 当前439份元数据内上述两个数组；依赖包提供字段 | filter编译已观察；包安装/写入权限和任意未来包元数据的信任边界未证明 |
| import.meta.glob / 动态import | 源文件或插件产生的静态glob/模板段提供模式；运行时参数选择生成的导入键 | 应用源码无相关入口；pre/post标记为0，不证明插件间所有中间输入或未来源码均不存在 |
| worker/asset URL | 五份Monaco模块含固定worker URL；另一份含诊断文字和运行时URL | 六模块/两阶段共12个共现标记；文字共现不等于glob调用，未来动态模板分支保留 |
| optimizeDeps / package exports | 普通React注入四个固定包名；只有dynamic include才进入exports/glob展开 | build resolved配置记录四名；固定Vitest配置另被覆盖，不能混用；仅exports字段存在不证明展开 |
| scanner / warmup | dev scanner默认HTML模式和outDir忽略；当前warmup为空 | 源码与build resolved配置；scanner/dev执行未在本批动态观察，未来entries/warmup值保留 |
| jsconfig paths | 当前baseUrl=./src、include/exclude固定；没有paths映射 | 使用globrex/glob-regex，不能归给picomatch；root、生成配置和未来映射仍可改变输入 |
| PostCSS目录消息 | 当前默认搜索stopDir=web，无配置；内置import发dependency、modules发export；dir/glob由消息插件提供 | 消费方及产生方源码已核对，实际消息数组未捕获；literal搜索未找到发出方不能排除动态构造 |
| CommonJS动态require | 当前没有dynamicRequireTargets；可选配置进入fdir.glob | 固定配置/源码，不证明所有可选require目标安全，也不另增原始审计claim |
| dev watcher | 默认ignored含.git/node_modules/test-results及escaped cache/outDir；bootstrap和core插件可增加文件 | 源码；disableGlobbing限制target解释，不取消ignored编译；本批未动态启动dev watcher |
| Vitest固定run | test include固定tests/**/*.test.jsx；CSS include为RegExp；optimizer include/entries为空、noDiscovery=true；watch=false/server.watch=null | 21项插件/配置静态清单；direct discovery与嵌套Vite分开，没有在本批重跑test或记录test resolved对象 |
| vite-node | 当前ViteNodeServer复用Vitest已有server；独立CLI可创建自己的Vite server | 静态构造链；当前scripts不调用独立CLI，不能把该图依赖写成本次新增server实例 |

VitestOptimizer的post config hook覆盖React原四个include；以后test配置补默认值，
不能反推更早Vite hook已启用优化器。固定run不注册watcher、api无port不进入listen；
这些只限现有命令和配置，不扩大为所有HTTP或可选API安全结论。

PostCSS/core watcher补充核对40个源码/元数据身份、113处hash/行号引用：8项PostCSS来源、
14项watcher来源、6项配置覆盖来源。当前两份web快照在17个PostCSS搜索名中仅有package.json，
其中没有postcss字段；默认workspace边界为web。未来root、工作区标记、配置函数或插件仍可改变选择。
根代理另核对上述两份web及正常构建副本共31层祖先：只记录workspace标记存在性和
package.workspaces布尔字段，三处默认stopDir各为自己的web；没有执行配置或保存其他字段。
watch文件来源包括配置/环境/public目录、包元数据、CSS/preprocessor deps、模块/HTML proxy，
以及当前Monaco两个?worker导入对应的Rollup bundle.watchFiles；文件名与派生ignored模式分开。
普通build的addWatchFile是依赖记账，不证明启动持续watcher；实际watchFiles/消息/事件集合未观察。

## 正常观察与未覆盖范围

两次独立临时构建使用Node22.20.0/Yarn1.22.22和显式yarn run build，均成功：
第一次17.642秒，修正观察器后第二次15.805秒；实际日志均为Vite7.3.5、19673模块，
保留chunk提示。package/lock未改写，没有安装、升级、审计或安全载荷。
各命令180秒超时、4096MiB普通构建上限；模式记录10000条、模块阶段记录60000条上限。
观察步骤只使用归档副本、当前隔离依赖和临时输出，没有修改仓库生产源码或依赖源码。

两个观察plugin的transform均返回null。第一版CJS导出包装未捕获调用，空记录明确为观察缺口，
没有据此排除调用；第二版仅在内存中包装closure API属性并委托原函数，实际捕获makeRe。
未证明所有ESM导出、直接scan/parse、支持API及模式调用均被观察；源码缺口不由空记录关闭。
初次helper生成语法错误发生在工具执行/文件生成前，修正后才运行；不是库测试失败或通过。

本次没有HTTP探针、攻击输入、真实OOM/中断、第三方扫描、真实凭据、翻译或付费调用。
未知插件/配置、安装与写入归属、运行时dev/test分支、全部中间生成源码及远程提供模式路径
仍待证据；十二条继续needs_review。普通固定输入的观察不能替代完整且兼容的共享修补。

私有补充文件位于artifacts/vite-pattern-input-inventory，完整元数据、模块哈希、调用栈、
两次日志、原观察缺口和helper均保留；公共记录不包含源码正文或真实业务数据。

| 证据 | SHA256 |
| --- | --- |
| 安装字段完整目录 | b53651f74b27bf944c6aec56685b16dbd36d1d74b39fe74a94e4672db8844837 |
| 21项插件输入来源 | b9660f9bfe291b793de140b102ea020d5d11a49076d2e0c0118f0043efe8700b |
| PostCSS/core watcher来源补充 | 881fce5ae0b5570929aea2146af99ad4cc3216ace8137e2ebd571c09308a502f |
| 根代理workspace边界核对 | 94502b5b452f339a5fa454cc4bd1769fd3b806bef8fc8782a9ceeb0902df0996 |
| 第二次正常构建日志 | 63620ef6cad63ef8c9d9eeb2be75740afd20b17beb9c095a77891b67b5645160 |
| 第二次resolved配置 | 38023d58fb6b51b920970c29f90f0afe869c8f05a82920b770cd31a6ff044aca |
| 第二次实际packageCache | 46af461e7378db8bc7c3a0e3fade82e85439a9fbb62689eb5f2bc5286c07c0ca |
| 第二次模块阶段记录 | 06b9adff45ff042bf54e5da420e56def8aed963bd69bcb5f5e947771bb4ac62d |
| 第二次实际编译记录 | 791f607ed9625102140359b603e452cee771ff7cfc83bb23afd06febe292a18c |

本批文档自己的独立事实审阅、准确候选CI及合并尚待完成；这些均不交付运行时修补。
四项受限审阅没有重试或改道，YAML v3、真实历史归属、生产事实和最终RC8门槛继续保留。
没有标签、镜像发布、部署或真实付费调用。
