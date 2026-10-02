# Rollup 构建产物输出边界

## 问题与作用范围

[GHSA-mw96-cpmx-2vgc](https://github.com/rollup/rollup/security/advisories/GHSA-mw96-cpmx-2vgc)
影响 Rollup 4.59.0 之前的 4.x：受控输出名称可能越出输出目录。
项目顶层 Vite 7.3.5 和测试工具内 Vite 7.3.6 均以 `rollup@^4.43.0`
解析到 4.53.3。实际 `yarn build` 经 Vite 调用 Rollup 及写盘接口，Docker
和 CI 使用该路径。此依赖在构建时运行，不是最终 Go 服务的浏览器业务代码。

项目配置使用默认本地 `index.html` 及 assets 命名，未配置命名 input、
manualChunks 或自定义输出名称。源码调查未找到业务 API 输入控制这些名称的
入口；合成插件验证的是工具库的前提，不能称为已确认的业务攻击链。

## 最小候选与回归

只将现有锁定 Rollup 4.53.3 更新为 4.59.0，并同步已发布包 metadata 要求的
25 个 `@rollup/rollup-*` 原生可选包（旧版为 22 个）；其余 fsevents、
`@types/estree` 保持现有满足范围的条目。不新增直接依赖或升级 Vite。
修复在最终输出阶段同时检查 bundle key 和实际 entry.fileName，而非仅检查
单个输入参数；错误码为 `FILE_NAME_OUTSIDE_OUTPUT_DIRECTORY`。

`web/tests/rollup-output-boundary.test.mjs` 从顶层 Vite 解析实际 Rollup，
使用虚拟模块、临时目录及无敏感内容的合成资产，通过真实 `generate()` 和
`write()` 分别验证以下路径：

- 命名 input、对象/函数两种 manualChunks、显式 asset 名称、修改后的
  entry.fileName、与 entry.fileName 不一致的 bundle key：共 12 个拒绝检查。
- 合法嵌套 input、归一化后仍在目录内的 input、嵌套 asset、动态 chunk 与
  sourcemap：共 8 个正常对照，检查 JS 内容、源映射内容及真实写盘结果。

旧版 12 个拒绝断言失败、8 个正常对照通过；旧版 write 的 5 种实际名称路径
确实越出指定输出子目录，仍只写在测试自建临时根内。bundle key 不一致的旧版
案例是最终元数据未拒绝，并非该案例也已观察到越界落盘；generate 不进行写盘。
候选 20 个叶子全部通过（Node 含父节点汇总为 21）。套件加入 `yarn test:deps`，
不执行输出代码，不请求外部服务，结束后关闭 bundle 并清理自建临时目录。

## 进度和剩余限制

原分支 `codex/rollup-output-boundary` / `42988901` 叠加于 PostCSS PR #64
候选 `60aa55ea`。接续时已核实该前置合并，原分支保持不变；交付分支
`codex/rollup-output-delivery` 承接 PR #66 的 main 和 Babel PR #67，保留两者
测试入口及最新台账。合并前须更新到 Babel 已合并基线并核对准确候选 CI；
不得使用前置 PR 的检查替代本项检查。

独立预修复调查、旧新专项对照和完整前端验证已通过：30 个 Vite、20 个 Axios、
16 个 PostCSS、20 个 Rollup 叶子与 60 个 Vitest；lint 零错误/9 条既存警告，
生产构建通过且保留大 chunk 提示。新审计仍有 214 条路径/103 个 Yarn ID/77 个
GHSA，Rollup 匹配为零，其余不关闭。独立候选审阅因平台内容检查中断，
未返回结论；接续时全新只读候选审阅也被平台内容检查中断，仍无审阅结论。
没有通过改名或重派规避限制，此门槛保持未通过。接续本地 frozen 安装、上述
回归加 28 个 Babel 叶子、60 个 Vitest、lint 和 build 通过。新鲜审计因依赖树
向公共服务外发的自动审批被拒，等待专项授权；上文审计仅为原候选历史快照。
准确提交 CI 待完成，当前未合并、未发布、未部署。
Windows 原生路径、其他插件组合、完整浏览器页面与生产端到端均未验收。
保留 PostCSS 文档中的 Vite 独立开发 sourcemap 加载器边界及其他依赖公告。

回滚本项独立提交并重跑 frozen 安装/前端回归即可恢复旧版本，无 schema 或
配置迁移；回滚同时恢复旧版公告风险，不视为依赖问题已关闭。


## 2026-10-02 continuation (supersedes historical pending statuses above)

Candidate 79316d4c already has draft PR #68 and nine successful exact-candidate CI jobs. Its public dependency metadata audit completed after explicit authorization; see NEXT_RELEASE for those historical results. Earlier pending-authorization and pending-CI statements above describe the earlier stage only.

The delivery branch now incorporates main 1a89e9e28e884ebee417d54e647e012718b4c4e2 while preserving existing commit history. Conflicting ledger content is taken from current main and supplemented here. package.json keeps all main test entries and appends Rollup; the automatically merged lockfile changes only Rollup and native platform packages. Monaco 0.57.0, js-yaml 4.3.2 and current business fixes remain.

Preliminary temporary integration passed 8 normal Rollup leaves (9 including parent), 22 files / 209 Vitest tests, lint and production build, retaining large-chunk warnings. That dependency overlay was not a frozen installation and does not replace final-candidate CI. Independent review remains incomplete; author checks do not replace it. Keep this PR draft and do not merge. No tags, image publication or deployment.

Final integration local verification: Node 22.20.0 / Yarn 1.22.22 frozen offline installation (lifecycle scripts enabled), complete existing yarn test, 22 files / 209 UI tests, lint (zero ESLint warnings) and production build passed. Large-chunk and dependency peer warnings remain. The Rollup test file is byte-identical to candidate 79316d4c. Updated commit CI is pending; independent review remains incomplete.


## Delivery and subsequent Vite candidate

The owner explicitly approved a one-time independent-review exception for candidate 4ddc5b3d only. PR #68 merged as b53412b493b878532b7b5d377af6792b4256ccb0 with identical candidate tree; ten candidate checks and four main checks (36993356458) passed. This supersedes historical pending/do-not-merge statements for that candidate only; review was not completed. No release/deployment.

The subsequent Vite8 candidate replaces Rollup with Rolldown and removes Rollup from its lockfile and installed graph, so its standalone Rollup regression is retired there. The original test and bounded results remain available in #68 history. This does not erase the review gap or extend the owner's exception to Vite.
