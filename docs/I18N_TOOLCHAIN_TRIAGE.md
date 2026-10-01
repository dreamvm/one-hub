# i18n 工具链首批静态核对

受检源码为 main `4603ed139c3be1353e99c7260b70dd97a314670b` 的 i18n 路径；
审计快照来自 Vite 候选 `9c90385c665440e1016e6b9eb4c970b626763d46`，该候选未改变
i18n 依赖或调用。这里只处理以下 19 条路径，不将整个依赖树的匹配自动视为应用漏洞。
其余 i18n 公告仍待核实。全部结论为静态判断，中等置信度，没有运行翻译 CLI、模型、
漏洞探针或真实服务，也没有读取凭据。

## 实际入口与边界

`web/package.json` 的 `yarn i18n` 调用 `@lobehub/i18n-cli` 1.20.3；普通 `yarn build`
只运行 Vite。`.i18nrc.js` 指定本地 zh_CN JSON 到 en_US/ja_JP/zh_HK，使用 JSON 模式。
浏览器 `src/i18n/resources.js` 仅静态导入四个语言文件，不导入 CLI/LangChain。
Docker 最终阶段仅复制 Go 程序，前端构建经 `main.go` 嵌入，不运行 Node 翻译工具。

未发现适用的 SECURITY.md；上述包、配置及部署源码只支持当前受检入口的范围判断。
“本地文件”不自动等于可信输入，模型响应和以前生成的语言文件也不能假定可信。
依赖仍有缺陷，未来新增调用或更改工作流时需重新判断。

## 保留的逐路径结果

下表以 `CLI` 表示 `@lobehub/i18n-cli`，`core` 表示 `@langchain/core`，
`openai` 表示 `@langchain/openai`；每行对应独立审计输入，没有合并删除重复路径。
`not_actionable` 仅表示当前调用缺少该公告必要前提，`needs_review` 表示证据不足。

| 编号 | 依赖路径 | GHSA | 静态结果 |
|---|---|---|---|
| 001 | CLI → langchain | r399-636x-v7f6 | not_actionable |
| 002 | CLI → core | r399-636x-v7f6 | not_actionable |
| 003 | CLI → openai → core | r399-636x-v7f6 | not_actionable |
| 004 | CLI → langchain → openai → core | r399-636x-v7f6 | not_actionable |
| 005 | CLI → lodash-es | r5fr-rjxr-66jc | not_actionable |
| 006 | CLI → lodash-es | f23m-r3pf-42rh | not_actionable |
| 007 | CLI → lodash-es | xxjr-mmjv-4gpg | needs_review，队列第 1 项 |
| 008 | CLI → @lobehub/cli-ui → ink → lodash | r5fr-rjxr-66jc | not_actionable |
| 009 | CLI → @lobehub/cli-ui → ink → lodash | f23m-r3pf-42rh | not_actionable |
| 010 | CLI → @lobehub/cli-ui → ink → lodash | xxjr-mmjv-4gpg | not_actionable |
| 011 | CLI → core → langsmith | fw9q-39r9-c252 | not_actionable |
| 012 | CLI → openai → core → langsmith | fw9q-39r9-c252 | not_actionable |
| 013 | CLI → langchain → openai → core → langsmith | fw9q-39r9-c252 | not_actionable |
| 014 | CLI → core → langsmith | rr7j-v2q5-chgv | not_actionable |
| 015 | CLI → openai → core → langsmith | rr7j-v2q5-chgv | not_actionable |
| 016 | CLI → langchain → openai → core → langsmith | rr7j-v2q5-chgv | not_actionable |
| 017 | CLI → core → langsmith | 3644-q5cj-c5c7 | not_actionable |
| 018 | CLI → openai → core → langsmith | 3644-q5cj-c5c7 | not_actionable |
| 019 | CLI → langchain → openai → core → langsmith | 3644-q5cj-c5c7 | not_actionable |

## 依据、反证与缺口

