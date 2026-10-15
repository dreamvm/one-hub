# form-data 类型依赖副本维护

2026-10-14。前置提交为 fast-uri 维护候选
`3aebe7d62fd347c4943f1f6677c6bd27dda19956`。本项只将
`form-data@^4.0.0` 合并到已锁定的 `4.0.6` selector，不添加依赖或全局 resolutions。
原 `@types/node-fetch/node_modules/form-data` 为 `4.0.2`；顶层 Axios 使用的
`4.0.6` 已在基线中，不能把顶层库当作旧失败对照，也不把它重复记作新修复。

原 175 路径有本库 4 路径、2 个去重公告。旧副本仅由 `@types/node-fetch`
的 `.d.ts` 引用；OpenAI 实际 Node multipart 用 formdata-node/encoder，Axios
用已修复的顶层副本，浏览器用原生 FormData。当前已检查的项目路径没有旧副本
运行调用。此项消除锁树中的受影响版本，不声称修补了已证实的产品运行时漏洞。

## 真实旧副本与正常对照

`web/tests/form-data-boundary.test.mjs` 从 `@types/node-fetch` 位置解析实际库，
纳入 `yarn test:deps`。不发请求，不读文件或真实上传；使用合成字段及 Buffer。
每个子进程限制 192MiB 堆、5 秒和 32KiB 输出。

- 1 个隔离默认边界对照固定 Math.random 并捕获 crypto.randomBytes，验证新的
  默认值使用 12 个加密随机字节且重复 getBoundary 不再生成；不依赖统计碰撞测试。
- 4 个字段参数对照及 5 个 filename 来源对照检查 CR、LF 和引号转义，覆盖
  string filename、options.filename/filepath 和 Buffer 的 name/path。
- 2 个合法对照检查 UTF-8 文本、普通 Buffer/数字、长度及 Content-Type，以及
  显式 boundary 和可信自定义 header。

同一测试的真实嵌套 `4.0.2`：10 个异常叶子失败，2 个合法对照通过。
候选新版、本地完整回归、独立审阅和准确 CI 分别在交付时核对，未完成步骤不预记通过。
官方 `4.0.6` tarball 的 SRI、SHA1 及 SHA256 独立核对通过，后者为
`fc4d94e9b629f5378367ba46da7d96115696c01e25c99cd57c2c9a3d098bb557`。

## 保留 API 边界

字符串 options.header、自定义 header/contentType 仍是原始模式，不因字段参数
转义变成通用可信输入。显式 boundary 由调用者控制，不能称其不可预测。
版本另有 null/undefined append 转为字符串、非字符串 setBoundary 抛错和默认
边界字符从数字变十六进制的差异；原副本是类型依赖，未发现运行调用受这些差异影响。
没有升级 YAML v3、matcher、flatted、Immutable 或 Lobe CLI，也不修改真实配置。

本项依赖前置 fast-uri PR，合并前须检查准确组合树。回退本项恢复旧嵌套副本
和相应库边界缺陷；合并、发布、部署均另核批准。
