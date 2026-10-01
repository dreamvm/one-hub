# 共享 js-yaml v4 配置边界

2026-10-01，接续[最后75条范围记录](FRONTEND_MATCHER_YAML_BOUNDARIES.md)。
本批只更新 `js-yaml@^4.1.0` 的共享锁条目：4.1.0 → 4.3.2。
候选基于实际合并 #119 的 dac105ff；其 web 内容与本地旧新对照基线 e84d12c1 相同。
本地通过和审计匹配减少不等于完成候选 CI、合并或发布。

## 实际入口与不变式

无扩展名 `.eslintrc` 由 @eslint/eslintrc 的实际 CJS 配置加载器执行完整 YAML 解析，
随后才验证配置 schema；`.yaml`、`.yml` 使用同一库。
当前文件虽然是 JSON 形状，也不因此走 JSON.parse。
解析结果应保留普通对象原型，将特殊键作为自有数据键；合并处理必须受工作量限制。
没有证明项目 HTTP YAML 入口、攻击者配置归属或全局 Object.prototype 污染。

ESLint、@eslint/eslintrc、cosmiconfig 和 langchain 均解析到这个共享 v4 安装。
ESLint 自身的 TAP formatter 使用 dump，不能把它误报成解析入口。
此次保留现有 API 和完整 YAML 配置支持，不改配置文件名、ESLint 版本或应用流程。

原审计两条 ESLint 图路径各有五个公告，共十条原记录，分别保留：

