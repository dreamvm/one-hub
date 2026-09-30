# Vite 独立 source map 加载边界

## 问题与范围

基线 `4603ed139c3be1353e99c7260b70dd97a314670b` 使用 Vite 7.3.5。
开发服务器在转换依赖 JS/CSS 时，自己的 `extractSourcemapFromFile` 会跟随外部
sourceMappingURL；`injectSourcesContent` 会补读缺少内容的 sources/sourceRoot。
这些读取不经过普通模块请求的文件许可检查，PostCSS/Babel 的修复不能代替它们。
CSS 注入响应还可能携带 map，不能仅检查 transformRequest 返回的 map 字段。

前提是开发服务器处理了带可控 map 的依赖文件，并且输出可被读取。当前配置
`host: true` 不能证明存在公网开发实例；未发现业务 API 将上传内容送入该编译路径。
Go 生产服务使用静态构建资源，本项不声称已证实生产业务接口可远程利用。

## 候选与兼容性

采用 [Vite 上游修复 f05f501](https://github.com/vitejs/vite/commit/f05f50173461789e0f1323fe06b51f18ca41c132)
所在的 Vite 8 系列，精确锁定 8.3.1。上游在依赖包根同时约束外部 map 和补充源文件读取；
允许包内 dist 到 src 的映射，保留第一方源码的合法父目录 map。已检查的 7.3.6 仍缺此控制。

plugin-react 4.7.0 的 peer 范围不含 Vite 8，故改为仍使用 Babel 且支持 Vite 8 的 5.2.0；
Vitest 改为 4.1.11，使测试工具也解析同一 Vite 8，移除其旧 Vite 7 副本。
直接 Babel 7.29.7、Axios 1.18.0、Monaco 0.57.0 保持不变。
锁文件新增项为这三者要求的构建/测试闭包及原生平台包，旧闭包和无引用条目随解析移除，
未整体刷新其他既有 selector 的版本。

这是构建引擎迁移：Vite 8 使用 Rolldown/Oxc 和 Lightning CSS，见
[官方迁移说明](https://vite.dev/guide/migration)。显式保留原 JavaScript 目标
Chrome 107、Edge 107、Firefox 104、Safari 16；这不等于已在这些旧浏览器运行验收。
候选锁文件不再依赖 Rollup，但独立 Rollup PR #68 的平台审阅限制仍原样保留，
不能据此宣称它通过审阅、已合并或已完成其候选交付。

## 受控旧新对照

`web/tests/vite-source-map-boundary.test.mjs` 接入 `yarn test:dev`，只创建并观察自己的
两个标记文件。每项启动随机 localhost 服务，检查实际读取、转换结果和 HTTP 内联 map；
请求超时 3 秒、父测试 30 秒，结束关闭服务器、恢复观察器、清理临时目录。

- 9 项边界检查：JS 外部 map、CSS direct 默认/开启 map、inline/external missing sources、
  sourceRoot、scoped 包两种输入、开启 map 的 CSS 注入响应。
- 6 项正常对照：包内父目录 map、第一方父目录 map、普通 JS 外部/inline map、
  包内补读 source content、CSS 外部 map。
- 7.3.5 的 9 项边界失败、6 项正常通过；8.3.1 的 15 项全部通过。
  旧 CSS 注入场景的返回 map 没有标记，但实际 HTTP 响应有标记，新版两者均无。

Node 22.20.0 / Yarn 1.22.22 下 frozen 安装、109 个依赖叶子（45 Vite、20 Axios、
16 PostCSS、28 Babel）和 62 项 Vitest 通过；Node 含父节点分别报告 47 与 67。
lint 零错误、9 条既存 warning，生产构建通过并保留大 chunk 提示。

官方 Yarn 审计接收域名和发送字段已再次核实：仅向
`registry.yarnpkg.com/-/npm/v1/security/audits` 发送公开包名、版本、完整性哈希、
依赖关系和 dev 标志；不发送源码、resolved 下载地址、凭据或运行配置。
本次返回 191 条路径、69 个唯一 GHSA，Vite/Vitest/Rolldown/PostCSS/Rollup 无匹配；
这不是全应用无漏洞结论，Axios、i18n 及 DOMPurify 等剩余公告仍需各自处理。

## 浏览器与未验证范围

Chrome 154.0.8037.58、macOS、1440×1000，localhost 静态构建与模拟 API：
频道列表未请求 Monaco runtime/worker；打开编辑器后 JSON/core worker 为同源资源，旧 CDN
请求为零。三个 JSON 入口完成正常键入、撤销/格式化、映射与列表错误拒绝、一次模拟保存；
深色模式重新打开编辑器并键入/撤销/格式化通过，截图已检查，无 pageerror。
外部 Iconify 使用合成占位响应，其他外部请求拒绝，不验证真实网络资源或后端保存。

开发模式实际页面加载 SCSS 与模块样式，React 可见文字修改通过热更新显示且保留同一 document；
仅修改临时副本，随后恢复。初始价格 API 夹具误用对象导致 prop-type 警告，已改为预期数组。
修正后开发模式的同源 JSON/core worker、键入、撤销和格式化通过，无 pageerror；
既有 MUI 对话框 aria-hidden/focus 警告另记，不宣称无障碍验收通过。

按 fix-finding Skill 发起的一次全新只读候选审阅被平台内容检查中断，未产生结论。
没有改写请求、换路由或重试来绕过限制。候选只可保留为待审阅草稿；本地结果及后续
准确候选 CI 不能代替缺失的独立审阅，当前未合并。

上游控制是依赖词法包根边界，不是通用 realpath 沙箱。符号链接、原生 Windows、
HTML style 及其他优化器入口未由上述 15 项穷尽验证；第一方可信源码的外部 map 有意保留。
旧浏览器、真实移动设备、生产网络、完整页面端到端和 arm64 实际镜像仍开放。
Monaco 的既有审阅覆盖限制、hover 裁剪和其余发布阻断项不由此关闭。

回滚须反向提交本项依赖/配置/回归改动、恢复旧锁文件并重做 frozen 安装和验收；
会重新引入旧版 source map 风险。RC8 仍预留，没有标签、镜像发布、部署或付费调用。
