# 原生 arm64 隔离镜像验收

2026-10-01 本地候选，基于语言图标候选 f33f9706ef2aeb656b2c01bb0585f6ee75032f97；尚无本项准确候选 CI、PR、实际 arm64 容器验收或合并。既有 amd64 结果不能替代本项。

现有 smoke 只在 amd64 上编译模拟上游、构建候选、拉取 fixture 并校验 binary；Compose 安装脚本也缺少 Linux arm64 分支。新增原生 ubuntu-24.04/ubuntu-24.04-arm 矩阵，架构统一传递给模拟上游 GOARCH、Buildx 平台、不可变 fixture 拉取与最终 binary 检查；拉取后额外核对候选和 fixture 实际 Architecture。两架构分别执行原有三数据库、故障/并发业务、升级及两条回滚、四种 Compose 模板验收，fail-fast=false 避免一边失败取消另一边证据。job 和各 step 既有超时不变。

[GitHub 官方 runner 表](https://docs.github.com/en/actions/reference/runners/github-hosted-runners)确认原生 arm64 标签；不是 QEMU 推断。Compose 5.5.1 官方 linux-aarch64 资产 SHA256 为 732e3a84c1a0f67256ce80bc2598a24546b10ca05f9faa97efceb1171ece2ef7，见[官方 release](https://github.com/docker/compose/releases/tag/v5.5.1)。安装仍仅写调用方临时目录、校验后才执行；已有平台不变。

通过 Docker 官方 registry 只读核实当前固定索引均含 Linux arm64：旧 v0.14.27 子清单 6d413eebf7de0741c88eaeb9ed398309b93786f162b4ac330182365a28c87dac；MySQL d4c783e85ef8647a4350cb7aa7a8ddc7ebfd7cb217de4c2d1643e212ba94fcbd；PostgreSQL 5d1822a7565aecbf17323dd54754ab8b832963e62b9512f465c2f155db9262dd；Redis d27d1ec3e13fb3d1d29e0772cd56c283d2097250275df542537e586c9dac651a；Go builder f5dec0e3d7c346eb12d191a9e2e57bcc2c6ac740396a181bf0dc8ed1e6e715ee。依旧固定原有多架构索引，不升级镜像或依赖。注册表清单不等于已运行镜像。

新增架构一致性门槛在旧 workflow 失败，原禁止发布及完整数据库/回滚控制通过；候选 19 项 workflow 父测试（含子用例）、38 项 Python 离线 smoke 测试及 actionlint 通过。actionlint 初次在无 .git 的归档副本未定位项目，改为显式工作流路径后成功；不是产品故障。平台策略测试不代替真实 arm64 执行。本地无 Docker，实际运行必须由准确候选 CI 完成。一次复用上下文、非 fresh 的独立只读审阅，审阅者未参与实现，未发现功能性阻断；发现新增 Go 测试导入分组规范问题，父任务修正分组后重跑19项工作流测试与actionlint通过。该格式修正在审阅之后，无第二轮复审。独立审阅重跑Go/38项Python及shell语法通过，但其Go环境未提供Task/Compose路径，相关检查跳过，actionlint尝试受离线模块解析限制；父任务使用固定本地工具及显式路径完成这些检查。审阅补丁d71f0ebcaa347f0fb20ab2726e28928e46072b7df95d8604d0df62c1e4f41b34；最终仅导入分组修正后补丁f2ae1b9af37110a5469c3d2751a16e2bf3f297872d2dd75a0758942ba59c5a37。

工作流仍仅 contents:read、无 registry 登录、push=false、load=true，无发布凭据、标签或生产部署。新增一个原生 smoke job 会增加 CI 资源用量；前置通用回归仍执行一次。回滚本项恢复仅 amd64 验收。OIDC 旧程序回退安全、真实生产配置/备份与历史归属仍需独立处理，隔离基础升级控制不关闭这些门槛。
