# 隐私与密钥防误提交

这些措施用于降低误提交风险，不改变应用运行、用户权限、计费或发布开关。
公开仓库的 CI 在推送后运行，不能收回已公开的密钥；本地提交前检查和私有配置分离同样重要。

## 每个克隆单独启用

Git 身份和 hooks 不随 clone 自动启用。先检查现有 `core.hooksPath` 与 `.git/hooks/pre-commit`；
如果已有自定义 hook，请合并执行逻辑，不要直接覆盖。当前受维护工作流采用：

```sh
# 使用 GitHub Settings → Emails 显示的个人 noreply 地址，不填本机 .local 地址。
git config --local user.name 'YOUR_PUBLIC_NAME'
git config --local user.email 'YOUR_GITHUB_NOREPLY_ADDRESS'
git config --local user.useConfigOnly true

# 安装官方 Gitleaks 8.30.1 并校验后，指定绝对路径。
git config --local onehub.gitleaksPath '/absolute/path/to/gitleaks'
git config --local core.hooksPath .githooks
python3 .github/security/check_secrets.py --staged
```

只修改本仓库配置，不修改全局 Git 设置。环境变量、`--author`、`git -c` 等仍可覆盖身份；
提交前可用 `git var GIT_AUTHOR_IDENT` 和 `git var GIT_COMMITTER_IDENT` 核实。
带注释标签也应使用隐私身份。更改配置不会修改旧提交、旧标签、PR 或旧构建元数据。

固定版本官方下载：`https://github.com/gitleaks/gitleaks/releases/tag/v8.30.1`。
本批验证的归档 SHA-256：

| 文件 | SHA-256 |
|---|---|
| `gitleaks_8.30.1_linux_x64.tar.gz` | `551f6fc83ea457d62a0d98237cbad105af8d557003051f41f3e7ca7b3f2470eb` |
| `gitleaks_8.30.1_darwin_arm64.tar.gz` | `b40ab0ae55c505963e365f271a8d3846efbc170aa17f2607f13df610a9aeb6a5` |

其他平台应自行核验对应官方校验清单；不要拿上述哈希校验不同平台文件。
本地也可通过 `GITLEAKS_BIN` 显式指定固定版本，缺失、版本不符或运行失败均阻止提交。

## 检查边界

- 提交前读取 Git **暂存区**新增、修改、复制、重命名或类型变化文件的完整 blob，在私有临时目录扫描。
  不依赖当前工作区内容；文件已暂存密钥后仅修改工作区不能绕过。重命名保留原内容也会扫描。
- 临时目录结束后删除；符号链接按链接文本扫描，不读取链接目标。子模块不被当作普通 blob
  静默放行，新增/修改子模块需要独立审计和策略支持。
- 使用固定版本的默认规则，忽略 `gitleaks:allow` 注释；不允许环境变量或未暂存的配置替换扫描策略。
- 输出仅报告通过/阻断，不输出原始命中或候选密钥，失败包括发现密钥和扫描器错误。
- 如需定位，请在安全的本地终端使用该固定版本 Gitleaks，始终加 `--redact=100`，
  不把原始配置、完整扫描结果或密钥贴入公开 Issue/PR。
- CI 的 `privacy` job 在原有可复用兼容性工作流中执行全历史扫描，`fetch-depth: 0`，
  使用调用方指定的确切提交。该工作流仍是手动镜像发布和隔离 smoke 的前置条件；失败不发布。
- `.gitleaksignore` 仅列出审阅过的六个上游历史文档命中指纹，精确到提交、文件、规则和行号。
  它们早于本 Fork 维护，不代表在线有效性已验证。没有整个文档目录、测试目录或 `sk-*` 豁免。
  新提交同一路径的密钥仍会阻断；本地暂存文件扫描不使用这份历史例外。
- hooks 可被 `--no-verify`、更换配置或其他客户端绕过；CI 也不能替代托管平台的分支保护。
  本批不改变 GitHub 分支保护或仓库/包可见性。

验证命令：

```sh
GITLEAKS_BIN=/absolute/path/to/gitleaks python3 -m unittest discover -s .github/security -p 'test_*.py'
GITLEAKS_BIN=/absolute/path/to/gitleaks python3 .github/security/check_secrets.py --history
# 在具备 Docker 的隔离机器或 CI 中加入真实上下文导出验证：
GITLEAKS_BIN=/absolute/path/to/gitleaks ONEHUB_DOCKER_TEST=1 python3 -m unittest discover -s .github/security -p 'test_*.py'
```

测试只生成虚构密钥和临时 Git 仓库，不使用生产 API，不推送测试提交。

## Docker 上下文

根 `.dockerignore` 排除 Git 元数据、环境文件、运行配置、数据库/日志/备份、私钥格式、
编辑器/代理状态、依赖缓存和本机生成物。保留构建所需的 Go/Lua/图片资源、前端源文件、
`config.example.yaml`、`VERSION` 和 `Dockerfile-action` 所需的 `one-api-amd64` / `one-api-arm64`。
不再把本机 `web/build` 带入构建，前端产物由 Docker 内部构建步骤生成。

CI 使用 Docker 实际导出一次未保护的合成上下文和一次启用规则的上下文，验证敏感标记文件
从可复制变成不可复制，同时保留必要输入；现有完整镜像构建与 SQLite/MySQL/Redis smoke 继续运行。

排除名单不是内容安全沙箱：任意命名的密钥仍可能被复制。不要把生产数据放进源码目录，
也不要通过 build args 传密钥；新增 Dockerfile 专属 ignore 文件或改变 context 根目录时应重新审计。
本批不删除旧镜像/缓存，不自动发布、升级生产或改写历史。
