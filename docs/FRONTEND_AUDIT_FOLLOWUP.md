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
