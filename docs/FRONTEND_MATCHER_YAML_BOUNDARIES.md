# 匹配器与 YAML 的剩余路径

2026-10-01，接续[21条可选工具记录](FRONTEND_OPTIONAL_TOOL_PATHS.md)。
本批核对最后75条原始路径输入，依赖文件与[205条准确审计](FRONTEND_AUDIT_FOLLOWUP.md)相同。
每条原公告、版本范围、依赖路径和来源均保留，没有按相同GHSA或物理安装位置删除输入。

## 静态结果与边界

沿用codex-security:triage-finding，先规范化全部75条，再逐项内联核对。
阶段内没有运行目标库、测试、构建、应用、PoC或资源耗尽验证，也没有修改仓库或PR。
该阶段已结束；现有#118 CI的独立只读采集不属于新漏洞验证。

| 输入组 | 路径记录数 | 当前维护入口的结果 |
| --- | --- | --- |
| brace-expansion：8个公告、每个4条路径 | 32 | not_actionable，高静态置信度；受检调用条件缺少特制pattern入口 |
| minimatch：3个公告、每个4条路径 | 12 | not_actionable，高静态置信度；同上 |
| picomatch：Sass watcher两条、react-app六条 | 8 | not_actionable，高静态置信度；当前配置未调用对应可选分支 |
| yaml：react-app/macros配置解析一条 | 1 | not_actionable，高静态置信度；当前未选macros预设 |
| js-yaml：5个公告、每个2条路径 | 10 | needs_review，中等置信度；实际启用完整YAML配置解析，输入归属/支持边界尚缺 |
| picomatch：Vite及tinyglobby的6条图路径、各2个公告 | 12 | needs_review，中等置信度；实际matcher API存在，完整pattern来源未证实 |

共53条受检入口不适用、22条待核实、零条已确认项目漏洞。
这是静态第一轮判断，**全部受影响库版本仍安装着**；没有称库已修补或完成动态验证。
无适用web SECURITY.md，不能仅凭“本地文件”“开发依赖”或当前普通输入排除安全边界。

## brace-expansion / minimatch：四条图路径分别保留

前三条均解析到minimatch3.1.2 / brace-expansion1.1.11：

1. eslint → minimatch → brace-expansion。
2. eslint → @eslint/eslintrc → minimatch → brace-expansion。
3. eslint → file-entry-cache → flat-cache → rimraf → glob → minimatch → brace-expansion。

第四条为vite-jsconfig-paths → recrawl-sync → sucrase → glob → minimatch9.0.5 /
brace-expansion2.0.1；minimatch公告的路径截止minimatch，未另生成不存在的输入。

