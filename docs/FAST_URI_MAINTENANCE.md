# fast-uri 的兼容维护边界

2026-10-14，基线为 PR137 实际合并提交
`4c2f31844c5a992879b9ffd83d21cb3cd0d04907`。本项仅将 web 锁定的
fast-uri `3.0.6` 更新为 `3.1.8`，保留消费者范围 `^3.0.1`，不增加全局 resolutions。
官方 npm tarball 的 SRI、SHA1 和完整下载 SHA256 均已独立核对；SHA256 为
`86be033b406a7737c0521edc8fe3e15c7ac0cb6b5e509478cc9539a2efaa086c`。

原 175 条审计路径中，本库对应 18 条路径、9 个去重公告。路径数不是漏洞数；
当前证据没有建立 One Hub HTTP 输入到本库的可利用链。受检实际消费者是
`conf → Ajv 8.17.1`，CLI 配置 schema 固定为两个字符串字段。Ajv 使用
`parse/serialize/resolve`；不会把本库直接用作网络目的地授权校验。

## 回归及正常调用

`web/tests/fast-uri-boundary.test.mjs` 纳入 `yarn test:deps`。所有输入均为
合成字符串或保留域名，测试不发请求，Conf 只写自己的临时目录。
子进程有 192MiB 堆上限、5 秒超时和 32KiB 输出上限。

- 24 个边界用例覆盖反斜线 authority、替代 authority introducer、控制字符、
  百分号转义、scheme、IPv6/端口、IDN、保留 path 转义、组件端口和 host 大小写。
- 3 个正常对照分别检查普通 URI、实际嵌套 Ajv 的相对/转义 fragment 引用，
  以及实际 Conf 的默认值、合法写入、类型拒绝及拒绝后的值保持。
- 同一份测试在旧 `3.0.6` 上：24 个边界失败，3 个正常对照通过；
  在 `3.1.8` 上：27 个叶子用例全部通过。Node TAP 总数另包含父测试。

分别断言异常接口：parse 返回 error 字段，normalize 保留原输入，equal 返回
false，resolve 抛错。数字端口 `65536` 在 parse 时拒绝，serialize 本身仅检查
端口表示，不提供数值范围保证。测试不把 error 字段当成通用授权边界；
Ajv 自身也不全面检查该字段。

## 保留的兼容及验收限制

`3.1.8` 移除了旧运行时复数导出 `resolveComponents`，只保留单数
`resolveComponent`；`reference: 'suffix'` 配合 scheme 的解析也有行为差异。
已检查的实际消费者不使用这些接口，不宣称所有公共 API 完全兼容。
不升级 Ajv、不改变 Lobe 配置入口，不触及 YAML v3、matcher 或 Monaco。

本项不覆盖 docs 的独立依赖树、Prettier 内嵌依赖，也不消除其他公告。
独立绕过/回归审阅、精确候选 CI、合并和发布分别记录在 PR 中；本记录不预先
宣称这些步骤成功。回退方式是回退本项提交并执行 frozen install；回退会恢复
旧库的已记录边界缺陷。任何发布仍需先关闭其余发布门槛。
