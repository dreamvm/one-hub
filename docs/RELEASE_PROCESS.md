# Fork 修复与发布流程

适用于本 Fork 后续维护。既有协议兼容说明见 [FORK_MAINTENANCE.md](FORK_MAINTENANCE.md)，
密钥与提交保护见 [PRIVACY_GUARDRAILS.md](PRIVACY_GUARDRAILS.md)。
当前版本计划与进度以 [NEXT_RELEASE.md](NEXT_RELEASE.md) 为准。

## 版本规则

- 保留上游基线 `v0.14.27`；本 Fork 的正式目标版本为 `v0.14.27-dreamvm.1`。
- 下一个候选标签预定为 `v0.14.27-dreamvm.1-rc.6`。RC 编号只递增，已发布标签不移动、不复用。
- 计划号不等于 Git 标签、GitHub Release、镜像或生产版本；这四项分别记录状态。
- `VERSION` 由现有镜像工作流在构建副本中注入实际标签；不提前把计划版本写成已发布版本。
- 每次打标签前再次核对远端标签。若编号已占用，递增并更新版本计划，不能覆盖旧标签。
- 正式版必须对应所有阻断项已关闭且已验收的提交；新代码进入候选后，重新测试并分配新 RC。
  即便源码相同，正式标签生成的新镜像也要核对版本与 digest，不能把 RC 镜像摘要直接当成正式镜像。

## 每项修复的流程

1. **界定问题**：记录触发条件、源码位置、受影响行为、正常行为和验收标准；未证实的问题明确标为待核实。
2. **建立分支**：从已核实的基线建立 `codex/<topic>` 分支，每项独立修复单独提交和 PR。
   需要前置修复时明确依赖，可使用叠加分支；合并前更新基线并重新验证。
3. **先复现**：使用虚构数据、本地 HTTP 模拟服务和临时数据库，证明旧实现失败；同时保留正常对照。
4. **最小修复**：在共享边界修复，核对直接调用方、其他输入形式及可选模式；不顺带重构其他模块。
5. **验证**：格式/编译 → 原问题及替代输入 → 正常对照 → 相关包回归 → 仓库 CI。
   测试缺少工具或失败时标记阻塞，不能记为通过。安全修复增加一次独立的绕过与回归审阅。
6. **交付审阅**：提交说明包含问题与新行为、测试结果、兼容性影响、回滚方式及剩余不确定性。
   公共 PR 不包含真实凭据、生产数据、内部地址、完整攻击载荷或未脱敏日志。
7. **合并与更新台账**：仅在相关检查通过后进入合并步骤；记录最终提交和 CI 链接，再更新版本计划。

本次用户请求已授权本地流程建设、源码修复及隔离测试；当前未完成范围内不重复索取这些授权。
合并、创建远端标签、镜像发布、生产部署和真实付费模型验收分别核对对应授权；
已有明确且未撤回的同项授权继续有效，不能把历史已完成发布的授权用于新版本。
只有依赖缺失输入或批准的步骤等待，其余已授权步骤继续推进。

## 检查层级

| 层级 | 必需内容 | 通过证明 |
|---|---|---|
| 本地修复 | 新增失败用例、正常对照、相关包 race 测试、vet、格式检查 | 命令、结果、检查的提交或未提交 diff |
| PR | `Gemini compatibility`：privacy、frontend、regression；`Isolated image smoke` | 候选 SHA 对应的成功运行链接 |
| RC | SQLite、MySQL + Redis、升级/回滚；受本批影响的权限/额度/支付失败路径 | 同一候选源码和镜像的验收记录 |
| 正式版 | 全部发布阻断项关闭；PostgreSQL、Redis 故障、依赖扫描及客户端验收 | 已审阅的候选记录与新镜像身份核对 |
| 生产 | 镜像 digest、配置差异、备份校验、维护窗口、回滚触发条件 | 实际运行版本、核心请求与恢复点验证 |

前端有既存 warning 时区分旧问题和本批新增，不把“lint 零错误”写成“零警告”。
生产检查必须另外获取真实证据，不能从标签、Compose 配置或 CI 成功推断已上线。

## 安全的本地验证入口

Go 使用与 CI 一致的 `1.25.x`，前端 Node 22 / Yarn 1；锁文件不因普通修复自动更新。
以下测试使用隔离数据，不要替换成未经准备的 `go test ./...`：

```sh
go test -race -count=1 ./model ./types ./providers/gemini ./providers/claude ./common/requester
go test -count=1 ./.github/tests
go test -race -count=1 ./.github/smoke/...
go vet ./types ./providers/gemini ./providers/claude ./common/requester
go build ./providers/... ./relay/...
python3 -m unittest discover -s .github/smoke -p 'test_*.py'
export GITLEAKS_BIN="$(git config --path --get onehub.gitleaksPath)"
python3 -m unittest discover -s .github/security -p 'test_*.py'
python3 .github/security/check_secrets.py --staged
```

每项修复还须运行它的所属包和新增回归；新增纯离线测试包后纳入 CI。
隐私检查需先按隐私文档配置固定 Gitleaks；真实 Docker 上下文测试在隔离 CI 中执行。
隐私 unittest 会创建临时 Git 仓库，必须显式传入 `GITLEAKS_BIN`；仅配置当前仓库路径不足以让临时仓库继承。
如果安装方式仅使用环境变量，则保留已有的绝对路径，不执行上面从 Git 配置读取的赋值。
前端变更执行 `cd web && yarn install --frozen-lockfile --non-interactive && yarn test && yarn lint && yarn build`。
工作流变更执行 `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12`。
构建可使用 `task build`，但目前会执行 `go mod tidy` 且可能复用既存前端产物，构建后需检查差异；
其改造列入后续工程批次。`task docker` 含旧的直接推送行为，不作为维护版发布入口。

## 标签、镜像与上线

1. 列出准备发布的最终 SHA、修复清单、CI、隔离验收、已知限制及回滚方案。
2. 核对合并和标签授权，在已审阅且已合并的提交上创建不可变候选标签。
3. 手动执行 `Manual GHCR image`，先 `publish=false`。镜像目标为 `ghcr.io/dreamvm/one-hub`。
   此构建不会导出可下载镜像，运行验收使用隔离 smoke 构建的同一 SHA 或另行准备的镜像。
4. 在需要的发布批准具备后，才以 `publish=true` 执行。记录源码 SHA、完整版本和实际 digest；不推 `latest`。
5. 生产部署作为独立步骤：核对实际旧版，校验备份，确定是否停写，固定镜像 digest，执行后验证运行状态。
6. 回滚先判断数据库是否兼容。恢复备份会丢弃备份后的写入，必须明确数据恢复点；不得自动以旧镜像替代数据决策。

每一步只报告实际完成状态；“已规划”“代码完成”“本地通过”“CI 通过”“已合并”“已发布”“已部署”互不替代。
