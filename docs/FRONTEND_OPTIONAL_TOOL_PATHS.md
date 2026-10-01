# Immutable 与可选工具链路径

2026-10-01，接续[准确前端审计](FRONTEND_AUDIT_FOLLOWUP.md)，审计输入与
#117候选6901678f、实际合并2cb47018的web依赖文件逐字一致。
205条匹配及受影响版本保留；这份记录不安装或升级依赖。

## 输入与方法

原96条未处理输入中，本批分两次静态阶段完成6条Immutable和15条可选工具输入。
每批先规范化全部输入，再逐项内联核对；没有把重复依赖路径合并删除。
沿用codex-security:triage-finding，阶段内没有测试、构建、服务、PoC或仓库修改。
无适用web SECURITY.md；依据当前维护脚本、配置优先级、实际安装源码和产物范围，
没有用“本地文件可信”或“开发依赖”标签代替调用条件。

21条各自为受检入口内`not_actionable`、高静态置信度，无修复排名。
**库版本仍受影响，这不是库补丁、动态验证或任意库API的安全结论。**
其余**75条**保留未核对状态：brace-expansion32、js-yaml10、minimatch12、
picomatch20、yaml1；已有103条范围记录也不表示全部已修补或关闭。

## Immutable：编译器实际存在

Immutable4.3.5、Sass1.72.0，Sass依赖`immutable ^4.0.0`。
index.jsx、themes/index.js、password-strength.js实际导入SCSS；仓库有三份SCSS，
不能因没有直接导入immutable而忽略Sass。Vite7.3.5未找到sass-embedded，选择sass；
SCSS worker用compileStringAsync和返回文本的内部importer。

