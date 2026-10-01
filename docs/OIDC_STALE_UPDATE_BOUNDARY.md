# OIDC 解绑与旧用户快照

独立分支 `codex/oidc-stale-update-boundary`，基线为已合并 main
`5fb760c608a86df0690534ba884279b79744068d`（PR #80）；不依赖未合并的 Realtime 草稿。
本项只处理普通用户保存覆盖 OIDC 绑定的并发问题，不实现 issuer 迁移或身份唯一注册。

## 共享保存边界

登录、资料、密码以及其他 OAuth 操作可能读取完整 User，再调用 `User.Update`。
原实现把读取时的非空 oidc_id 一并写回；其间发生的显式解绑或绑定变化会被旧快照覆盖。
正常资料更新不能成为重新绑定身份的操作。

最小修改是在共享 Update 的 omit 字段中加入 oidc_id，同时覆盖密码更新和普通更新。
显式 Unbind 的按列清空仍有效；当前经验证的新 OIDC 注册使用 Insert，不受该保存修复影响。
普通管理资料更新也不再通过单独传入 oidc_id 改绑定；现有界面没有通用 OIDC 绑定编辑入口。
没有新增登录、迁移、核销或生产操作接口。

## 验证

新鲜只读预调查和父任务源码追踪覆盖 setupLogin、Unbind、UpdateUser 与 User.Update 调用方。
临时 SQLite 的确定性交错：先读用户，再进行独立绑定操作，最后保存旧资料。
旧实现四个清空/替换场景失败；两种密码模式下绑定不变的正常控制通过。
候选 44 个专项叶子/47 个节点通过，包括实际本地解绑路由、普通资料及密码正常保存、
既有 OIDC claims/同名/精确 subject 控制和相邻 OAuth session 行为。
测试只使用合成账号、临时数据库和模拟 OIDC；没有真实身份平台或凭据。

Go 1.25.14 规定离线包及 controller/middleware 的 race 回归、工作流策略、相关 vet、
providers/relay 编译全部通过。全新只读候选审阅未发现本范围具体绕过或阻断回归；
审阅者独立用旧源码 overlay 确认四个解绑/替换场景及实际 HTTP 解绑场景失败，
候选正常资料、密码、token rotation、OIDC claims 和相邻 OAuth 控制通过。
SQLite/MySQL/PostgreSQL 的两种密码模式生成 SQL 均排除 oidc_id；后两者仅为 DryRun，
不冒充真实数据库执行。审阅后源码/测试未改动。
源码/测试补丁 SHA256 `01ffdbf918b5acc0efbba7ffdfa5f5f4318053e4b77b9029e5629658e9b01ec4`。
准确候选 CI/PR/合并仍待完成，本地通过不等于交付。

## 兼容与保留限制

回退会恢复旧快照覆盖绑定的行为；本项无 schema 或数据迁移。
解绑没有被扩展为撤销所有既有 session；登录与解绑并发的 session 授权时点仍按原规则。
其他身份字段、角色/状态的旧快照行为不由本项概括保证。

issuer 持久归属、subject 唯一性、并发新注册及历史身份恢复仍需后续独立实现和验收。
用户已选择保守迁移：未知历史 issuer 或重复归属保留待核实，不根据当前配置或首次登录自动关联。
相关旧 OIDC-only 账号可能需要独立恢复路径；当前未改变生产身份或启用该迁移。
没有标签、镜像发布、生产部署或真实付费调用。

## 合并验收

[PR #81](https://github.com/dreamvm/one-hub/pull/81) head
`86e6a6601a1e3076deba0e43c34a10f4c0c818a5` 已合并为
`416a52ea533131d1f64f29efb3764c9c775bb489`；候选/合成/计算/实际树均为
`b0383f636fb9efb8ec360a84beeac6f7f286628b`。
[36827130791](https://github.com/dreamvm/one-hub/actions/runs/36827130791) /
[36827131193](https://github.com/dreamvm/one-hub/actions/runs/36827131193) 全 9 项成功；
合并后 [36828389683](https://github.com/dreamvm/one-hub/actions/runs/36828389683) 也已通过。
隔离 smoke 48 个业务/升级回滚 PASS；最终 runner 镜像
`sha256:f8e9c4e82868b178bd300b4b896a124a6680532ab2d1694118662c854995d456`
包含 Go1.25.14 / one-api / linux/amd64 / CGO1，binary SHA256
`062ad2c12b2d472371f9d4eb9852349895d535984afe357087d81b7b6397bdfb`；未发布。
此交付仅覆盖旧快照保存；后续 issuer 迁移另见 [OIDC_IDENTITY_BOUNDARY.md](OIDC_IDENTITY_BOUNDARY.md)。
