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
