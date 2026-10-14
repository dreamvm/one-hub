# brace-expansion 两个维护分支

2026-10-14。本项叠加 fast-uri 和 form-data 两项独立维护提交，仅将 web
brace-expansion `1.1.11 → 1.1.21`、`2.0.1 → 2.1.7`。保留两组 selector 和
balanced-match/concat-map 依赖，不加全局 resolutions，不改变 minimatch/picomatch
或 YAML 的已保留兼容限制。实际前置 SHA 和精确组合 CI 在 PR 中记录。

原 175 条审计路径中，本库占 40 路径、16 个 Yarn ID、8 个去重公告。
维护阶段受检的是源码中固定的小型文件模式和 JSON i18n 入口，未建立 One Hub
HTTP 输入到 expand 的可利用链；这也不是任意用户 glob 排除规则的完整性保证。

官方 npm 完整 tarball 的 SRI、SHA1 及 SHA256 已核对：
1.1.21 为 `d56cc19743e0521ebe46964412a5a4b247749036de7af3a9f78dd384f58fdb56`；
2.1.7 为 `6f4d64fdab4cdc5096d48537f35a6bbb2b8b918abaa4bdb0833e92486f66befc`。

## 有界及正常对照

`web/tests/brace-expansion-boundary.test.mjs` 从实际 minimatch3 和 glob10 的
minimatch9 加载对应两个分支，纳入 `yarn test:deps`。每个子进程限制 192MiB
堆、5 秒和 32KiB 输出。只用短字符串和独立临时目录，不运行 OOM 或无界扩展。

每分支分别检查输出条数、总字符、嵌套深度和 rewrite 的小预算，以及常规 grammar、
真实消费方文件匹配。旧两个分支：8 个预算叶子失败，4 个正常 grammar/消费方
叶子通过。新版结果、完整回归、独立审阅及精确 CI 在交付时另核，不预记成功。
小预算对照验证了控制逻辑，不冒充对所有默认阈值的压力测试。

## 必须保留的语义

默认 max=100000、maxLength=4000000、maxDepth=1000、maxRewrites=1000。
超限返回部分输出或剩余字面内容，没有异常或截断标记；minimatch 不转发本测试
显式设置的小预算。调用方若需要任意规则完整展开，应另界定/拒绝超限输入，不能
根据升级版本推断全部文件仍匹配。

已核对的小型 rewrite 差异：`{a},}` 旧版含额外空结果，新两版去掉该空结果；
1.x 的 `{a,b}{c},}` 也会去掉两项旧结果。受检维护入口不使用这些模式，固定
四扩展名、常规序列、嵌套备选及转义模式保留正常对照，不声称所有 Bash 边角行为兼容。

Prettier 内嵌 brace/minimatch 和 docs 独立锁树仍不随本项更换；braces 3.0.3
是另一库，其无补丁公告也不由本项关闭。minimatch3 嵌套 extglob、picomatch4
既有分支差异继续开放。回退本项恢复旧库及其边界缺陷；合并/发布需另核授权。
