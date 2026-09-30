# Vite 开发服务器文件边界

本批处理顶层 Vite 开发服务器的文件读取边界，属于开发工具依赖。
项目 `server.host=true`、WebSocket 未关闭，运行开发服务器时适用以下公告：

- [HTTP deny/query](https://github.com/vitejs/vite/security/advisories/GHSA-v2wj-q39q-566r)：检查对象与实际读取路径不一致。
- [WebSocket 模块读取](https://github.com/vitejs/vite/security/advisories/GHSA-p9ff-h696-f583)：客户端模块调用绕过 HTTP 文件限制。
- [优化依赖 sourcemap](https://github.com/vitejs/vite/security/advisories/GHSA-4w7w-66w2-5vf9)：解析后的文件越过优化缓存目录。

顶层 Vite 从 7.1.11 固定升级到 7.3.5，React 插件从 4.3.1 固定升级到 4.7.0，
后者声明支持 Vite 7。只更新必需依赖闭包，Vitest 3.2.7 及其独立 Vite 7.3.6 保留。
Node 22.20 与 Yarn 1.22.22 满足工具要求。默认开发配置及 Go 静态资源服务保持原有行为。

## 回归与正常对照

`web/tests/vite-file-boundary.test.mjs` 使用 Node 原生 runner，显式导入顶层 Vite，
并加载真实项目插件。`yarn test` 先执行此边界回归，再执行既有 Vitest UI 回归。
临时根目录只含合成文本、证书标记及有效 sourcemap；禁止 dotenv 加载与浏览器自动打开，
仅监听 localhost 的随机端口，明确限定文件 allow 范围，不调用 API 代理或真实上游。
测试在退出时关闭 WebSocket、服务器和文件监听，再清除本次临时目录。

30 个叶子用例覆盖普通与绝对文件路径、查询顺序、编码路径、WebSocket 模块访问及直接拒绝控制。
正常 raw/inline、缓存内 sourcemap、HMR 自定义消息与服务端模块加载保持成功。
原始锁文件的隔离旧版副本中 17 个叶子失败，均实际返回合成保护内容；正常控制通过。
修复版 30 个叶子全部通过，保护内容不再返回。Node 汇总另包含父测试，因此显示 31 项。
SPA fallback 的 HTTP 200 本身不表示泄漏，断言核对实际合成内容是否返回。

本地 frozen 安装、边界回归、56 项 UI 回归、lint、build 已通过；lint 仍有 9 条既存警告，
构建仍有大 chunk 提示，留给后续行为与体积专项。候选 SHA、独立审阅、CI 与合并事实
按 [NEXT_RELEASE.md](NEXT_RELEASE.md) 和最终 PR 记录，不从本地通过推断已合并。

## 未完成边界与回滚

7.3.5 也包含上游的 [Windows ADS/短文件名修复](https://github.com/vitejs/vite/security/advisories/GHSA-fx2h-pf6j-xcff)
及 [UNC editor 修复](https://github.com/vitejs/launch-editor/security/advisories/GHSA-v6wh-96g9-6wx3)。
已核实上游版本及源码依据，未执行 Windows/NTFS 原生验证或 NTLM 验收，不能登记为已原生验证。
本批没有证明所有浏览器运行代码、构建工具及前端依赖公告均已关闭，也没有证明生产 Go 服务受这些开发服务器公告影响。
后续继续分别核对其余依赖和发布阻断项。

撤销本批依赖提交会恢复旧版开发服务器风险；回滚后边界回归应失败，不能绕过测试将其记为通过。
本批不改数据库、不创建候选标签、不发布镜像、不部署生产。
