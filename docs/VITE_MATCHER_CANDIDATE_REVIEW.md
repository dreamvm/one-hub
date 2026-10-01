# Vite匹配器候选的审阅与保留状态

2026-10-01，接续[原十二条Vite记录](FRONTEND_MATCHER_YAML_BOUNDARIES.md)。
**4.0.7运行时候选未通过完整修补与兼容验收，只保留在独立临时副本。**
未应用到仓库、未提交、未创建运行时PR、未通过候选CI、未合并。
当前main的共享YAML修补已由[#120](JS_YAML_CONFIG_BOUNDARY.md#2026-10-01-120合并验收)交付。

## 实际来源与最小候选

[POSIX继承方法错误匹配](https://github.com/advisories/GHSA-3v7f-55p6-f55p)和
[重复extglob复杂度](https://github.com/advisories/GHSA-c2c7-rcm5-vvqj)的六条原图路径各自保留：
根Vite及tinyglobby、Vitest内Vite及tinyglobby、vite-node内Vite及tinyglobby。
根Vite7.3.5、两个独立Vite7.3.6均实际解析picomatch4.0.3；Vitest直接4.0.7不能替代它们。

fresh只读调查与父代理追踪补齐filter、fs.deny、package sideEffects、导入glob/动态URL、
exports展开、scanner/warmup、PostCSS目录消息、CommonJS动态require和watch ignored。
country-flag-icons的sideEffects实际到达CSS filter，React注入四个固定optimizeDeps值；
Vite bundled watcher的ignored仍编译v4模式，disableGlobbing不能排除该分支。
受检HTTP链上的路径是匹配对象，业务HTTP到pattern来源和攻击者归属尚无证据。
保留host=true及未知插件、配置、依赖元数据的支持边界；不宣称已确认远程项目漏洞。

临时候选只把^4.0.3合入现有^4.0.2/^4.0.4的4.0.7锁条目，增加固定回归及test:deps入口；
没有升级Vite、tinyglobby或其他major。官方完整性及安装十个源码文件字节相等已核对。
默认重复保护采用安全展平或字面匹配，并非抛错；maxExtglobRecursion:false会跳过分析，
受检调用未设置该选项。POSIX真正控制为posix:false，未设置时仍启用；noposix不是该开关。
4.0.6的scan变化实际到达tinyglobby的parts:true，不能称所有扫描行为未变。

## 有界对照与未通过审阅

两份独立副本基于b7677242/实际合并相同树，使用同一固定源码：

| 检查 | 结果及范围 |
| --- | --- |
| 旧4.0.3 | 19正常叶子通过、12修补断言失败；Node含父32项为19通过/13失败 |
| 4.0.7候选专项 | 31叶子/Node含父32项通过；三份实际Vite filter、正常POSIX/展平、默认deny及实际tinyglobby扫描 |
| 冻结安装 | 正常安装成功，package/lock未改写，peer提示保留 |
| 完整前端 | 实际运行26.79秒；22文件/209 UI，既有HTTP/HMR文件边界及依赖回归通过 |
| lint/build | ESLint无诊断；普通4096MiB上限构建14.59秒通过，大chunk提示保留 |

固定短模式/短字符串和自建临时标记，worker192MiB、8秒、32KiB输出，专项90秒；
未用长失败匹配、耗时攻击、OOM或服务中断验证。
初次Yarn包装参数只列出脚本，退出0仍记为未执行；修正为显式run并核对执行标记后才记通过。
Sass元数据helper的初次package exports错误为设置失败，保留并修正，非库缺陷。

一次fresh、非作者、只读候选审阅发现具体残留，父代理用短输入及实际tinyglobby复核：

- 分析器未涵盖多字符前缀重叠、嵌套非重复包装及大小写折叠的替代表示，部分模式仍生成
  重复重叠结构。这里验证编译结构和短正常匹配，未测阻塞耗时。
- 展平前trim删除分支内有意义的字面空格；只创建三个标记，旧tinyglobby选中带空格文件
  与另一正常分支，候选改为无空格文件与另一分支。

源码位于picomatch/lib/parse.js122–161、241–346、539–568及1095–1099；
受检Vite/tinyglobby调用没有后续拒绝这些表示。当前固定模式不含这些形式，
不据此称One Hub可被远程利用，也不忽略候选残留和兼容变化。
POSIX空原型查找表未发现六条目标路径的具体绕过；不替代另一公告的完整修补。
没有第二轮候选审阅、泛化净化器或强制改变所有调用模式。

## 准确候选审计及新增两条输入

授权预检确认937个公开包名未变，实际只POST官方registry.yarnpkg.com；
字段不含源码、凭据、内部地址、resolved URL、scripts或proxy。
压缩前JSON289439字节，预检与发送前哈希相同，不是线上gzip字节哈希。
结果185路径/91Yarn ID/72GHSA，低13、中62、高108、严重2，退出30、一个summary、零error。
相对#120同树的195条，十二个Vite claim key消失、两个Sass claim key新增，净减少十条。
**185仅为未合并候选快照；当前main快照仍为195。公告匹配消失不关闭已发现的候选边界。**

新增Yarn1115549/1115552各自保留公告、范围和sass→chokidar→readdirp→picomatch2.3.1路径。
先规范化全部两条，再按triage-finding内联静态核对；阶段内没有库执行、测试、构建、
新探针、仓库修改或子代理。两条在受检图路径均not_actionable/高静态置信度：

- readdirp/index.js36–55直接返回function filter，仅string/array才编译picomatch。
- nodefs-handler.js468–472及fsevents-handler.js470–474两个调用均传箭头函数；
  Depth仅返回depth，chokidar/index.js939–942保留这些函数，不进入编译分支。
- Sass1.72.0、chokidar/readdirp3.6.0及五份受检源码在旧新副本相同，readdirp均解析独立2.3.1。

无适用web SECURITY.md；以上依赖实际分支反证，不按“本地工具”假定可信。
2.3.1仍受影响，任意直接readdirp字符串API和独立chokidar/anymatch不由这两条关闭。

## 证据及下一步

私有artifacts/picomatch-candidate保留完整输入、发送字段/接收端、旧新日志、准确diff、
官方源码身份、唯一fresh审阅、根代理复核与两条静态输入/结果。

| 证据 | SHA256 |
| --- | --- |
| 三文件候选diff | 0a27e6a05e3b3e423af10f8ce8723f3b31e0ff3e6b3c2f2996ec46607fc1ac6c |
| 同一专项源码 | 8e325706604e0a38e54f124f5d7ab60ff3eb7dc333a204088c2c98d99c2bbf13 |
| 旧版最终日志 | b43dd29107a7f604034bc0c4380a3efe457f725e15df7795ca7cdc1832815017 |
| 候选专项日志 | 2c3997b5af5f2d053cbb739224a1030364828bfbbb2482de5d5f30d63ceeefec |
| fresh候选审阅 | 8c5626edfefede715f9a32d0ca5c444c2fb2bfbd23bf6575aac7c10487c48779 |
| 根代理tinyglobby复核日志 | eaf3111dd302929f0dfbaf49ca02b92af04d0e1b7c4fd030f79f008c09791bef |
| 候选原始审计 | 648593c7afa23db5aef9aad78c92f9397f7db9c0c1989e661962825f6bde2fe9 |
| 新增两条静态结果 | cda97979fb3e147d5f4f390138e99de56d37e2185b5c13bf46404402ea853bb1 |

运行时整改仍待兼容且完整的共享修补，或经核实的支持输入边界及相应证据；
原十二条保持开放，继续独立已授权任务。本文件不是运行时候选交付。
四个受限审阅、独立YAML v3、历史归属、生产事实及最终RC8门槛继续开放，没有发布或部署。
