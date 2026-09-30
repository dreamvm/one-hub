# Babel 开发依赖的 source map 边界

## 范围

本项对应 [GHSA-4x5r-pxfx-6jf8](https://github.com/babel/babel/security/advisories/GHSA-4x5r-pxfx-6jf8)。
旧顶层 `@babel/core` 7.24.3 在编译带外部 source map 注释的代码时，会读取包根之外的文件；
有效 map 的 `sourcesContent` 可进入输出 map。非 JSON 文件也可能被读取，但解析失败，
不能把这个结果描述为任意文本均被返回。公告需要输入源码可控、输出可读等前提。

当前 ESLint 的 `@babel/eslint-parser` 通过 `parseSync` 解析 JSX，不经过该编译器读取路径。
Vite 的 `react()` 没有自定义 Babel options：生产模式跳过 Babel，开发 FastRefresh
使用插件解析到的另一份 7.29.7。未发现业务 API 将用户代码送入 Babel 的路径；本项
修复开发依赖的已复现缺陷，不表示已证实生产接口可远程利用。

## 最小修复与兼容性

将直接开发依赖固定为 7.29.7，并将锁文件中 `^7.16.0`、`^7.28.0` 的 core selectors
统一到已有 7.29.7 条目及其现有依赖闭包。覆盖直接依赖、react-app 配置/预设和 Vite
插件的解析，不添加 resolutions、不整体升级 Babel 插件或其他工具。

上游从 7.29.6 起限制外部注释 map 的词法路径：以输入文件最近的 package 根目录为界，
缺少 package manifest 时使用配置的 root。它允许同目录及仍处于同一 package 内的父目录
map，保留 inline map 和显式 `inputSourceMap` 对象。`sourceMaps:false` 仅控制输出，
不能代替输入读取控制；`inputSourceMap:false` 才禁用输入 map。没有全局禁用合法 map。

## 回归与证明范围

`web/tests/babel-file-boundary.test.mjs` 明确解析直接 Babel 和 plugin-react 的实际 copy，
在导入前观察测试自己创建的文件读取，保留真实文件系统和编译逻辑。临时文件均为合成
内容；不加载项目 dotenv、不监听端口、不请求真实上游，结束后恢复观察器并清理夹具。

28 个叶子检查已接入 `yarn test:deps`：

- 六种同步/异步 code、AST、file API 的包外拒绝与同目录正常 map，共 12 项。
- 绝对路径、归一化路径与 block comment、非 JSON、最近 package、无 manifest 的 root、关闭输出 map，共 6 项。
- 包内父目录、inline、显式对象、关闭输入 map、无 filename、缺失/无效 map、直接 parseSync JSX、实际 ESLint parser，以及插件 copy 的拒绝/正常对照，共 10 项。

7.24.3 旧版：12 个边界失败，16 个控制通过；六种 API 均观察到包外读取和合成 map
内容进入输出。7.29.7 候选：28 个叶子通过，原读取和输出内容不再出现（Node 总数
含父节点为 29）。正常 map 的原始内容、JSX AST、编译结果和错误 map 的回退均保留。

本地 Node 22.20.0 / Yarn 1.22.22 frozen 安装、30 个 Vite、20 个 Axios、16 个 PostCSS、
28 个 Babel 叶子及 60 个 Vitest 检查、lint、生产 build、工作流策略检查通过。Lint
仍有 9 条既存 warning，build 仍有大 chunk 提示。首次沙箱内离线安装因无法使用原有
缓存、缺少已有依赖缓存条目而失败；使用原有缓存的离线 frozen 安装随后成功，未改锁文件。

新审计已无 `@babel/core` 匹配；快照为 217 条路径记录、110 个 Yarn ID、84 个唯一 GHSA。
相较 PostCSS 后旧快照，Babel 公告不再匹配，但新增 7 个 Axios 公告匹配当前 1.18.0，
这些新公告尚未完成业务适用性调查，单独保留。审计路径数与应用可利用性不是同一指标，
旧 Axios 验收对应旧公告集，不能代替本次新增项的判断。

## 未关闭项与回滚

独立候选审阅和主执行者的合成对照确认：上游路径检查不调用 realpath，包内符号链接
仍可指向包外 map 并合并其内容，因此本项只修复词法路径边界，不构成文件系统沙箱。
另有低影响兼容性限制：包根内以 `..` 开头的合法 map 文件名会被误拒绝，编译仍成功
但丢弃输入映射；当前仓库无此类 map 或业务消费者。普通相对/绝对包内 map 保持正常。
原生 Windows 未验证。Vite 自己的 map 加载器、其他 Babel 插件、Monaco/DOMPurify、
新 Axios 公告、Rollup 缺失审阅和其余发布条件保持独立。浏览器、arm64 实际镜像与生产
端到端没有由本项重新验收。API、数据库与运行时配置不变。

独立调查因新线程额度耗尽使用复用线程，已明确记录；接续后的候选由全新只读审阅者
审查代码/测试 diff，未预先提供调查结论、修复理由或通过声明。审阅未发现项目交付阻断，
上述限制保留；审阅者独立重跑 28 个叶子检查通过。准确 CI 与合并状态另记于
NEXT_RELEASE.md，不由本地通过推断完成。RC8 仍预留，未打标签、发布或部署。
回滚采用本项提交的反向提交并重新执行 frozen 安装与回归；回滚会恢复旧依赖风险。
