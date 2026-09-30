# PostCSS 构建工具文件边界

## 范围与最小修复

顶层 Vite 7.3.5 和测试工具内的 Vite 7.3.6 均通过 `postcss@^8.5.6`
解析到锁定的 PostCSS 8.5.6。本项只将该锁文件条目更新为 8.5.23，并更新其
必要的 nanoid 闭包至 3.3.19；不增加直接依赖、resolutions 或更换 Vite。
PostCSS 在开发和构建时处理 CSS，最终 Go 镜像不运行该编译器。

相关上游公告：

- [GHSA-6g55-p6wh-862q](https://github.com/postcss/postcss/security/advisories/GHSA-6g55-p6wh-862q)：非 map 文件注释读取。
- [GHSA-r28c-9q8g-f849](https://github.com/postcss/postcss/security/advisories/GHSA-r28c-9q8g-f849)：父目录 map 读取。
- [GHSA-fxqj-rqcc-2cmp](https://github.com/postcss/postcss/security/advisories/GHSA-fxqj-rqcc-2cmp)：缺少 `from` 时的 map 读取。
- [GHSA-qx2v-qp2m-jg93](https://github.com/postcss/postcss/security/advisories/GHSA-qx2v-qp2m-jg93)：HTML style 上下文；当前 Vite 使用 `style.textContent`，未发现应用将 PostCSS 输出拼接进 HTML style 的路径，仅作当前受检路径 no_change，不冒充动态 HTML 验收。

真实 Vite 主处理及 CSS import 均提供 `from`；缺少 `from` 的对照属于库边界。
当前未发现业务 API 上传内容进入 CSS 编译器的入口，不能把源文件或依赖 CSS
可控这一前提写成生产 API 已可远程利用。

## 旧版失败与正常对照

`web/tests/postcss-file-boundary.test.mjs` 直接解析顶层 Vite 使用的 PostCSS，
在导入库前观察 `readFileSync`，真实文件系统读取与编译逻辑不替换。
观察范围仅为测试自己创建的两个合成文件；结束后关闭开发实例并清理临时目录。
测试不加载项目 dotenv，不启动 HTTP 监听，不打开浏览器或请求真实上游。

共 16 个叶子检查：

- 3 个库边界：非 map、父目录 map、缺少 `from`。
- 4 个实际 Vite 编译边界：生产构建的非 map、父目录 map、import map；开发转换的 import map。保留默认 `css.devSourcemap=false`、`build.sourcemap=false`，实际观察处理器版本。
- 9 个正常对照：库的同目录 map 与 inline map；Vite 普通 CSS、CSS Modules、SCSS、import/资源 URL、同目录 map、inline map及包含这些内容的生产构建。正常 map 对照显式开启 sourcemap 并检查原始内容保留。

旧版 8.5.6：7 个边界检查失败，9 个正常对照通过；其中非 JSON 合成文件导致
真实解析失败，其他越界读取被计数断言捕获。候选 8.5.23：16 个叶子检查全部
通过（Node 报告含父节点为 17）。测试已接入 `yarn test:deps`，随 `yarn test` 运行。

## 保留边界与当前状态

独立探针发现 Vite 开发加载器会在 PostCSS 前自行加载直接 CSS 注释所指向的
map：升级前后均会异步读取合成父目录文件。当前默认 `devSourcemap=false`
探针成功返回的 map 是空 mappings，未证明其他配置均无披露。因此本项只关闭
已验证的 PostCSS 读取路径，**不能声称整个 Vite sourcemap 文件边界都已关闭**。
该剩余行为继续保留，后续需结合开发服务器暴露范围及可控 CSS 来源验收。

本项没有验证符号链接的 realpath 约束、Windows 原生路径、浏览器完整页面或
生产端到端。上游修复的路径检查不能据此扩写为任意文件系统访问均被阻断。

本地 Node 22.20/Yarn 1.22.22 frozen 安装、30 个 Vite 边界、20 个 Axios
边界、16 个 PostCSS 边界及正常对照、60 个 Vitest 检查、lint 和生产构建通过。
新审计仍有 217 条路径记录、104 个 Yarn 公告 ID、78 个唯一 GHSA；PostCSS 及
nanoid 公告已无匹配，其他依赖保留。Lint 仍有 9 条既存警告，构建仍有大 chunk 提示。新鲜只读候选审阅已独立复跑 16 项，
并验证归一化父目录拒绝、大写扩展名/合法子目录 map 与显式受信任回调正常，
未确认阻断项。审阅分派包含父任务测试结果和范围解释，因此并非盲审；此流程
限制仍保留。准确提交 CI 和合并已完成，见下方；未创建标签、发布镜像或部署。

回滚使用本项独立提交的反向提交并重跑 frozen 安装和前端回归；回滚会重新引入
旧版公告匹配，不能作为依赖问题已关闭的版本。

## 合并与准确候选验收

[PR #64](https://github.com/dreamvm/one-hub/pull/64) head
`60aa55eab6270992dbd1426c9715531f30005351` 合并为
`04ea23dbc3cf45a0f6d1f0477a0cd0c3dd755634`；候选、GitHub 合成、计算和
实际合并树均为 `8ae8d22ab43ac126f4378b64572d8a85093719e2`。
[兼容性 CI](https://github.com/dreamvm/one-hub/actions/runs/36742294716) 与
[隔离镜像 CI](https://github.com/dreamvm/one-hub/actions/runs/36742295282)
全 9 项通过，包括 Linux 前端 66 个原生叶子及 60 个 Vitest、三数据库各
245 个事务叶子、41 个后端 smoke 加 7 个升级/回滚检查、四种 Compose 启动。

实际 linux/amd64、CGO=1 程序确认使用 Go 1.25.14，binary SHA256 为
`2ed56f7aaf528acb063b511b77d8d529f884a2644b552cb922fda7e50ac734cf`；
隔离 runner 镜像 ID 为
`sha256:1e5350b05eee550c11ab5bd5a4c76eef0460d83b487976bbae67db36505d5408`。
镜像没有发布或部署。CI 通过不扩展上文的工具路径、审阅上下文和未验证边界。
