# gray-matter / js-yaml v3 候选审阅

2026-10-01 UTC。**运行时候选保留在 TEMP，尚未交付。**本项独立于已交付的共享 v4 #120，
未应用到维护分支，没有运行时提交、PR、准确候选 CI 或合并。
本文叠加于台账 #121；该前置须先交付，本文自己的检查不能用前置 CI 替代。

## 输入、实际路径与支持范围

基线为 `6b1b3283b4322cda7d0f3e3df0257a7ff57df10f`；CLI 1.20.3 → gray-matter 4.0.3 →
独立 js-yaml 3.14.1。五个原公告分别为
[结果原型](https://github.com/advisories/GHSA-mh29-5h37-fv8m)、
[重复别名](https://github.com/advisories/GHSA-h67p-54hq-rp68)、
[合并链](https://github.com/advisories/GHSA-52cp-r559-cp3m)、
[omap 查找](https://github.com/advisories/GHSA-5p4m-2wfm-xmqj)、
[空来源预算](https://github.com/advisories/GHSA-2883-xcg3-v3hh)。

实际 YAML 引擎绑定 v3 `safeLoad/safeDump`。可选 Markdown 原文经 `matter(raw)` 解析；
关闭 includeMatter 时，模型返回字符串在 `matter.stringify` 内先再次解析，再合并数据和序列化。
当前[维护范围](I18N_MAINTENANCE_SCOPE.md)仍为固定单文件 JSON、固定配置及 DEV=false。
JSON 语言内容和译文不进入 gray-matter；既有五条输入的项目范围结论继续保留，
不把可选库能力推断成当前 HTTP 漏洞。历史 Markdown 来源和使用情况仍未核实。

## 临时候选与本地验收

一次 fresh 只读预调查和父代理独立调用链核对后，仅将 `js-yaml@^3.13.1` 锁到官方 3.15.2，
新增 v3 专项并加入 test:deps。没有全局主版本覆盖、解析适配层或 CLI 升级；共享 v4 4.3.2
和 Vite 的 picomatch 4.0.3 保持不变，不带入未交付的 matcher 补丁。
官方[3.15.2 记录](https://github.com/nodeca/js-yaml/blob/3.15.2/CHANGELOG.md)保留 safe API，
merge 来源及其键计费、单序列上限100、累计默认10000，omap 改为 own-key 表检测。
调用方显式 schema、引擎和关闭/非有限预算选项仍是 API 能力，不能声称全部任意选项安全。

官方包完整性和36个普通文件字节均已核对，安装文件与官方包一致。
完整三文件补丁 SHA256 为 `dd694349e1dde3c418a2595fde571f43b77a850bcd12b8aacb8889e3f74af559`；
最终专项源码为 `b7150a74da2a2c193b6c216673427701f2091889f6103df9b4cb68d89aa6f6fc`。
相同最终源码在旧版11正常叶子通过、16修复断言失败；Node汇总28项含父测试，11通过/17失败。
候选27叶子、Node28项全部通过，覆盖原文解析、stringify再次解析、safe API、空来源、合并链、
默认预算、正常100来源、序列化、保留schema、omap语义、JSON和模块主版本分离。

冻结锁文件正常安装通过，既存 peer 警告保留；实际 Yarn run test 的22文件209项UI、
dev Node31及deps Node116项均通过。lint没有ESLint诊断，build成功但chunk体积警告保留。
固定合成输入小于4KiB，worker为192MiB/8秒/32KiB输出，专项90秒；普通构建4096MiB/180秒。
没有耗尽压测、真实翻译/模型请求、第三方扫描或生产数据。输入和锁文件哈希保持一致。
初始解包工具不支持filter、已完成拷贝后的包元数据路径错误和后续确认夹具路径错误均保留，
修正后重做相应设置/报告；不将失败设置写成通过。

## 一次独立审阅及确认

一次 fresh、非作者只读审阅建议 revise，原报告 SHA256 为
`a17a0888a9498afdad7ffdfd8500f9d10930e872c72f529f3a7a52f23a172d57`。
父代理使用短输入和正常控制确认：

- v3 的 overridableKeys 仍为普通对象，不能登记合并后的 own `__proto__`；随后正常显式覆盖被误判为重复键。
- gray-matter 的 `Object.assign({}, file.data, data)` 将顶层 own 同名键转成中间目标原型，safeDump丢失该键。
  原专项的嵌套数据不能覆盖这个浅拷贝边界；未证明全局 Object.prototype 污染。
- 既存调用方在解析前缓存默认对象，第一次无options解析失败后，同内容第二次返回空data和未解析内容，
  错误契约被改变；它不重新执行耗时解析，不能称为CPU预算绕过。

omap专项仅有微小语义对照及官方源码修复依据，缺少区分旧二次工作算法的失败判据，
不冒称已完成复杂度失败复现。gray-matter语言头可选择JS/custom引擎是既存可选能力，
不由这五条YAML公告关闭；未执行这些引擎，启用Markdown前仍须单独核实来源和边界。

审阅额外提出“共享v4也有同样覆盖跟踪瑕疵”的假设被父代理反证：v4的跟踪表为
Object.create(null)，v3才是普通对象。旧4.1.0和当前4.3.2经真实ESLint YAML加载器均接受
该显式覆盖，普通title对照也通过；不据此误记#120回归。原报告与反证同时保留，没有第二轮审阅。
确认日志分别为 `38b2a2d63986708288462eefbffb8f2d47a1909b463c0be425c4e7f9f551e18d` 与
`d79fd09f65d85ff71e4b1315062b366091fffa718eec47e3468192fc408634b4`。

## 精确审计与交付门槛

此未合并候选的官方Yarn复查为190路径/88Yarn ID/67GHSA，低13/中65/高110/严重2，
退出30、一个汇总、零error；与已交付main195比较，五个原v3 claim key消失、没有新增。
**main仍为195快照；190不代表已交付或审计清零。**原始匹配和此前范围缺口继续保留。
实际仅向 registry.yarnpkg.com 官方审计端点发送同937个已核实公开包的元数据；
发送前JSON289812字节、哈希 `2f03ad371fe0cc814eee67ae34545a8e426b2e94ee3a263c0ccff1a11a5f5a33`
与无网络预检一致，没有源码、凭据、内部地址、运行配置、scripts或proxy字段。
raw审计为 `7134ea0cf2c3c4f02f07ecc21ad70d15e8780de6cd46d0360aef5789200a263f`。

证据和原始候选保留于Security supplemental `artifacts/yaml-v3-candidate/`，
outcome SHA256为 `ee5c441f81ee6d4e0deb22767462c7481fb1e377211867ba102141e4dea892f9`。
该运行时候选尚不满足完整边界和兼容门槛；不强行加入依赖fork或全局主版本覆盖，
不通过第二轮审阅消除原结论。受限#68/#71/#78/#79、真实历史归属、生产事实及最终RC8门槛继续开放；
未创建标签、发布镜像、部署或真实付费调用。

## 2026-10-01 前置台账#121交付

[PR #121](https://github.com/dreamvm/one-hub/pull/121)候选b56d09174543235f554287b88156b072be6040dd的
[Compatibility](https://github.com/dreamvm/one-hub/actions/runs/36940627164)及
[Isolated smoke](https://github.com/dreamvm/one-hub/actions/runs/36940627607)十项成功，
前者四项实际checkout为合成811f3c6bf0b373fe8fc32aff1a70ab986d9a70fc，后者六项为准确候选。
候选、合成、最新main计算及实际合并树均为719275bdfa7dd10229a1c4f1f6fd2b126dcf2868。
实际合并df9b650d16b0fa2de43be365d330aa53b42e0686，
[自身main CI36942692779](https://github.com/dreamvm/one-hub/actions/runs/36942692779)成功，
四项真实完整checkout均为该合并提交。

两份候选前端与main实际YAML20叶子/Node21、22文件209项UI、lint及build成功；peer/chunk提示保留。
每个原生架构89项业务/升级PASS、九轮并发及四种Compose成功；镜像内程序Go1.25.14、
one-api、Linux/对应架构、CGO=1，不是已发布或部署的镜像。

| 架构 | CI本地image SHA256 | 最终程序SHA256 |
| --- | --- | --- |
| amd64 | 74890e8f576a5ae483e0a75165654a42802b6e8ea8fc22f3d54f8d441b46a467 | 5e183a8c590bccdcfa26070aa876e3204c448e03f1d054b2f5a5b8f2ecf4c816 |
| arm64 | 81a5b0f620ead6dd6493b1f6754184e45117736503ffc1c55c020941462ca80a | 7be9fa08ff2c8f99b20ecfca3cd38ee8d163a576d0ef26774632cb9518997473 |

候选及main共34份证据保存于artifacts/pr121-ci，另存父代理premerge核对；
verification为6a425f24717ccd96d9b0d5571e5f0c8fea3ff037ba8511d1c6aaa9239cbaeea3，
merge-verification为cd69483d30eac2f5766d5b529e2c876a359c39f63338250feaab9930b6a5f6ae，
原候选与main清单字节均复核。前置文档交付不补足v3/Vite运行时审阅，也不批准RC8。