| 原审计记录 / 独立路径 | 公告 | 必要条件与当前反证 |
| --- | --- | --- |
| 153 immutable；154 sass → immutable | [wf6x-7x77-mvgw](https://github.com/advisories/GHSA-wf6x-7x77-mvgw) | 需危险合并或转成普通对象；Sass没有mergeDeep/toJS/toObject调用，map桥接返回OrderedMap，应用也没有相关消费者。污染影响返回对象原型，不能写成全局Object.prototype污染 |
| 155 immutable；156 sass → immutable | [xvcm-6775-5m9r](https://github.com/advisories/GHSA-xvcm-6775-5m9r) | OrderedMap的set会使用Immutable Map，桥接本身不能视为免疫；仅两个JS值getter调用转换，当前无自定义Sass函数消费它们，默认解析/求值使用Dart集合 |
| 157 immutable；158 sass → immutable | [v56q-mh7h-f735](https://github.com/advisories/GHSA-v56q-mh7h-f735) | 需大外部索引/size/path进入List变更；JS值getter从现有数组构造List，Sass没有相关setSize/setIn/updateIn或外部索引List.set；SassList.get是有界Dart数组读取 |

全部19个self.immutable调用点及转换调用方已核对。dartMapToImmutableMap仅由
SassArgumentList.keywords与SassMap.contents的公开JS getter调用。
当前Vite配置没有Sass functions/preprocessorOptions；_parseFunctions0(null)返回空回调列表。
这不是把任意SCSS视为可信，也不声称JS自定义函数、任意Sass值接口或未来配置安全。
没有执行大索引、碰撞负载、无限循环或OOM复现。

## 15条可选工具输入

| 原审计记录 / 独立路径数 | 公告 | 当前缺少的必要条件 |
| --- | --- | --- |
| 106 / 1：vite-jsconfig-paths → recrawl-sync → sucrase → glob10.4.5 | [5j98-mcp5-4vw2](https://github.com/advisories/GHSA-5j98-mcp5-4vw2) | glob CLI必须带-c/--cmd才走shell:true；当前插件调用recrawl API，其运行文件不导入Sucrase CLI；Sucrase自身CLI也调用glob API，维护脚本未启动glob命令分支 |
| 126、127 / 各1：eslint → file-entry-cache → flat-cache → flatted3.3.1 | [25h7-pfq9-p65f](https://github.com/advisories/GHSA-25h7-pfq9-p65f)、[rf6f-7fwh-wjgh](https://github.com/advisories/GHSA-rf6f-7fwh-wjgh) | lint/lint:fix无--cache；默认cache=false，只有options.cache时才构造LintResultCache并进入flatted.parse。未假定缓存文件可信 |
| 144、145 / 各1：flowtype → lodash4.17.21及react-app → flowtype → lodash | [r5fr-rjxr-66jc](https://github.com/advisories/GHSA-r5fr-rjxr-66jc) | 全部78份flowtype dist JS没有template调用；维护配置也未选择flowtype |
| 147、148；150、151 / 各1：同上两条lodash路径 | [f23m-r3pf-42rh](https://github.com/advisories/GHSA-f23m-r3pf-42rh)、[xxjr-mmjv-4gpg](https://github.com/advisories/GHSA-xxjr-mmjv-4gpg) | 没有unset/omit或间接lodash default消费者；字符串与数组路径公告分别保留。这不改变i18n路径或受限审阅 |
| 182、183、184 / 各1：react-app经env/classes/runtime-polyfill到旧Browserslist4.23.0 | [c83g-rgw3-j3cx](https://github.com/advisories/GHSA-c83g-rgw3-j3cx) | 当前未选择这些旧preset调用方；不是以单次查询或开发用途推定长期缓存无风险 |
| 185、186、187 / 各1：上述三条旧Browserslist路径 | [73wf-gq98-2v4g](https://github.com/advisories/GHSA-73wf-gq98-2v4g) | 同上；旧getStat无条件读父目录stats，缺少my-stats查询不能作安全控制 |

package.json的eslintConfig确实extends react-app，不能漏掉。实际ConfigArrayFactory
在同目录优先选择.eslintrc并返回首个配置；web/.eslintrc的root:true停止上溯，
当前插件/extends不含react-app或flowtype，受检源码目录没有其他已跟踪ESLint配置。
Flowtype的15种lodash调用均非上述危险API，未因插件未启用而省略消费者核对。

当前Babel core7.29.7解析到自己的helper-compilation-targets7.29.7和Browserslist4.29.3，
与受影响的根目录helper7.23.6/Browserslist4.23.0分开记录。package只设React preset，
Vite没有自定义env/polyfill preset；旧core-js-compat及preset图的调用方未选入当前维护配置。
固定查询或缺少仓库stats文件不能证明旧normalizeStats安全，父目录/环境数据也未实查。

## 证据与交付限制

两批逐输入结果、原始来源、实际安装路径、源码片段与哈希保存为私有补充证据：

- immutable-normalized-inputs / static-evidence / triage：
  ca2f4d665533dc8e36392268a2dac3d0fd76b1dcb2d2c799d5304cee1ad31457 / 79822bbcb129ac632888752f8f353d4b005df90729a259cd38ccba4b851fe558 / 2a44fdd88b6a94af96577c86f78211039661322032229cbc4d76d1d9f9490fc1。
- optional-tool-inputs-normalized / installed-resolutions / static-evidence / triage：
  b10847b47a098f032d5ece53fc22692b8cc5c68df320815b9c96dfdc8ee1675b / 4b6721254b684e46c0c163de4c954ab11c8bc152344e86a1a7a947667929bafd / 9d2a5ba640d294131e822d41695f153dca2dc0e3832c510c355d93b10bed5146 / 3f5cc69bcd59070840aebe3931a1918cbd60bc134bdcd377fc5e90fa46560ca2。
- pending-75-records：d7f0bdddde89135476173adac98c49c57a6c0fcc2b0a15c8f1b8f5d250cd87b4。

新增直接API、缓存、CLI命令选项、自定义Sass函数或旧preset须重新评估。
任意库用法、生产版本/配置、最终候选全面审计仍未验证；本批独立证据审阅、
准确候选CI和合并另记。四个受限审阅未重试，没有发布、部署或真实付费调用。

一次非作者、复用上下文且非fresh的独立只读文档证据审阅已完成，无具体发现。
审阅核对21/75计数、逐输入证据、Sass桥接、配置优先级、安装副本哈希及#117验收，
没有新triage、测试、构建或受限审阅；不把文档一致性当作库修补或动态验证。
本批自己的准确候选CI及合并尚待完成。
