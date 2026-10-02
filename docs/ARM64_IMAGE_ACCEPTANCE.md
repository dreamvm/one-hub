# 原生 arm64 隔离镜像验收

2026-10-01 本地候选，基于语言图标候选 f33f9706ef2aeb656b2c01bb0585f6ee75032f97；尚无本项准确候选 CI、PR、实际 arm64 容器验收或合并。既有 amd64 结果不能替代本项。

现有 smoke 只在 amd64 上编译模拟上游、构建候选、拉取 fixture 并校验 binary；Compose 安装脚本也缺少 Linux arm64 分支。新增原生 ubuntu-24.04/ubuntu-24.04-arm 矩阵，架构统一传递给模拟上游 GOARCH、Buildx 平台、不可变 fixture 拉取与最终 binary 检查；拉取后额外核对候选和 fixture 实际 Architecture。两架构分别执行原有三数据库、故障/并发业务、升级及两条回滚、四种 Compose 模板验收，fail-fast=false 避免一边失败取消另一边证据。job 和各 step 既有超时不变。

[GitHub 官方 runner 表](https://docs.github.com/en/actions/reference/runners/github-hosted-runners)确认原生 arm64 标签；不是 QEMU 推断。Compose 5.5.1 官方 linux-aarch64 资产 SHA256 为 732e3a84c1a0f67256ce80bc2598a24546b10ca05f9faa97efceb1171ece2ef7，见[官方 release](https://github.com/docker/compose/releases/tag/v5.5.1)。安装仍仅写调用方临时目录、校验后才执行；已有平台不变。

通过 Docker 官方 registry 只读核实当前固定索引均含 Linux arm64：旧 v0.14.27 子清单 6d413eebf7de0741c88eaeb9ed398309b93786f162b4ac330182365a28c87dac；MySQL d4c783e85ef8647a4350cb7aa7a8ddc7ebfd7cb217de4c2d1643e212ba94fcbd；PostgreSQL 5d1822a7565aecbf17323dd54754ab8b832963e62b9512f465c2f155db9262dd；Redis d27d1ec3e13fb3d1d29e0772cd56c283d2097250275df542537e586c9dac651a；Go builder f5dec0e3d7c346eb12d191a9e2e57bcc2c6ac740396a181bf0dc8ed1e6e715ee。依旧固定原有多架构索引，不升级镜像或依赖。注册表清单不等于已运行镜像。

新增架构一致性门槛在旧 workflow 失败，原禁止发布及完整数据库/回滚控制通过；候选 19 项 workflow 父测试（含子用例）、38 项 Python 离线 smoke 测试及 actionlint 通过。actionlint 初次在无 .git 的归档副本未定位项目，改为显式工作流路径后成功；不是产品故障。平台策略测试不代替真实 arm64 执行。本地无 Docker，实际运行必须由准确候选 CI 完成。一次复用上下文、非 fresh 的独立只读审阅，审阅者未参与实现，未发现功能性阻断；发现新增 Go 测试导入分组规范问题，父任务修正分组后重跑19项工作流测试与actionlint通过。该格式修正在审阅之后，无第二轮复审。独立审阅重跑Go/38项Python及shell语法通过，但其Go环境未提供Task/Compose路径，相关检查跳过，actionlint尝试受离线模块解析限制；父任务使用固定本地工具及显式路径完成这些检查。审阅补丁d71f0ebcaa347f0fb20ab2726e28928e46072b7df95d8604d0df62c1e4f41b34；最终仅导入分组修正后补丁f2ae1b9af37110a5469c3d2751a16e2bf3f297872d2dd75a0758942ba59c5a37。

工作流仍仅 contents:read、无 registry 登录、push=false、load=true，无发布凭据、标签或生产部署。新增一个原生 smoke job 会增加 CI 资源用量；前置通用回归仍执行一次。回滚本项恢复仅 amd64 验收。OIDC 旧程序回退安全、真实生产配置/备份与历史归属仍需独立处理，隔离基础升级控制不关闭这些门槛。

## 2026-10-01 合并验收

[PR #103](https://github.com/dreamvm/one-hub/pull/103) head `23b5c2639e122fe03c7bd39c2f4c4e51bd4e8307` 的手动准确 [Compatibility36874432973](https://github.com/dreamvm/one-hub/actions/runs/36874432973) 四项及 [Isolated36874440707](https://github.com/dreamvm/one-hub/actions/runs/36874440707) 六项全部成功，共十项。最初叠加基线不触发 main PR 事件，使用 workflow_dispatch；不声称自动 PR 检查已运行。

| 实际原生 runner / job | 镜像 ID（sha256） | `/one-api` SHA256 | 实际耗时 |
|---|---|---|---|
| ubuntu-24.04-arm / 110411933207 | 1b7e65cb751d5819eb74ce66384866c9ddcd9fbfc0e1e0e7d2d5794a22b5fe62 | e44368a756504843a61eca41bf6275634c2be649c1214836449bd4cde82df7a1 | 13分37秒 |
| ubuntu-24.04 / 110411933082 | 811af1b552581565d887ab5bbde54d4ef7b4a5b92ed8d1d13918044c57e93742 | d6bcb67dbe09129bad34db23e179f89f6aac0896e6f43499c8395c8029a85560 | 15分24秒 |

两架构分别读取最终镜像程序，确认 Go1.25.14、主包 one-api、Linux、对应 arm64/amd64、CGO1，并校验候选及 fixture 镜像架构。各自实际 89 个业务/升级 PASS（SQLite12、MySQL35、PostgreSQL35、升级及两种回滚7）、九波有界并发、四种 Compose 启动通过；所有 step 成功，均在35分钟 job预算内。仅在隔离 runner 构建和加载，未发布镜像；上述 image ID 不是发布仓库 digest。

前置 #102 合并后，最新 main `7860d0d3db79f7bfdb8b04587df56efe50ff4847` 的计算合并树与候选同为 `46e2b51636d1b3d40a83166ea7f12f8ad233d573`。旧合成 `0eb3669a2b1671d6212f5aa6c161f24a2b560ccc` 仍保留 f33f9706/23b5c263 父节点，树相同，不声称其祖先刷新。锁定 head 合并 `0229f0aaf4a6f00720b402467ae317f47248735e`，API核实实际父节点为最新 main/候选，实际树相同。[合并后 main CI36877499093](https://github.com/dreamvm/one-hub/actions/runs/36877499093) 四项已成功。

这关闭本项真实 arm64 隔离镜像及现有合成升级/回滚验收缺口。未创建 RC8 标签，不替代 OIDC 历史身份回退、真实生产配置、备份恢复点、历史账目及最终发布验收。
