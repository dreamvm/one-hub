# source-map-js 索引映射依赖维护

2026-10-13，从已合并 PR134 的 main `37ba502fde7c5b7bcd79f4e7f0d8f70a2f2ee0fa` 接续。
本项是开发/构建/测试依赖维护候选，不声称已证实生产业务接口可远程利用。

## 官方公告与三条独立路径

对应 [GHSA-68fv-2mgg-jv7q / CVE-2026-93749](https://github.com/advisories/GHSA-68fv-2mgg-jv7q)，
官方影响区间 `>=1.0.0,<1.2.2`，修补版本为 1.2.2。
公告涉及 indexed source map 的 section offset 引起同步资源放大。
[实际上游补丁 cf765805](https://github.com/7rulnik/source-map-js/commit/cf7658058ceeaa8619d5ae0ec90be6905209d016)
还修正 nested `sources` getter 重复计算，并避免 SourceNode 在代码耗尽后继续逐行填充、
以及 generator 用逐次拼接构造大行间隔。不得仅依据 SourceNode 名称漏掉消费者的其他调用。

静态阶段先读项目流程及 Babel、浏览器、PostCSS、Vite 与相关依赖记录，未运行目标库或修改源码。
当前未找到仓库根或 web 范围的 SECURITY.md，范围依据实际脚本、配置、消费者和 Dockerfile。
安全 Skill 的共享 security-guidance/static-finding-assessment/artifact-storage 引用在本环境返回
`failed to read skill resource`，因此未声称使用过其政策解析器；这是流程工具缺口，不能用本记录
替代完整 Skill 工具链或缺失的历史证据。直接 Skill 和 triage 结果契约可读。

| 原独立路径 | 基线实际版本 | 静态结果与最小缺口 |
| --- | --- | --- |
| Sass → source-map-js | Sass 1.72.0 → 1.2.0 | 当前受检入口 not_actionable，高静态置信度：Sass 的 Node 加载表没有目标库，Dart 实现内部生成 map，未发现 SourceMapConsumer/Generator/Node 调用；不是库已安全或任意 Sass API 的结论 |
| Vite → PostCSS → source-map-js | Vite 8.3.1 → PostCSS 8.5.28 → 1.2.1 | needs_review，中等置信度、队列1：实际 previous map consumer 存在；可提供依赖/工程 map 的主体、权限和运行实例暴露范围未证实 |
| Vitest → Vite → PostCSS → source-map-js | Vitest 4.1.11 → 同一 Vite/PostCSS → 1.2.1 | needs_review，中等置信度、队列2：Vitest 使用 Vite peer 与合并配置；具体样式输入及 consumer 激活条件仍须分别核实 |

后两条共用物理副本，仍保留两条输入。Sass 依赖声明成立，但不把它写成实际 consumer 调用。
Vite 的 CSS Modules、import、URL 或配置/parser 激活 PostCSS；普通 CSS 可直接跳过。
PostCSS `PreviousMap` 接受 inline base64/URI、外部 map、显式字符串/对象及 consumer/generator。
`Input` 构造 consumer，`MapGenerator.applyPrevMaps` 再调用 `applySourceMap`，后者访问 indexed
consumer.sources；无插件的库 API 还可进入 `SourceMapGenerator.fromSourceMap`，不能把该可选
库入口自动当作当前 Vite 默认模式。外部 map 的文件位置控制不限制 inline sections/offset。
当前生产 Go 镜像只嵌入静态前端，不运行上述编译器；未发现业务上传 API 将 map 送入此管线。
这既不是生产漏洞确认，也不以“本地文件”或“开发依赖”标签关闭调用和来源缺口。

独立预修复调查只读核对了这些调用及合法 flat/indexed/SCSS 兼容要求，没有运行测试或 PoC。
历史 `artifacts/dependency-refresh-20261008/triage.json` 未在云端找到，也未取得原聊天读取工具，
不能宣称已读该文件。交接所报 173→178、新增三个公告及 DOMPurify/sprintf-js 的静态结论
仅作历史信息；本轮官方公告重新获取，审计和专项结果另记。

## 最小候选及有界旧新对照

只将两个现有 lock selector `>=0.6.2 <2.0.0`、`^1.2.1` 统一到官方 1.2.2；该版本满足
Sass/PostCSS 已有范围，无新增直接依赖或 resolutions，不升级父库、业务源码或 docs 站点锁文件。
官方 npm tarball 的 SHA512 已校验，安装副本全部18个普通文件与该官方包逐字节一致。

新增 `web/tests/source-map-js-boundary.test.mjs` 接入 `test:deps`，通过真实父包解析保留三条路径：

- 非整数/非有限/负数及非数值 offset 在 consumer 构造时拒绝；大 line 与嵌套累计越界也只构造，
  不遍历或序列化大偏移，不执行耗尽或真实停服。
- 三层小 map 的 sources getter 用底层读取次数证明重复计算，未用长时间压测作判据。
- SourceNode 使用64行偏移及三行代码，证明耗尽代码后不再拼入额外内容。
- flat 和小的合法 ordered indexed/nested map 保留来源、内容和位置；flat generator 回写正常。
- PostCSS 真实 Input 分别处理 base64/URI inline map，非法 offset 拒绝、合法源内容保持。

每个父测试5秒、专项外层45秒，只有内存合成数据。SourceNode 和三条独立库 API 对照是
库维护证据，不能当作三条项目入口都可被同一业务攻击触发。
最终专项在基线25个叶子中20失败、5正常通过；候选29个Node节点（含4个父节点）全部通过。
初版正常 indexed generator 对照碰到库已有 `sourceRoot` 缺失异常，未把它记作本公告复现；
改用合法 indexed consumer 对照和 flat generator 回写后重跑旧版，初始日志保留。
该既存 indexed generator 能力限制没有由本候选宣称修复。

frozen 安装、完整前端回归、lint（零ESLint诊断）和生产 build 已通过；安装 peer 和构建大 chunk 提示保留。
完整回归为26文件/234项组件、dev Node47节点、deps Node117节点，均零失败/跳过。
使用单worker、单组件测试15秒、总600秒上限，实际约116秒通过。
一次 fresh 非作者只读候选审阅未发现具体阻断问题；审阅者独立执行29节点专项和普通SCSS/map
对照通过，并核对了旧版失败判据。生成器行间隔优化有官方源码依据，专项没有独立复杂度断言，
不声称该优化已动态完成耗尽验证。准确提交 CI 和最终 PR 状态将在取得结果后补记。
不复用 PR134 或此前恢复验收的绿色结果替代本候选。

## 2026-10-13准确官方审计

使用公开仓库两份隔离的 manifest/lock 副本，关闭默认rc读取，没有运行安装或生命周期脚本。
Yarn1源码的审计树仅包含公开包名、版本、完整性及依赖关系、dev标志和空install/remove/metadata；
官方端点固定为 `registry.yarnpkg.com/-/npm/v1/security/audits`，不发送源码、运行配置、凭据、
脚本或resolved下载URL。未在这次运行额外捕获发送体，发送字段依据当前Yarn1.22.22源码核对。

准确基线178条、候选175条；两者各有一个汇总、零error，退出30是严重性位掩码，不是执行故障。
以advisory ID/包/路径/版本区间比较，仅移除本公告三条 source-map-js 路径、零新增。
原审计ID `1241209`、三个独立路径及官方URL已重新取得；未用原聊天未迁移的triage文件代替。
其余175条匹配仍保留，不能称审计清零，也没有在本轮重做DOMPurify/sprintf-js或全部库的triage。
本结果只证明候选依赖图不再匹配此公告，不能替代可利用性、独立审阅、CI、合并或生产验收。

## 回滚及继续保留的门槛

回滚以本项反向提交并执行 frozen 安装和回归，会恢复受影响的库版本。
上游仍允许至一千万行的有效 section offset；1.2.2 不是任意大小或任意深度 map 的资源沙箱，
本项没有证明所有资源消耗受严格预算约束。
YAML v3/matcher 已记录的候选残留、受限审阅与单项例外、Realtime #78/#79、历史账务、
生产事实、新恢复点及最终候选双架构/页面/升级验收继续保留。RC8仍预留，未创建标签、
发布镜像、部署或调用真实付费服务。新PR合并须维护者明确批准。
