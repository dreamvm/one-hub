# Axios 1.18.0 新公告的当前调用核对

2026-09-30，源码基线 main `5f0a7fc8139757ec31bfb297806a7bd6ab9e5690`。
授权审计中的 12 条新记录均匹配 Axios 1.18.0，公告列出的修复版为 1.20.0。
本项不升级依赖，仅作当前产品调用的静态判断；不能将 not_actionable 写成库已修补。
审计快照来自未合并 Vite 候选，但其 Axios 版本及业务调用与 main 相同。

## 产品边界

生产 Axios imports 仅在 `web/src/utils/api.js`、`StatusPanel.jsx`、`SystemLogs.jsx`。
前者创建两个浏览器客户端，只注册 response interceptor；后两者使用明确 get/post 方法。
应用入口为 DOM + BrowserRouter，没有 SSR 或 Node Axios 服务。包的 browser 映射排除
Node HTTP adapter，默认适配器在标准浏览器先选 XHR。自定义 Node 测试适配器不是生产入口。
没有适用 SECURITY.md；判断范围来自现有源码、构建与部署事实，不假定原型污染永远不存在。

## 逐条静态结论

12 条均为当前产品路径 not_actionable，中等置信度；依赖版本仍匹配公告。

| 编号 | 公告 | 必要前提与当前反证 |
|---|---|---|
| 001 | [vh66-26gq-q6x8](https://github.com/advisories/GHSA-vh66-26gq-q6x8) | fetchOptions 继承值需进入 fetch adapter；当前标准浏览器选 XHR，无 adapter/fetchOptions 覆盖 |
| 002 | [9fr6-4gfg-395g](https://github.com/advisories/GHSA-9fr6-4gfg-395g) | 默认实例省略 method；当前调用全部使用明确方法别名，别名写入 own method |
| 003 | [c29m-xwm3-cm6r](https://github.com/advisories/GHSA-c29m-xwm3-cm6r) | Node data URL 解析器；浏览器产物排除 Node HTTP adapter |
| 004 | [mghh-pgcx-3jjj](https://github.com/advisories/GHSA-mghh-pgcx-3jjj) | Node 代理重定向及 NO_PROXY；没有产品 Node Axios 消费者 |
| 005 | [x97p-jq2g-jp4f](https://github.com/advisories/GHSA-x97p-jq2g-jp4f) | 进入受影响表单序列化；无 multipart/urlencoded 配置、formSerializer 或表单别名，当前对象请求为 JSON |
| 006 | [3pq3-5fj3-cg6v](https://github.com/advisories/GHSA-3pq3-5fj3-cg6v) | Node HTTP/2 DNS/代理边界；没有该产品调用 |
| 007 | [542g-h47m-68v8](https://github.com/advisories/GHSA-542g-h47m-68v8) | Node HTTP/2 session；没有该产品调用 |
| 008 | [j8rh-479h-cp32](https://github.com/advisories/GHSA-j8rh-479h-cp32) | request interceptor 返回缺少 own headers 的新配置；项目只有 response interceptor |
| 009 | [4hqw-qxg8-jxx2](https://github.com/advisories/GHSA-4hqw-qxg8-jxx2) | 非标准环境调用继承 getHeaders；标准浏览器/webworker 分支跳过该调用 |
| 010 | [m8m8-qj5v-23w3](https://github.com/advisories/GHSA-m8m8-qj5v-23w3) | Node HTTP adapter 继承 createConnection；浏览器产物不含该路径 |
| 011 | [44g4-m2mj-wpvx](https://github.com/advisories/GHSA-44g4-m2mj-wpvx) | Node CIDR NO_PROXY；没有产品 Node Axios 消费者 |
| 012 | [r4gj-5m52-g5wh](https://github.com/advisories/GHSA-r4gj-5m52-g5wh) | 将 fetch 的 maxRedirects:0 当 SSRF 控制；项目未使用该选项或服务器 Axios 边界 |

源码反证包括 Axios `lib/core/Axios.js:243–266` 的明确方法别名、
`lib/defaults/index.js:79–100` 的表单/JSON 分支及 `lib/helpers/resolveConfig.js:74–83`
的环境判断。009 的 helper 也被 XHR 使用，不能只凭公告标题含 fetch 就排除；实际标准
浏览器分支不会调用 getHeaders。已有 FormData 仅从搜索表单提取 keyword，没有作为请求体。
四个 Content-Type 设置属于原生 fetch 的 WebAuthn JSON 请求，未进入 Axios 表单序列化。

没有运行新漏洞探针、升级候选、独立补丁审阅或候选 CI；本项没有代码补丁。
未来增加 SSR、fetch-only 运行时、上传表单、请求拦截器或自定义适配器必须重新核对。
Go 侧上游 URL/代理控制与未决依赖另行验收，不由此项关闭。

