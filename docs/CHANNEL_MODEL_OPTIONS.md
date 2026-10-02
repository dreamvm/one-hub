# 渠道模型选项空集合

2026-10-01，基于浮层候选 b2c64f6b，独立分支叠加 PR #108。渠道页 `fetchModels` 对空集合执行 `null.sort`，显示技术异常。真实 `relay.ListModelsForAdmin` 成功协议为 `{object:"list", data:...}`，没有必需的 `success:true`；无价格条目时 nil slice 编码为 null。

候选仅将 null/undefined 模型集合规范为零选项，保留供应商/模型ID排序和选项映射；明确 `success === false` 时显示业务消息并返回，不把失败吞为成功。没有修改后端响应、价格、模型许可、渠道提交或上游调用。`pricing data not found` 是另一接口的现有业务提示，本项保留，不宣称所有空数据提示已关闭。

## 验证

- 基线真实 localhost Gin/SQLite 浏览器截图已有 null.sort 提示。新增组件回归使用实际 object:list 协议，旧版两个失败（null空集合、业务失败消息），三个控制通过（空数组、非空排序、网络拒绝）。初版同义回归使用通用success包装，之后按后端实际协议重跑，旧版仍3通过/2失败；有效证据为后者。
- 候选五项专项、196项完整Vitest与依赖边界、零警告lint、Prettier和生产构建通过。Node22.20.0/Yarn1.22.22；保留既有大chunk提示。
- 一次复用上下文、非fresh的独立只读审阅，无具体发现；独立五项专项、目标ESLint零警告与格式通过。未独立完整测试/build/浏览器。测试隔离API和编辑器，不代表真实拦截器错误链已全覆盖。
- 真实同源Gin/SQLite与生产前端，1440浅色/390深色两组实际返回 `{data:null,object:"list"}`，无null.sort提示，仍显示空定价提示。正常打开创建对话框、输入自定义模型，实际POST一次，刷新及只读接口确认模型持久化。未调用任何模型或外部服务。
- 0 pageerror，保留匿名初始401两条控制台错误；移动深色截图已核对，仅空定价业务提示仍在。随机localhost、合成管理员与临时数据库、后端拒绝外部HTTP、浏览器外部请求拦截和本地图标替身。夹具95.13秒主动结束PASS，五分钟上限；浏览器关闭。

未改变其余初始化请求生命周期；畸形响应和既有拦截器返回undefined时的额外错误提示、跨浏览器、真实移动输入及完整页面验收不在本项结论中。回滚恢复原模型列表处理，无数据迁移。本项准确候选CI、PR、合并尚待完成，不能复用前置CI。未创建标签、发布镜像、部署或执行真实付费调用。

## 2026-10-01 合并核对

[PR #109](https://github.com/dreamvm/one-hub/pull/109) head abd750d47dcabddf8c0083b2a16a84cb7e5aea6b，
[Compatibility36896955878](https://github.com/dreamvm/one-hub/actions/runs/36896955878)及
[Isolated36896972595](https://github.com/dreamvm/one-hub/actions/runs/36896972595)十项准确手动候选检查成功。
两架构实际checkout该head，各89项业务/升级PASS（SQLite12、MySQL35、PostgreSQL35、升级7）、
九波并发及四种Compose通过。最终程序均Go1.25.14、main=one-api、Linux对应架构、CGO=1：

| 架构 | image ID sha256 | 程序 SHA256 |
|---|---|---|
| amd64 | fb740cd789cd2a0678737f3861c502cd5fcef326f4252485948e9d5be4a250c6 | 06b5638a368407f435fdecaabeda028fc386c93c2084e92c3e662c757723e669 |
| arm64 | b6370ee092be15bc0740131508bf2d62de04c7f564572afcb602e9f4bd0e57dd | 5acdf1f9e80fbfeea0d0df56db23624f3cc15dd9ccd142d4198161ade4acb357 |

候选与最新main e60ed46c计算的合并树均60e5eb622c1923476c5357aab00b8106fbe9ed43。
GitHub合成12f1bda7仍保留旧父节点b2c64f6b/abd750d4，树相同；没有把旧合成父节点写成已刷新。
实际合并e7dc28f7d28d6b14283b3355822cc719a7385e74的父节点为e60ed46c/abd750d4，实际树一致。
[main CI36899618190](https://github.com/dreamvm/one-hub/actions/runs/36899618190)成功；前述本项待交付状态由此更新，其他范围限制继续有效。
