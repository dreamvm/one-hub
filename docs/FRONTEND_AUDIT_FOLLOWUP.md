# 当前前端审计与工具链调用范围

2026-10-01，审计输入为c193edc1；#116实际合并f2476a68与它内容树一致。
本次只增加记录，不安装或升级依赖，不运行翻译、模型或新的漏洞探针。

## 准确审计输入

固定Node22.20.0 / Yarn1.22.22，使用公开仓库的package.json/yarn.lock隔离副本。
两文件SHA256分别为c215bbf4d1917f07a2e5296a0ea1274ba6a29b94524755a1d7944d7726b1dc93、
ddd1deae31b2c374e3b9cf64654cebe0b565c63869a2c5b8c06258f7748a3ac6；审计前后未变。
没有用未合并Vite/Rollup分支的锁文件替代main。

按用户已有审计授权，先核实仓库公开性、Yarn源码及实际预检树；默认rc读取关闭，
执行环境不带认证/代理配置。实际送入gzip的JSON哈希与无网络预检一致，唯一POST接收端为
`registry.yarnpkg.com/-/npm/v1/security/audits`。字段为公开包名、版本、完整性哈希、
依赖关系、dev标志及空install/remove/metadata；没有源码、凭据、运行配置、内部地址、
下载镜像URL或package脚本。锁文件的公共镜像URL不属于审计发送字段，也未向它们请求。