| 公告 / 原记录编号 | 缺少的必要输入形式 |
| --- | --- |
| [v6h2-p8h4-qcjw](https://github.com/advisories/GHSA-v6h2-p8h4-qcjw) / 1,2,3,87 | 导致解析正则复杂度异常的brace pattern |
| [f886-m6hf-6m8v](https://github.com/advisories/GHSA-f886-m6hf-6m8v) / 15,16,17,89 | 零步长数值/字母序列 |
| [3jxr-9vmj-r5cp](https://github.com/advisories/GHSA-3jxr-9vmj-r5cp) / 19,20,21,91 | 连续非展开brace组 |
| [mh99-v99m-4gvg](https://github.com/advisories/GHSA-mh99-v99m-4gvg) / 23,24,25,93 | 产生过量展开结果的pattern |
| [rgw5-rvv9-x895](https://github.com/advisories/GHSA-rgw5-rvv9-x895) / 26,27,28,95 | 产生过量中间数组的pattern |
| [q2hr-2g5m-vwhr](https://github.com/advisories/GHSA-q2hr-2g5m-vwhr) / 31,32,33,97 | 反复改写并重解析的pattern |
| [qhr7-859c-m2p7](https://github.com/advisories/GHSA-qhr7-859c-m2p7) / 34,35,36,99 | 深层嵌套brace |
| [6j4f-fj2g-mc7p](https://github.com/advisories/GHSA-6j4f-fj2g-mc7p) / 37,38,39,101 | parseCommaParts递归嵌套 |
| [3ppc-4f35-3m26](https://github.com/advisories/GHSA-3ppc-4f35-3m26) / 5,6,7,107 | 重复通配符和失败匹配 |
| [7r86-cg39-jmmj](https://github.com/advisories/GHSA-7r86-cg39-jmmj) / 8,9,10,108 | 多个不相邻globstar和失败匹配 |
| [23c5-xmqv-rm74](https://github.com/advisories/GHSA-23c5-xmqv-rm74) / 11,12,13,109 | 嵌套extglob量词和失败匹配 |

直接lint/lint:fix只传固定`src/**/*.{js,jsx,ts,tsx}`，当前普通cwd没有pattern元字符。
它含四个扩展名备选和一个globstar，没有上述序列、嵌套、改写或extglob输入。
文件名作为匹配对象不等于文件名被编译成pattern；不能把这一控制推广到任意cwd或CLI参数。

@eslint/eslintrc的Minimatch位于OverrideTester。当前.eslintrc及所选六个预设源码没有
overrides/files配置；package.json内react-app回退未被选中。
缓存分支仅options.cache=true时构造LintResultCache；现有命令没有--cache。
非缓存时删除旧缓存采用直接fs.unlinkSync，并不进入rimraf/glob；没有假定旧缓存可信。
jsconfig插件运行recrawl API；recrawl运行文件不导入Sucrase CLI，维护脚本未启动该CLI。
上述四类反证分别对应原图路径，不能互相替代。

## js-yaml：JSON形状不代表JSON解析

js-yaml4.1.0解析到同一根安装位置；原记录4/116、18/117、22/118、29/119、30/120分别对应：

- [mh29-5h37-fv8m](https://github.com/advisories/GHSA-mh29-5h37-fv8m)：merge可能改变解析结果对象原型。
- [h67p-54hq-rp68](https://github.com/advisories/GHSA-h67p-54hq-rp68)：重复alias merge的二次工作量。
- [52cp-r559-cp3m](https://github.com/advisories/GHSA-52cp-r559-cp3m)：merge链的二次工作量。
- [5p4m-2wfm-xmqj](https://github.com/advisories/GHSA-5p4m-2wfm-xmqj)：!!omap重复键检查的二次工作量。
- [2883-xcg3-v3hh](https://github.com/advisories/GHSA-2883-xcg3-v3hh)：空merge来源绕过工作量计数。

两个图路径eslint → js-yaml、eslint → @eslint/eslintrc → js-yaml各自保留；
图路径不是两条独立应用漏洞的证明。ESLint自己的TAP formatter使用dump；实际配置解析
来自@eslint/eslintrc共享根库的load，不把dump误报为解析sink。

无扩展名web/.eslintrc实际进入loadLegacyConfigFile，先执行
yaml.load(stripComments(readFile()))；.yaml/.yml也进入完整YAML解析。
只有.json后缀走JSON.parse。配置schema验证位于解析之后，不能限制解析阶段的工作量。
调用方未传restricted schema、深度或工作量限制；4.1.0默认schema包含merge与!!omap。
merge未对重复来源去重，omap使用array.indexOf；这些实际API足以保留待核实状态。

当前提交文件没有merge alias、!!omap或危险键；未发现HTTP/应用YAML解析器，也没有证明
全局Object.prototype污染或项目可利用路径。谁可提供lint配置、YAML是否为维护接口及
完整兼容边界仍需核实，不能将当前普通文件当作关闭依据。

## picomatch / yaml：启用与可选路径分开

[3v7f-55p6-f55p](https://github.com/advisories/GHSA-3v7f-55p6-f55p)的十条记录为
69,128–133,198–200；[c2c7-rcm5-vvqj](https://github.com/advisories/GHSA-c2c7-rcm5-vvqj)为
70,134–139,201–203。两组各自保留全部原路径。

Sass1.72.0的可选watch/chokidar/anymatch解析到picomatch2.3.1；当前默认SCSS编译
不启动Sass watch CLI，watcher本身还设置disableGlobbing:true。后者不是任意watcher输入免疫。
react-app下三个TypeScript parser/type-utils图路径也解析到2.3.1；当前没有选react-app。

六条Vite路径为vitest → vite、vitest → vite → tinyglobby、vitest → vite-node → vite、
vite → tinyglobby、vitest → vite-node → vite → tinyglobby、vite，均到picomatch4.0.3。
根Vite为7.3.5，Vitest与vite-node的独立Vite为7.3.6；两个物理picomatch安装分别核对，
没有把根版本代入嵌套库。Vite string filter/fs.deny、import-glob变换、package export glob
及tinyglobby确有受影响API；POSIX属性查找与extglob编译仍存在，64KiB长度限制不是执行时间限制。

当前React include是RegExp；默认fs.deny、jsconfig和测试pattern为普通固定值，应用源码没有
import.meta.glob或模板动态import。serviceWorker普通new URL不是编译期glob入口。
这些是反证，尚未完成所有插件、package、watch及HTTP来源的pattern清单。
server.host=true事实保留，没有按默认localhost排除；十二条均needs_review。

yaml1.10.2的原记录204为react-app → Babel preset → macros → cosmiconfig → yaml，
对应[48c2-rrv3-qjmp](https://github.com/advisories/GHSA-48c2-rrv3-qjmp)。当前Babel只有React预设，
其源码不包含macros，应用没有macro配置/import；lazy cosmiconfig YAML分支未被调用。
这不代表yaml库已修补或未来宏配置安全。

## 证据、队列与交付

私有补充证据保存于artifacts/frontend-audit-20261001，每条输入包含来源、路径、
范围判断、反证、缺口及下一步；22条needs_review使用独立连续排名，没有与确认漏洞混排。

| 文件 | SHA256 |
| --- | --- |
| remaining-matcher-yaml-normalized-inputs.json | 2320235b60d152a62741565a111bfa54ace4c719ef14f84213f13cb3c5fcd08e |
| remaining-installed-resolutions.json | d6947db1561751701187069d7038bb2c99f13be9dbca922d6c50ed08cc85a16c |
| remaining-matcher-yaml-static-evidence.json | 6a7f2512fa6e6dfb34f4cc4bd2e829e3f002355049c65115da01718547a61ae2 |
| remaining-matcher-yaml-triage.json | 53da44e232efab33163a7dd5472b52a0b4ac28ad120aa1d8bf164c727c852e41 |
| remaining-matcher-yaml-manifest.json | ca809b32bbf16b7e50e17e7862e8407004f23ad301cd00e70299d39811024e53 |

最后75条未初筛队列归零，**22条待核实及此前103条记录各自的缺口继续开放**。
此前六条和后续21条静态结论范围不扩大；205条匹配完整保留。
下一步优先核实实际YAML调用的兼容修补，再完成Vite pattern来源核对；这是后续独立工作。
本批文档自己的独立证据审阅、准确候选CI及合并尚待完成。
四个受限审阅未重试或改道；没有标签、镜像发布、部署或真实付费调用。

一次非作者、复用上下文且非fresh的独立只读文档证据审阅无具体不一致。
核对75条身份、53/22分组、源码/安装哈希及#118准确候选、真实合并和main验收；
未作新triage或执行测试。该审阅不构成后续js-yaml修补候选审阅，
也不关闭22条待核实、此前缺口或四个受限审阅。

## 2026-10-01 #119合并验收

候选f9db638add373972bf28323ccd421cd5b44cd819自己的十项检查成功：
[Compatibility36931754911](https://github.com/dreamvm/one-hub/actions/runs/36931754911)、
[Isolated36931755330](https://github.com/dreamvm/one-hub/actions/runs/36931755330)，均attempt1。
四项Compatibility真实checkout为合成efdd2674，六项Isolated为准确候选。
候选/合成/基于最新main计算/实际合并内容树均为6d3f01d51b462b8c486267b645ab6a1356c6390d；
实际合并dac105ff0e712793b86f4f5dc679277a561d62a6，父为e84d12c1/f9db638a。
[本次main CI36934262113](https://github.com/dreamvm/one-hub/actions/runs/36934262113)已成功，准确push head为dac105ff、attempt1，四项真实checkout均为该合并提交。

两项前端各22文件/209 UI通过、ESLint无诊断、build成功，chunk及安装peer提示保留。
每个原生架构89PASS/9waves/4Compose，最终程序Go1.25.14、one-api、Linux/对应架构、CGO=1。

| 架构 | CI本地image SHA256 | 最终程序SHA256 |
| --- | --- | --- |
| amd64 | 9a6a18db7de34b5ba9d6c6bcac675f088d0da7f3dba84b35537b5d62d73db3be | e8c15fd1de18e2d96b081de73173e08e6c8939e7708c5c9640fcb70cfa29b51a |
| arm64 | b080c6dd6885e0df397ee8e6c2820a56eb93059ab6071b2d344c28863e793c65 | ba3036027d5e4793adab83d04a6c4e18cc86145481721889cf21dedc975eaf05 |

只读CI证据采集及父代理核对完成；34份日志/元数据/清单保留于artifacts/pr119-ci，
verification.json SHA256为faca7b55dda224d3304ecbd3b6983a38396196eae6e2c690fba4b58fa865dd84。
后续merge-verification.json SHA256为ad1802db00836d72a94ab371b99cb4de4670a586d4ce9709ba555da78e61f5a2；原23份候选证据保持不变。
该文档交付不修补库，不替代后续安全候选的独立审阅。

后续[共享YAML v4候选](JS_YAML_CONFIG_BOUNDARY.md)已完成有界旧新对照、本地完整回归及
一次fresh独立审阅；自身准确候选CI、合并和main仍待完成，独立v3和Vite十二条继续开放。