| 公告 | 受检问题 |
| --- | --- |
| [mh29-5h37-fv8m](https://github.com/advisories/GHSA-mh29-5h37-fv8m) | merge 改变解析结果对象原型 |
| [h67p-54hq-rp68](https://github.com/advisories/GHSA-h67p-54hq-rp68) | 重复 alias merge 工作量 |
| [52cp-r559-cp3m](https://github.com/advisories/GHSA-52cp-r559-cp3m) | merge 链工作量 |
| [5p4m-2wfm-xmqj](https://github.com/advisories/GHSA-5p4m-2wfm-xmqj) | !!omap 重复键检查工作量 |
| [2883-xcg3-v3hh](https://github.com/advisories/GHSA-2883-xcg3-v3hh) | 空 merge 来源绕过工作量计数 |

## 最小补丁与兼容边界

所有共享 v4 调用方的范围均允许 4.3.2，argparse 依赖范围不变。
只修改上述锁条目的版本、官方 npm resolved URL 和完整性值；没有全局 resolution。
官方包 SHA512/SHA1 已核对，冻结安装后的36个发布文件与下载包逐字节相同。

4.3.2 的实际 loader 使用受保护属性写入；每个 merge 来源（包括空来源）和键均计费，
默认 maxTotalMergeKeys 为10000，loadAll 的一次调用跨文档共享预算。
每个 merge sequence 在循环前执行100项硬上限；!!omap 通过自有属性集合检查重复键。
依据[上游 v4 变更记录](https://github.com/nodeca/js-yaml/blob/v4/CHANGELOG.md)及实际发布源码，
没有把旧版二次工作量问题写成已经完成动态复杂度或资源耗尽验证。

升级也带来上游语义变化：带下划线的数字标量改为字符串，部分无效 block scalar 被拒绝，
空白折叠和标签处理有修正，默认 maxDepth 为100。当前仓库配置和正常合成配置通过；
不能据此保证任意旧 YAML、其他项目配置或下游对象遍历兼容，也不声称任意不可信 YAML 安全。

gray-matter 的独立 v3 3.14.1 保持原样，因为它使用 safeLoad/safeDump，不能被全局强制为 v4。
其可选 Markdown/front matter 路径仍含同类既存缺陷；正常 front matter 对照只证明本次未破坏该 API。
[固定 JSON i18n 维护范围](I18N_MAINTENANCE_SCOPE.md)没有启用该模式，不代表库已经修补。
此入口单独保留整改记录，不计入本批十条共享 v4 修补结果。

## 有界旧新对照与独立审阅

新增 `web/tests/js-yaml-config-boundary.test.mjs`，纳入现有 test:deps。
只有固定合成用例，配置小于4KiB；仅在测试自建临时目录写入标记，使用随机临时名称并清理。
每个子进程192MiB堆、8秒超时、32KiB输出上限；套件90秒上限。
没有真实密钥、生产数据、第三方扫描或用于证明漏洞的 OOM/服务中断。

| 对照 | 4.1.0 | 4.3.2 |
| --- | --- | --- |
| 三种配置后缀、plain/quoted 特殊键的6项原型断言 | 6失败 | 6通过 |
| 三种配置后缀的101项小型 merge sequence 拒绝断言 | 3失败 | 3通过 |
| CJS 三种 load/loadAll 形式及 ESM 的空来源预算/跨文档断言 | 4失败 | 4通过 |
| 正常100项合并、真实仓库配置、omap重复错误、TAP及共享调用/独立v3对照 | 7通过 | 7通过 |

共20个叶子用例；旧版13个安全断言失败，候选全部通过，Node含父测试汇总为21项。
初次旧版运行的仓库正常对照缺少 ESLint 内置预设提供者，是测试设置错误；
修正为实际 ESLint.calculateConfigForFile 后，旧新使用完全相同的测试源码。
初始失败记录保留，不能计作库缺陷。

一次 fresh、非作者、只读绕过/回归审阅没有发现共享 v4 候选的具体绕过或兼容回归。
审阅指出上述独立 v3 的既存残留，父代理源码复核确认并单独保留；没有第二轮或受限审阅重试。

Node22.20.0/Yarn1.22.22 冻结安装、完整 yarn test、lint、build 均通过，依赖文件未被改写。
UI为22文件/209项通过，ESLint无诊断，安装 peer 与构建大 chunk 提示保留。
首次完整测试因沙箱拒绝 localhost 监听而未运行，允许已授权隔离监听后重跑通过。
首次普通构建的1536MiB人为堆上限不足；改用约普通默认上限的4096MiB后通过，
两份构建记录均保留，不把初次失败隐藏或当成安全测试证据。

## 准确依赖复查与交付状态

22:14 UTC 的复查使用候选相同 package/lock 哈希。无网络预检确认937个公开包名不变，
新增版本仅 js-yaml4.3.2；字段不包含源码、凭据、内部地址、resolved URL、scripts 或 proxy。
实际仅向官方 registry.yarnpkg.com 的审计 HTTPS 端点 POST，明文 JSON 的发送前哈希与预检一致。
没有把压缩前 JSON 的哈希误称为线上 gzip 字节哈希。

结果195路径/93个Yarn ID/72个GHSA，退出码30、一个summary、零error；
低13、中67、高113、严重2。相对原205条仅消失上述十个共享 v4 claim key，没有新增。
v3 的五个同名 GHSA 匹配仍存在，全部历史输入保留；既有范围结论不是全部关闭。

私有补充证据保存于 artifacts/js-yaml-candidate，主要哈希：

| 证据 | SHA256 |
| --- | --- |
| 同一测试源码 | 552e3a7461aea186a00cb2bd5d09289202d3893b5957b2bfefb6e04021bc5214 |
| 修正设置后的旧版日志 | 1ee9bca86df4adf918734f2b698feb857692069a5d047c754684dae0a0c768b2 |
| 候选专项日志 | 88315ed15cefa155bdcb4b18408d0763765cbe9e5f8655c592805bd5fc21be09 |
| fresh候选审阅记录 | c80947ea86ac17fb8c2a6adddb5497948cfbfc0b9cd2785b2450cb250e403e0f |
| 审计原始输出 | 0fc21608a73d3ae4196e2b10e2e27085fb42575e386facc8483e230df1e55c85 |
| 压缩前payload | 0c3503983d3ba892e9f26944185bad640082438ac32a33906e800184e3866785 |

本批准确候选 CI、实际合并和合并后 main 验收待完成。回滚本批提交会恢复受影响旧库，
不能作为安全完成状态。四个受限审阅、Vite十二条、独立v3及最终发布门槛继续开放；
没有标签、镜像发布、生产部署或真实付费调用。

## 2026-10-01 #120合并验收

[PR #120](https://github.com/dreamvm/one-hub/pull/120)准确候选为
b7677242332f2dfc3d37a1b2b1638411a24c30c8，自己的十项检查全部成功：
[Compatibility36935607473](https://github.com/dreamvm/one-hub/actions/runs/36935607473)、
[Isolated36935608084](https://github.com/dreamvm/one-hub/actions/runs/36935608084)，均attempt1。
四项Compatibility真实checkout为合成160a3798861bb291101ada30bd4897f68e7a7f6c，
六项Isolated为准确候选；两项前端均新增20叶子/含父21项通过，实际js-yaml4.3.2。
每个原生架构89PASS、9轮并发、4种Compose；最终程序Go1.25.14、one-api、Linux/对应架构、CGO=1。

候选、合成、基于最新main计算及实际合并树均为049f64b6769a0d47a016f547675a72be0b4219fb。
实际合并6b1b3283b4322cda7d0f3e3df0257a7ff57df10f，父为dac105ff/b7677242。
[自身main CI36937929600](https://github.com/dreamvm/one-hub/actions/runs/36937929600)已成功，
准确push head为实际合并、attempt1；四项真实checkout均已核对。
前端仍为22文件/209 UI、ESLint无诊断、build通过，peer及chunk提示保留。

| 架构 | CI本地image SHA256 | 最终程序SHA256 |
| --- | --- | --- |
| amd64 | 781deb56b4346a477e5a22fe898c5612f382bfe979005d438ff6da509bae640b | 7d329a8da69a9c5f57785ba12589f4c5928b96403943a5586933814e4bf47c11 |
| arm64 | 495353263f18d5e56224dbf1c15c0661c8a9ec1eb028391a0ec1e7db72abfe40 | df8b84c07799992d13f23f7cfb0766bae13cff6f702e7f05b0d36f51c0509fb4 |

私有artifacts/pr120-ci保留原23文件、独立main清单和根代理合并树复核；
候选verification SHA为ff20038734eba073a02759f4ee8d1a71d333f81ecc97fbcf80ec281ba4b1bdec，
实际合并/main verification为3525f17ea2815713cb43e283d9ebd0f4859a664db0d99cbdf1ad9603a143bb4f。
本项共享v4修补已交付；独立v3、[Vite候选审阅缺口](VITE_MATCHER_CANDIDATE_REVIEW.md)、
四个受限草稿及最终RC8门槛仍开放，不称所有YAML或审计记录已关闭。
