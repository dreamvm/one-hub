# PR147 合并后的普通发布准备

2026-10-15。基线为实际 main `96a83b68d53eb9c43017dabad91a019870953e23`。
本轮只完成普通维护和证据核对，没有恢复暂停的依赖安全验证，也没有生产操作。
本文件及关联台账的更新作为新候选交付，合并仍须专项批准；不预写新候选 CI 结果。

## 实际合并与 CI

[PR147](https://github.com/dreamvm/one-hub/pull/147) 批准 head 为
`95a001d7d9289b8c8de5afd7bd7a67df6562c9dd`。合并前重核 3 提交/7 文件/+148/-7，
独立非作者最终审阅和十项准确候选检查，范围未变。规范 diff SHA256 为
`dda87b64d81a60a705563ac36e1d8a710b98d2e4c37d3378be9306fb1ee49726`。

实际 merge 的 parents 为 `d83f6f0ba0f80a1b3acf11789f2a954e35b51b16` 与批准 head；
tree `093007904d424f6dfe0caaa67e837b870a7c3816` 与批准候选及受测合成
`47cf6af0026be0a5831e07a79d7990af81f638fb` 相同。
候选运行 38700556571 / 38700556480 的十项成功与实际 main CI 分开记录，均为 attempt 1。

| 实际 main 检查 | 运行及证据 |
| --- | --- |
| privacy | [job116333316247](https://github.com/dreamvm/one-hub/actions/runs/38708363722/job/116333316247)，成功 |
| regression | [job116333316695](https://github.com/dreamvm/one-hub/actions/runs/38708363722/job/116333316695)，成功 |
| quota-databases | [job116333316820](https://github.com/dreamvm/one-hub/actions/runs/38708363722/job/116333316820)，968 个 PASS 节点，零失败/跳过 |
| frontend | [job116333316903](https://github.com/dreamvm/one-hub/actions/runs/38708363722/job/116333316903)，26 组/234 项成功；既有警告保留 |

[运行38708363722](https://github.com/dreamvm/one-hub/actions/runs/38708363722) 为实际 merge
触发的 push，attempt 1 完成成功；四份原始日志都确认 checkout 为实际 merge。
没有将取消、旧 main 或被替代运行计为通过。main 工作流不重建原生镜像，
PR147 双原生架构各 92 项 smoke 及程序身份仍绑定批准 head，不冒称 main 重新构建的镜像。

## 实际 main 的完整普通构建

在上述 merge 的干净链接工作树上，固定 Task3.53.1 / Go1.26.9 执行完整 `task build`。
首轮前端成功，但 Go 获取 VCS status 退出128：此工具链没有把链接工作树的 `.git`
文件识别为仓库根，trace 在外层工作区执行 `git status`，外层不能提供有效仓库状态。
这是已取得的元数据故障证据，不推断为代理故障、安全验证拦截或普通编译错误。
首轮 exit201 日志保留，SHA256 为
`18286d73bc1c8ecd82c0691cd4ac7da5f889b4602814f42dc87ebd076fc97540`。

随后仅在本次普通构建命令中使用 Go 的显式元数据选项：

```sh
GOFLAGS="${GOFLAGS:+$GOFLAGS }-buildvcs=false" task build
```

命令退出0，前后工作树干净，`go.mod`、`go.sum`、`web/package.json`、`web/yarn.lock`
逐文件 SHA256 不变。使用同清单的既有前端依赖树，仍执行 frozen install 和完整前端构建，
没有把它称为全新依赖安装。没有全局修改 Go 配置、Taskfile、平台策略、代理或 TLS，
也未设置放宽多 VCS 检查的 GODEBUG。默认构建在此布局的失败没有改记为通过。

| 程序身份 | 实际结果 |
| --- | --- |
| 源码 / tree | 上述实际 main / `093007904d424f6dfe0caaa67e837b870a7c3816` |
| 编译器 / 主包 / 平台 / CGO | Go1.26.9 / `one-api` / linux-amd64 / 1，直接读取最终程序 build info |
| 程序大小 / SHA256 | 73,362,664 bytes / `d2ea90b70d72a16d2282cdcc535f1a2f8eebe989e878fee94e1e7efbef6b5f66` |
| 成功日志 SHA256 | `d8a02eb88b495d051b66870d14fa54597e074980633e59bcb67e1ac264da079e` |
| build info SHA256 | `82e8d450646e7ec1c97a35592dca3032a47f0c24961430ceef24521db3c698e4` |

该程序未嵌入自动 VCS revision；源码身份由独立 Git/head/tree/依赖哈希记录对应，
不能称为带自动 VCS stamping 的默认构建。程序未启动，未封装或发布镜像，
也不是 RC8/正式标签产物。既有大 chunk、peer dependency 和前端测试警告不宣称消失。

## 已有证据与仍缺的门槛

| 项目 | 当前证据与下一步边界 |
| --- | --- |
| 普通端口维护和资源恢复 | PR147 已合并、实际 main CI 成功；[环境恢复记录](CI_QUOTA_SERVICE_PORTS.md) 保留 EPERM/ENOSPC/首轮端口失败、精确清理与释放空间。本轮没有新增清理或刻意占端口。 |
| 浏览器普通流程 | PR146 准确 head `f53b5c429a16412e4b221954af52a0c2dab7a4b4` 已有 18 项记录，包含实际主题按钮、深色模式刷新和390px页面；本轮只读取既有记录，未重跑。应用前端未改，证据来源 SHA 不改写为当前 main。 |
| 浏览器覆盖缺口 | 原生 cache/默认 web 限额180的限定导航通过，不是容量验收；冷加载429历史保留，不主动重放。Firefox/Safari、Windows、真实移动设备、完整键盘/无障碍和全页面仍未全面验收。 |
| #68/#71历史审阅 | [既有补验](POST145_ACCEPTANCE_20261015.md#历史6871动态补验) 已记录原 base/candidate 的20/15叶子限定测试；本轮未重跑。不是历史完整 frozen 应用构建、Windows/符号链接全覆盖或原专项后台审阅成功。原中断和单项例外保留；#78/#79仍按原记录限制，不重复合并。 |
| 剩余依赖安全事项 | 安全验证暂停；既有前端107路径/39 GHSA、Go19模块/10包/5调用等仅为历史匹配快照，不能用路径数替代漏洞数，也不等于已修补、已验证或风险已接受。不恢复 YAML 等受阻验证。 |
| 最终 RC 源码及新镜像 | 本轮只读查询远端14条 tag refs；RC8及正式目标标签不存在，RC7仍指向 `d0f180451f770a7b01f41e52b4e14aeeb64dd924`。编号不是发布批准，创建前必须重新核实；最终标签和新镜像身份仍待对应阶段。 |
| 生产事实 | 由本地在原服务器私有记录核实实际身份、最新恢复点及对应恢复证明、凭据轮换和旧值撤销、账务/OIDC边界、维护窗口和回退约束。云端不索取原件或副本，公共PR不载私有核验摘要；见[职责和模板](PRODUCTION_READONLY_RECHECK.md)。 |

手动 GHCR 工作流要求已有且位于 main 历史上的版本标签；本轮没有为测试创建标签、
调度旧标签冒充新 main，或触发发布。后续先确定准确已审阅源码、架构、已关闭项与
未关闭项，再分别核对新PR合并、标签、`publish=false` 构建、镜像发布和生产变更授权。
生产窗口方案须明确恢复点、允许丢失的写入范围、责任人、超时/取消及回退触发条件；
仅镜像回退与完整恢复分别决策，不能用合成演练或CI替代真实恢复点与旧版本约束。
本轮未部署、轮换、重启/升级生产或真实付费，整体发布门槛继续开放。