新结果：**205条路径记录、98个Yarn advisory ID、72个GHSA**，一份成功汇总、无审计错误。
退出码30为公告严重性位掩码，不是执行故障，也不是205个已证实应用漏洞。
这与已有Monaco后快照的ID/包/路径/版本区间键一致，没有新增或删除这些匹配键；
快照时间和实际锁文件身份仍单独保留，不能仅凭旧文件名“current”推断当前状态。
[Yarn1审计说明](https://classic.yarnpkg.com/en/docs/cli/audit/)解释输出及退出码。

## 已有记录与剩余队列

205个输入逐条保留原来源、版本、依赖路径及缺口，没有删除重复路径。
其中103条已有范围记录：i18n75、Axios12、Router12、DOMPurify1和Rollup3；
“有记录”不表示均已修补、已交付或已关闭。

- [i18n记录](I18N_TOOLCHAIN_TRIAGE.md)保留历史54/21初筛、007后续反证及数组兼容失败；
  #110固定JSON维护范围的20条更新不覆盖可选模式或真实历史使用。
- [Axios记录](AXIOS_NEW_ADVISORIES.md)、[Router路径](FRONTEND_BROWSER_BOUNDARIES.md)和
  [Monaco记录](MONACO_RUNTIME_BOUNDARY.md)仍各自限定调用范围，库版本匹配保留。
- [Rollup草稿记录](https://github.com/dreamvm/one-hub/pull/68)缺失的独立审阅继续开放；#71的Vite map
  审阅也不因本次审计无该包匹配而关闭。两个受限审阅均未重试或改道。

此前102条缺少直接范围索引。本批仅完成下面六条静态核对；其余**96条**继续逐路径处理，
涉及ESLint配置/缓存与匹配器、Sass/Immutable、jsconfigPaths、Picomatch、Browserslist等。
这些未处理记录没有被自动赋予安全结论；生产事实和最终候选审计仍未完成。

## 六条输入的静态结果

沿用codex-security:triage-finding，在全部六条规范化后逐项内联核对。
无适用SECURITY.md；依据当前脚本、配置、实际依赖源码及维护产物，不假定本地文件可信。
已有frozen安装副本的两份依赖输入与本候选逐字一致。每行独立为当前维护入口内
`not_actionable`、高静态置信度，无修复排名；**库仍受影响，没有动态验证或库补丁。**

| 本批编号 / 审计记录 | 独立依赖路径 | 公告 | 当前缺少的必要前提 |
| --- | --- | --- | --- |
| 001 / 14 | eslint → @eslint/eslintrc → ajv6.12.6 | [2g4f-4pwh-qvx6](https://github.com/advisories/GHSA-2g4f-4pwh-qvx6) | 所有相关Ajv工厂消费者未启用$data；动态pattern分支需该构造选项 |
| 002 / 71 | vite → esbuild0.27.7 | [g7r4-m6w7-qqqr](https://github.com/advisories/GHSA-g7r4-m6w7-qqqr) | 当前Vite/插件调用transform/build/context rebuild，没有esbuild servedir服务 |
| 003 / 72 | vitest3.2.7 | [82fw-gwwq-j7x9](https://github.com/advisories/GHSA-82fw-gwwq-j7x9) | jsdom测试未启用Browser Mode，也未注册公开mocker/interceptor插件 |
| 004 / 73 | vitest → @vitest/mocker3.2.7 | [82fw-gwwq-j7x9](https://github.com/advisories/GHSA-82fw-gwwq-j7x9) | 同上；该路径单独保留，没有合并删除 |
| 005 / 163 | eslint-config-react-app → babel-preset-react-app → preset-env → SystemJS插件7.24.1 | [fv7c-fp4j-7gwp](https://github.com/advisories/GHSA-fv7c-fp4j-7gwp) | 当前解析/构建不选择SystemJS输出；auto只选择ESM保留或CommonJS |
| 006 / 197 | eslint → ajv6.12.6 | [2g4f-4pwh-qvx6](https://github.com/advisories/GHSA-2g4f-4pwh-qvx6) | ESLint自身工厂同样未启用$data，此独立路径也保留 |

### 必要源码反证

- ESLint和@eslint/eslintrc的验证工厂只接受其调用方固定选项；legacy/flat验证器无附加选项，
  RuleTester仅传strictDefaults。Ajv的definitions.def:118先要求it.opts.$data，pattern.jst才会
  使用数据产生RegExp。配置、schema或验证失败不能启用这个构造选项，不依赖输入可信。
- esbuild公告要求Windows的esbuild servedir服务器。Vite的scanner和optimizer使用
  rebuild/cancel/dispose；dev/start/build运行Vite及其preview。当前host=true事实保留，
  未以“默认localhost”作为控制，也没有关闭Vite独立文件边界或声称已完成Windows验收。
- @vitest/mocker的interceptor load仍可readFile(mock.redirect)，公开注册入口仍存在。
  当前Vite只注册React/jsconfigPaths；Vitest Node使用hoistMocksPlugin/automockPlugin，
  不是这个interceptor。jsdom配置没有Browser Mode；未以缺失peer、token或Origin推定安全。
  也没有把Node的vi.mock与公开WebSocket注册视为同一个入口。
- SystemJS插件仍为7.24.1。preset-env的auto分支只选false/commonjs，React-app预设没有
  modules=systemjs；实际ESLint为Babel解析器，package预设只有React，Vite react()无自定义
  Babel选项且关闭babelrc/configFile。此结论不靠源码可信，也不把core7.29.7当作该插件修补。

范围仅是当前维护的开发、构建、测试入口及嵌入式前端/Linux产物。
任意新增插件、直接库API、Browser Mode、SystemJS或esbuild --serve命令须重新分流。
没有运行测试、构建、应用、PoC或新安全验证；生产版本/配置、Windows和未来入口未验证。
逐输入结果、源码片段/哈希、审计来源和接收端证明保留为私有证据。

一次非作者、复用上下文且非fresh的独立只读证据审阅核对六条结果和全部审计来源，
指出压缩前JSON哈希、build警告表述和旧记录顺序三处问题；按已有证据修正。
没有新triage、动态测试或服务运行。准确候选CI及合并另记，静态判断不能替代交付步骤。

## 2026-10-01 #117合并验收

候选6901678f6204485c370e4879af452455ea24374c的
[Compatibility36921857761](https://github.com/dreamvm/one-hub/actions/runs/36921857761)和
[Isolated36921858915](https://github.com/dreamvm/one-hub/actions/runs/36921858915)原始运行均成功，
十项检查通过。前者四项真实checkout合成609dfc1d，后者六项真实checkout候选。
候选、合成609dfc1db7463cd087a425c927a1e028243ba59a、最新main f2476a68计算及
实际合并2cb47018382ae9f086ec7471580c04b0c2505a4d的树均为
bf920560721ef979721fb806375b8f117772e618。

两个前端作业均22文件209UI通过，ESLint无诊断；build成功且>500kB chunk警告保留。
两个native作业各89PASS、九轮并发、四种Compose；最终程序为Go1.25.14、one-api、
Linux对应架构、CGO1，来自CI本地加载image，未发布。

| 架构 / job | image SHA256 | 最终程序SHA256 |
| --- | --- | --- |
| amd64 / 110571367470 | 204a2e42f1365e6e23a1005cb7c37b65c64909b3e972e90358b5adfbe510a3a5 | 8e3f56bdd0dd5f8248586458e87351c64f1f07a6377c7b0f761ed988e38cd0e9 |
| arm64 / 110571367350 | 5477c3059e1179f8c55a243f1610cabe23fbb9b80d81196dd2356389f7c8c249 | 2e5a5351712d258ae66111674d315101974224afed015fc828bfdf75940f3204 |

十份日志及真实checkout/树/程序身份记录保存为私有证据；verification.json SHA256为
21fe449622001a4464edef8338059de91784e6791fd81a3c70a0161d1d8900a8，SHA清单为
b46dcf4e92a52c33f617a8f041a4a981a4fe762d0b1be8444456a14db6dfd69f。
[合并后main CI36924792593](https://github.com/dreamvm/one-hub/actions/runs/36924792593)
已成功，准确head为实际合并提交2cb47018。

后续[21条可选工具路径](FRONTEND_OPTIONAL_TOOL_PATHS.md)完成静态核对，剩余队列由96变75；
这是后续独立批次，不改写#117初次六条的范围，也不称库已修补。

## 2026-10-01 最后75条静态核对

[匹配器及YAML路径](FRONTEND_MATCHER_YAML_BOUNDARIES.md)保留全部75条原输入，
完成内联静态核对：53条受检入口not_actionable，22条needs_review，零条确认项目漏洞。
剩余未初筛队列归零不等于问题关闭；实际js-yaml配置解析10条和Vite matcher12条继续核实，
此前103条索引也保留各自的未修补、受限审阅或范围缺口。受影响库版本未变。

## 2026-10-01 共享YAML v4候选复查

[共享配置边界候选](JS_YAML_CONFIG_BOUNDARY.md)仅更新js-yaml4.1.0为4.3.2，
原版本的13个安全叶子断言失败、7个正常对照通过，候选20叶子/Node21全通过。
一次fresh独立源码审阅未发现共享v4具体绕过/回归，但确认独立v3的同类既存残留；
完整前端回归、lint、build通过，候选CI和合并尚待完成。

同一候选依赖文件的官方Yarn复查为195路径/93Yarn ID/72GHSA，退出30、零error；
仅原十个共享v4 claim key消失，没有新增。v3的五个同名GHSA仍匹配，原205输入全部保留。
新包名集合不变，发送前JSON哈希与无网络预检一致，接收端和字段按既有授权核对；
复查不发送源码/凭据/内部地址，不等于最终发布审计或整体清零。
