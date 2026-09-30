# Axios 浏览器依赖维护

## 受检边界

直接依赖由 Axios 1.12.2（原声明 `^1.8.2`）固定到 1.18.0，锁文件仅更新其必要依赖闭包。
这是同一主版本的依赖缺陷修补与防御加固。既有审计中 28 条 Axios 记录匹配旧版本，
不等于 28 条已证实的 One Hub 可利用漏洞。

应用通过 `utils/api.js` 的 API/LoginCheckAPI，以及 StatusPanel/SystemLogs 调用 Axios。
浏览器导出移除 Node HTTP adapter，正常浏览器优先使用 XHR。最终容器运行 Go 程序；
当前构建和 i18n CLI 路径未发现 Node Axios HTTP 调用，不能把 Node 专属 SSRF 公告直接当作后台代理风险。

应用表单和外部价格 JSON 作为请求数据，搜索使用固定层级的 params；价格地址由管理员配置。
当前没有确认从这些输入污染 `Object.prototype` 的路径。共享核心的继承配置缺陷仍可在
人为设置此前置条件后复现；本批不宣称已证明应用攻击来源。

上游 [1.18.0 发布](https://github.com/axios/axios/releases/tag/v1.18.0)、
[无请求体方法公告](https://github.com/axios/axios/security/advisories/GHSA-mmx7-hfxf-jppx)和
[JSON reviver 公告](https://github.com/axios/axios/security/advisories/GHSA-3w6x-2g7m-8v23)
用于核对修复版本和前置条件。新版配置采用 own-property 边界，并收紧畸形 HTTP URL；
当前应用不依赖继承配置或缺少 `//` 的 HTTP 地址。

## 回归与正常控制

```sh
cd web
yarn install --frozen-lockfile --non-interactive
yarn test:deps
yarn test
yarn lint
yarn build
```

`axios-browser-boundary.test.mjs` 显式加载顶层包的浏览器发行产物，确认 Node HTTP adapter 不可用。
五项合成污染检查各在独立子进程运行：DELETE/GET/HEAD/OPTIONS 必须正常完成且无继承请求体，
JSON transform 必须忽略继承 reviver。旧版四个方法均在适配器前抛出 validator TypeError，
第五项发生响应字段改写；不能把旧版结果写成请求体已发送。正常批量 DELETE、JSON、显式 reviver、
baseURL/params/绝对地址、POST 数据、Blob 和取消控制旧新版本均通过。

`axios-xhr.test.mjs` 用同一浏览器发行产物和 jsdom 的实际 XMLHttpRequest，
连接两个随机端口的合成本地服务。八项控制覆盖默认 XHR、中文/标点/NUL 查询、JSON POST、
批量 DELETE、允许 CORS 的跨源价格地址、Blob 字节、401 JSON 与预先取消。
旧版合计 20 个叶子中 5 失败、15 通过；升级后 20 个叶子全部通过。
Node 汇总 21 项还包含 XHR 父测试。

`axios-api-compat.test.jsx` 加载真实应用 API 工厂和真实 Axios，仅模拟响应、通知与 store。
它保留两实例 baseURL/JSON、API 的 401 会话清理、普通错误及 LoginCheckAPI 的拒绝行为。
API 既有错误拦截器处理后返回 `undefined`，本批没有改变该约定。

## 验收范围与回滚

上述是库核心、jsdom XHR 和 API 集成控制，不是原生浏览器、真实跨站服务、完整页面或生产验收。
Cookie/重定向/代理的所有变体、应用原型污染入口及其他前端公告没有因此关闭。
取消控制检验库兼容性，当前应用没有 signal/cancelToken 消费点。
本地完整检查与独立审阅通过；准确候选 CI 和实际合并事实如下，完整隔离镜像身份及剩余项见
[NEXT_RELEASE.md](NEXT_RELEASE.md) 的 D10 合并验收。

## 合并验收

[PR #62](https://github.com/dreamvm/one-hub/pull/62) 已合并，最终候选
`93d58cb9a7d7add43b3c22e448b0a588468493ad`，实际合并
`49ddd66ec3826d2b440ea23dd79104ba0680d0d1`；候选、GitHub 合成、合并前 main 计算与
实际合并树均为 `985c2565a5c26223e7b8efee311441a3355a5c42`。
[兼容性 CI](https://github.com/dreamvm/one-hub/actions/runs/36735528407) 和
[隔离镜像验收](https://github.com/dreamvm/one-hub/actions/runs/36735528944) 全 9 项成功。

Linux 前端日志确认 30 个 Vite 与 20 个 Axios 的 Node 原生 runner 叶子、60 项 Vitest、lint 零错误/9 条既存警告及构建通过。
三数据库各 245 个事务叶子通过；三后端 smoke 41 项与升级/两条回滚 7 项合计 48 个 PASS，
另有四种 Compose 实际启动通过。独立审阅未确认存活绕过或兼容性回归。
升级后的依赖审计已无 Axios 匹配；其余包仍有匹配记录，应用污染来源仍未证实，原生浏览器验收仍未完成。
本批 runner 镜像仅在隔离环境加载，未发布。

回退本批提交会恢复旧依赖缺陷；没有数据库或生产配置变更。
RC8 仍只预留，标签、发布镜像、部署和真实付费验收分别核对授权。
