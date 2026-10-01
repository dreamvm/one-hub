# i18n 维护范围

2026-10-01，维护者委托选择合适方案后，本项目保留现有单文件 JSON 翻译流程。
这份约定规定今后的受支持用法，不证明历史上从未使用其他入口，也不改变依赖库本身的漏洞状态。

## 支持的入口

在 `web/` 中使用项目规定的 Node 22 / Yarn 1，固定使用经审阅的 `.i18nrc.js`：

```sh
DEV=false yarn i18n --config .i18nrc.js
```

源文件为 `src/i18n/locales/zh_CN.json`，目标为 en_US、ja_JP、zh_HK。
配置和输出目录由维护者审阅，不接受语言文件或模型返回值指定配置、路径或执行模式。
生成的译文仍是需要检查的数据，不能因为来自模型或本地文件就认定可信。
现有四份语言文件不含数组值；旧工具的数组缩短控制失败仍见[调查记录](I18N_TOOLCHAIN_TRIAGE.md)，新增数组前须处理兼容问题。

显式 `DEV=false` 避免继承 shell 的调试开关；受检 dotenv 默认不覆盖已有环境变量。
固定 `--config` 避免向上查找其他配置。这个命令仍会调用翻译模型，不是离线验证入口；
本次没有执行它，真实付费调用与外发仍按已有专项授权规则处理。

## 暂不支持的入口

- `md`、`--with-md`、Markdown 前言解析。
- 目录或 glob 模式、其他配置文件、动态输入路径。
- `DEV=true`、React DevTools 连接及为此补装可选 peer。

这些功能仍存在于依赖中，本约定不是技术沙箱，也没有修改 CLI 来阻止直接调用。
启用任何一项前，先重新核实输入来源和边界，完成对应依赖的最小修复、正常控制、有界旧新验证、
独立审阅和准确候选 CI，再更新本约定。不得把“未纳入支持范围”写成“依赖已修复”。

## 当前证据范围

锁定 CLI 1.20.3、gray-matter 4.0.3、js-yaml 3.14.1。固定 JSON 路径进入
`genFlatQuery`；目录/Markdown 才构造 glob。JSON 内容不转成模式，也不进入 gray-matter。
Ink 仅在 `DEV === 'true'` 时加载调试模块，因此明确关闭 DEV 的入口不经过这条 ws 调试路径。

按上述维护范围，原待核实的20条可选路径列为当前入口 `not_actionable`，中等置信度；
逐条编号见[后续分流记录](I18N_TOOLCHAIN_TRIAGE.md#2026-10-01-固定-json-维护范围)。
这不是全库安全、全部依赖审计清零、实际翻译兼容或生产验收。没有读取真实凭据、环境配置或运行耗尽测试。

一次未参与编写、复用上下文且非fresh的独立只读文档审阅无具体发现；独立核对命令、
CLI分支、Ink及dotenv语义和范围表述。未重新核实全部公告版本或私有逐条JSON，
没有执行CLI、测试或网络，不把这次文档审阅当成每项依赖的新安全验证。
## 2026-10-01 合并验收

- [PR #110](https://github.com/dreamvm/one-hub/pull/110) 候选 `d4f48aacb1b76d0c044aea89c608e51aa177aefe` 的
  [Compatibility](https://github.com/dreamvm/one-hub/actions/runs/36900349924) 与
  [Isolated](https://github.com/dreamvm/one-hub/actions/runs/36900350920) 共十项检查通过。
  Compatibility 前端实际检出合成提交 `9cb758ca1ac0e58aea777f075d8f60990c8812a7`，
  双架构 smoke 检出候选；候选、合成、最新 main 计算及实际合并树均为
  `6fba6f37f50169b851d80907f9913125093d2126`。
- 实际合并 `da936380c20e89e82006d69e4d69e10279d5c4f3`，父节点为 `e7dc28f7` 与 `d4f48aac`。
  [合并后 main CI](https://github.com/dreamvm/one-hub/actions/runs/36902867961) 尚待核对。
- 每个架构各 89 项业务/升级 PASS、九轮并发及四种 Compose 通过。
  最终 `/one-api` 均确认 Go 1.25.14、main=one-api、Linux、对应架构及 CGO=1。

| 架构 | 本地 CI image ID (sha256) | 最终程序 SHA-256 |
|---|---|---|
| amd64 | d64660274aa3bef721fcfc14d6c30ac0622f41d6754db22bb5256a24d127dfe4 | b70bf1d2ef9717e61555bd2b561c9bbc8d2e2c9f0e19ca643e18f05501b4b889 |
| arm64 | 826e51c47b14748f7da20b2b5433d371787762a3a72ca7b2773632091888b077 | edf075f8f64c1e356dc73e130e6e80929662d130619ce279c9ffc1d6e9c52bd2 |

本次镜像仅用于隔离验收；未发布、部署或执行真实翻译。上述结果不改变依赖本身未修复的边界。