- **001–004**：[LangChain 公告](https://github.com/langchain-ai/langchainjs/security/advisories/GHSA-r399-636x-v7f6)
  所需的 `load()` 反序列化没有连接到当前调用。CLI 的 `dist/cli.js` 使用
  ChatPromptTemplate/ChatOpenAI 后，以 JSON.parse/dirty-json 解析文本并合并保存。
  受检 model、prompt、text splitter、callback/tracer 路径没有调用该 load；未启用缓存，
  可选内存缓存也不调用它。存在受影响 reviver 不等于当前翻译流程会执行该 reviver。
- **005–006、008–010**：CLI 的 lodash-es imports 不含 template/omit；just-diff 6.0.2
  的路径来自 Object.keys 字符串和数组数字下标，没有自定义 path converter，不能产生
  f23m 公告要求的嵌套数组路径成员。Ink 受检构建仅导入 lodash/throttle.js。
  这些结论不覆盖任意新脚本直接使用库 API。
- **007**：CLI 差异函数确实将 remove 操作的路径传给 `unset(cloneDeep(target), path)`。
  尚未证明路径会进入内建原型而非克隆对象自己的属性；也未确定低信任语言文件进入维护
  工作流的边界。下一步是仅对合成语言文件执行实际差异函数的有界对照，不调用真实模型。
  不能仅由函数名或版本匹配宣布应用可利用，也不能在缺少证据时关闭此项。
- **011–013**：当前 CLI 不调用 createAnonymizer，也不提供 anonymizer 函数。
  LangChainTracer 构造默认 Client，环境变量可提供布尔 hideOutputs，但不提供该函数。
- **014–016**：CLI 使用 call/invoke 且不设置 streaming；ChatOpenAI 默认 false。
  BaseChatModel 的隐式流式分支要求 streamEvents/streamLog handler，CLI 没有提供；
  默认 LangChainTracer 不是这两类 handler。没有假定环境不会开启普通 tracing。
- **017–019**：提示词直接由固定 ChatPromptTemplate.fromMessages 构造，没有 Hub 的
  pullPrompt/pullPromptCommit。受检 tracer 只持久化 run，不拉取提示配置。

LangSmith 的自动 tracing 仍可由环境启用；本项没有读取实际环境或授权任何 tracing 外发。
SDK 的其余公告、Markdown/目录 glob 模式及工具更新兼容性仍开放。npm 官方当前工具版
1.27.0 已移除 LangChain，但引入 React 19/Ink 6 等依赖，未在此项升级或声称兼容性通过。
逐输入规范化、源码证据、反证、缺口及排名保留在私有 triage JSON 中。

本项没有修复提交，因此不以静态分流代替未来补丁的旧新验证、独立审阅或准确 CI。

## 后续有界对照与其余 56 条静态记录（2026-09-30）

以下为首批记录之后的新证据，不覆盖前面的历史判断。基线为 main `5f0a7fc8`，
i18n 依赖与 `4603ed13` 相同。未升级翻译工具或执行真实模型。

首批 007 的全新只读调查未找到普通 JSON 语言文件到内建原型删除的路径。
父任务提取已安装 CLI 的原样纯 diff 函数，不启动 CLI、dotenv 或网络：7 组合成输入
保持测试自建标记及原输入，普通对象增删对照通过；直接 lodash 控制能删除同一自建标记，
说明库本身的缺陷与本项目调用可达性不同。只对 JSON 来源、未预先改写的标准原型作此判断，
不扩展为库安全结论，也未增加推测性补丁。

**数组正常对照失败**：源数组缩短到一个元素时，译文结果为 `[null, "two"]`，
而非预期 `["one"]`。旧工具使用 delete 而非 splice；失败日志单独保留，不能用后续驱动
退出码 0 代表全部正常对照通过。四份当前语言文件均没有数组值，独立调查也确认这一点；
数组兼容性仍开放，没有顺带改写旧行为。

其余 56 条输入逐条保留如下，静态结果为 36 条当前调用不适用、20 条证据不足。
与首批合计 75 条初筛记录：54 条 not_actionable、21 条 needs_review；007 的后续证据
另记如上，未悄悄改写初筛结果。重复依赖路径没有删除。

| 编号 | GHSA | 包 | 静态结果 | 待核实排名 |
|---|---|---|---|---|
| 020 | w5hq-g745-h8pq | uuid | not_actionable | — |
| 021 | w5hq-g745-h8pq | uuid | not_actionable | — |
| 022 | w5hq-g745-h8pq | uuid | not_actionable | — |
| 023 | w5hq-g745-h8pq | uuid | not_actionable | — |
| 024 | v6h2-p8h4-qcjw | brace-expansion | needs_review | 10 |
| 025 | f886-m6hf-6m8v | brace-expansion | needs_review | 11 |
| 026 | 3jxr-9vmj-r5cp | brace-expansion | needs_review | 12 |
| 027 | mh99-v99m-4gvg | brace-expansion | needs_review | 13 |
| 028 | rgw5-rvv9-x895 | brace-expansion | needs_review | 14 |
| 029 | q2hr-2g5m-vwhr | brace-expansion | needs_review | 15 |
| 030 | qhr7-859c-m2p7 | brace-expansion | needs_review | 16 |
| 031 | 6j4f-fj2g-mc7p | brace-expansion | needs_review | 17 |
| 032 | fjxv-7rqg-78g4 | form-data | not_actionable | — |
| 033 | fjxv-7rqg-78g4 | form-data | not_actionable | — |
| 034 | hmw2-7cc7-3qxx | form-data | not_actionable | — |
| 035 | hmw2-7cc7-3qxx | form-data | not_actionable | — |
| 036 | 73rr-hh4g-fpgx | diff | not_actionable | — |
| 037 | 73rr-hh4g-fpgx | diff | not_actionable | — |
| 038 | 73rr-hh4g-fpgx | diff | not_actionable | — |
| 039 | 73rr-hh4g-fpgx | diff | not_actionable | — |
| 040 | 73rr-hh4g-fpgx | diff | not_actionable | — |
| 041 | 73rr-hh4g-fpgx | diff | not_actionable | — |
| 042 | 2g4f-4pwh-qvx6 | ajv | not_actionable | — |
| 043 | 2g4f-4pwh-qvx6 | ajv | not_actionable | — |
| 044 | 48c2-rrv3-qjmp | yaml | not_actionable | — |
| 045 | 58qx-3vcg-4xpx | ws | needs_review | 6 |
| 046 | 58qx-3vcg-4xpx | ws | needs_review | 7 |
| 047 | 96hv-2xvq-fx4p | ws | needs_review | 8 |
| 048 | 96hv-2xvq-fx4p | ws | needs_review | 9 |
| 049 | v2hh-gcrm-f6hx | fast-uri | not_actionable | — |
| 050 | v2hh-gcrm-f6hx | fast-uri | not_actionable | — |
| 051 | 7p8r-x3mc-p8w7 | fast-uri | not_actionable | — |
| 052 | 7p8r-x3mc-p8w7 | fast-uri | not_actionable | — |
| 053 | q3j6-qgpj-74h6 | fast-uri | not_actionable | — |
| 054 | q3j6-qgpj-74h6 | fast-uri | not_actionable | — |
| 055 | v39h-62p7-jpjc | fast-uri | not_actionable | — |
| 056 | v39h-62p7-jpjc | fast-uri | not_actionable | — |
| 057 | f65p-4m7j-42xc | fast-uri | not_actionable | — |
| 058 | f65p-4m7j-42xc | fast-uri | not_actionable | — |
| 059 | jqff-g426-hqxp | fast-uri | not_actionable | — |
| 060 | jqff-g426-hqxp | fast-uri | not_actionable | — |
| 061 | 4c8g-83qw-93j6 | fast-uri | not_actionable | — |
| 062 | 4c8g-83qw-93j6 | fast-uri | not_actionable | — |
| 063 | qw65-cvwx-89v3 | fast-uri | not_actionable | — |
| 064 | qw65-cvwx-89v3 | fast-uri | not_actionable | — |
| 065 | hrr3-gc8f-f4qj | fast-uri | not_actionable | — |
| 066 | hrr3-gc8f-f4qj | fast-uri | not_actionable | — |
| 067 | 5j98-mcp5-4vw2 | glob | not_actionable | — |
| 068 | mh29-5h37-fv8m | js-yaml | needs_review | 1 |
| 069 | h67p-54hq-rp68 | js-yaml | needs_review | 2 |
| 070 | 52cp-r559-cp3m | js-yaml | needs_review | 3 |
| 071 | 5p4m-2wfm-xmqj | js-yaml | needs_review | 4 |
| 072 | 2883-xcg3-v3hh | js-yaml | needs_review | 5 |
| 073 | 3ppc-4f35-3m26 | minimatch | needs_review | 18 |
| 074 | 7r86-cg39-jmmj | minimatch | needs_review | 19 |
| 075 | 23c5-xmqv-rm74 | minimatch | needs_review | 20 |

- **020–023 UUID**：当前 core/LangSmith 回调使用 v4；indexing 中的 v5 只传值与 namespace，
  不传外部输出缓冲区，缺少公告必要前提。
- **032–035 form-data**：依赖来自 OpenAI 的 @types/node-fetch 声明；运行时使用
  formdata-node/form-data-encoder，翻译请求为 JSON，并未使用受影响的 form-data 编码器。
- **036–041 diff**：uvu 错误格式化只调用 diffArrays/diffLines/diffChars，包含 development
  条件导出；没有调用公告要求的 parsePatch/applyPatch。
- **042–043 Ajv**：conf 固定 schema 只有 apiBaseUrl/openaiToken 字符串，未启用 $data，
  不接受动态 pattern。**049–066 fast-uri** 仅解析这套固定 schema 的引用，未连到
  请求目的地、文件访问或异步 loadSchema，不能把库 URL 差异直接推断为项目 SSRF。
- **044 yaml**：LangChain 的 YAML config loader 不在 CLI 的 text_splitter 导入链；
  Markdown 使用的是另一包 js-yaml，后者单独列为待核实。
- **067 glob**：CLI 使用 globSync 库，不调用 glob 可执行文件的 --cmd/shell 分支。
  此结论不关闭下面的 glob 模式复杂度公告。
- **068–072 js-yaml**：工具支持 md/--with-md，输入前言会进入 gray-matter.safeLoad；
  当前项目配置仅用 JSON。仍需明确 Markdown 来源及支持的信任边界和受影响 merge/omap
  语义，不能仅凭默认模式关闭可选模式。
- **045–048 ws**：Ink 在 DEV=true 时尝试载入 react-devtools-core，且仅在没有全局
  WebSocket 时使用 ws。当前安装树没有 react-devtools-core，但它是 Ink 声明的 peer；
  支持的调试环境、对端归属和 TypedArray close reason 路径仍需核实，没有读取真实环境。
- **024–031、073–075**：目录/Markdown 模式能将自定义配置模式交给 glob/minimatch。
  是否允许较低信任来源提供模式仍不明确；未做真实耗尽、长时间阻塞或外部目标验证。

详细逐路径来源、源码行号、反证及未决事实保留在 triage JSON。此静态阶段没有运行漏洞
探针、翻译任务或模型，不以它替代未来补丁的旧新对照、独立审阅和准确候选 CI。

## 2026-10-01 固定 JSON 维护范围

维护者委托选择方案后，按[维护范围](I18N_MAINTENANCE_SCOPE.md)保留现有单文件 JSON，
固定配置并显式 `DEV=false`；Markdown、目录 glob、自定义配置和 DEV 调试在完成各自整改前不启用。
这是新的支持约定，不能倒推历史使用情况；旧依赖及可选入口仍然存在，没有技术封禁或升级。

在该范围下，以下每个原始输入分别为 `not_actionable`、中等置信度，无待核实队列排名。
原始75条及历史54/21初筛记录保留不改；这里更新20条可选路径的项目适用性，不宣称审计清零。

| 输入编号 | 各自公告 | 当前入口缺少的必要路径 |
|---|---|---|
| 024、025、026、027、028、029、030、031 | 依次为 v6h2-p8h4-qcjw、f886-m6hf-6m8v、3jxr-9vmj-r5cp、mh99-v99m-4gvg、rgw5-rvv9-x895、q2hr-2g5m-vwhr、qhr7-859c-m2p7、6j4f-fj2g-mc7p | 固定 zh_CN.json 进入 genFlatQuery，内容不成为 brace-expansion 模式 |
| 045、046 | 各自依赖路径上的 58qx-3vcg-4xpx | DEV=false 跳过 Ink devtools 导入；不靠缺失 peer 或全局 WebSocket 推断 |
| 047、048 | 各自依赖路径上的 96hv-2xvq-fx4p | 同上，无该调试 ws 连接路径 |
| 068 | mh29-5h37-fv8m | JSON 内容不进入 gray-matter/YAML |
| 069 | h67p-54hq-rp68 | 同上 |
| 070 | 52cp-r559-cp3m | 同上 |
| 071 | 5p4m-2wfm-xmqj | 同上 |
| 072 | 2883-xcg3-v3hh | 同上 |
| 073、074、075 | 依次为 3ppc-4f35-3m26、7r86-cg39-jmmj、23c5-xmqv-rm74 | 固定 JSON 不调用目录/Markdown glob，不把语言文本作为 minimatch 模式 |

上述表格仅压缩显示；私有 `i18n-supported-json-scope-triage-20261001.json` 保留20个独立输入、
路径、来源、证据、反证及缺口，没有合并或删除重复输入。受检代码为 abd750d4，相关依赖与早期基线相同。

新增源码证据确认：Markdown `genFilesQuery` 在翻译请求前解析前言，`includeMatter` 不会跳过解析；
目标文件已存在时只跳过该输出，不是通用安全控制。gray-matter 的 safeLoad 仍启用 merge/omap，
js-yaml 3.14.1 的重复合并、空来源遍历及 omap 线性去重机制均存在；异常捕获不能中断同步解析。
原型公告涉及解析结果的原型，未证明本 CLI 的安全敏感继承属性消费，不扩大为全局原型修改或代码执行。

官方公告分别列出修复线：[结果原型3.14.2](https://github.com/advisories/GHSA-mh29-5h37-fv8m)、
[重复别名3.15.0](https://github.com/advisories/GHSA-h67p-54hq-rp68)、
[合并链3.15.0](https://github.com/advisories/GHSA-52cp-r559-cp3m)、
[omap3.15.1](https://github.com/advisories/GHSA-5p4m-2wfm-xmqj)、
[空合并来源3.15.2](https://github.com/advisories/GHSA-2883-xcg3-v3hh)。
这里只核对官方修复依据，没有安装升级、执行漏洞探针、翻译请求或宣布兼容通过。

007 的既有 JSON 对照及数组失败限制继续保留。未来配置、模式或环境改变须重新分流；
真实历史使用情况和任何曾处理的文件仍未核实。
