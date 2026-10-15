# PR137 合并后的发布前整改

2026-10-14。PR137 获得准确候选的专项合并批准后，实际合并提交为
`4c2f31844c5a992879b9ffd83d21cb3cd0d04907`。
父提交是 `4cecf90c86ca0ab5ce0a5d43e57f79c04153a5e7` 和
`a4d9aeba90015b642b5eb4e3d03a130d94268728`；树为
`7faac64fba5d95b0dc54ca6bff80c3d15d7f5784`，与批准候选及受测合成
`d0659bbfd109c7f1d50e5d2b62388d53b67adf21` 相同。
合并前重核三文档范围、独立只读审阅、两个候选运行十项有效 CI；合并后另核
[main CI38598613933](https://github.com/dreamvm/one-hub/actions/runs/38598613933)
四项成功，逐任务实际 checkout 均为真实合并提交。没有用旧 CI 替代合并后 CI。

## 三个独立维护项

| PR / 准确提交 | 最小范围 | 旧失败和正常对照 | 独立审阅 |
| --- | --- | --- | --- |
| [142](https://github.com/dreamvm/one-hub/pull/142) / `3aebe7d62fd347c4943f1f6677c6bd27dda19956` | fast-uri3.0.6→3.1.8 | 24边界失败、3正常通过；新27叶子通过 | fresh只读绕过/回归审阅，无范围内阻断 |
| [143](https://github.com/dreamvm/one-hub/pull/143) / `523c36c5d9cf636238f0bcfa314ad0d2ab159b6e` | 旧form-data类型copy合并到已锁4.0.6 | 真实嵌套旧copy10边界失败、2正常通过；新12叶子通过 | fresh只读审阅，无范围内阻断；额外HTTP filename/本地multipart解析 |
| [144](https://github.com/dreamvm/one-hub/pull/144) / `a9df0d5b14b639d901f2b3af36559402749aee87` | brace-expansion1.1.11/2.0.1→1.1.21/2.1.7 | 8小预算失败、4正常通过；新12叶子通过 | fresh只读审阅，无固定入口阻断；601短语法及默认/可选预算 |

每项有独立提交和叠加 PR，依次依赖前项，四个候选文件的审阅指纹与提交一致。
官方 tarball 完整下载，SRI/SHA1/SHA256 及安装内容均核对；没有全局 resolutions。
各项 frozen install、全部 dev/dependency 边界、26 UI 文件/234 测试、lint、build 和
暂存隐私检查通过；build 保留既有 chunk 警告。PR142 自身两个运行十项 CI 成功。
工作流仅自动覆盖 base=main，PR143/144 的精确组合检查在集成 PR 另核，不能借用
PR142 或历史 main 的绿灯。集成候选的自身 head/tree/CI 在该 PR 中记录，未完成不预记通过。

一次 brace 独立审阅误纳入旧零步长输入，受限子进程退出134，已结束且无残留；
不计通过或验收证据，永久回归没有该旧库压力测试。限定维护入口与 API 兼容限制见
[fast-uri](FAST_URI_MAINTENANCE.md)、[form-data](FORM_DATA_TYPE_COPY_MAINTENANCE.md)、
[brace-expansion](BRACE_EXPANSION_MAINTENANCE.md)。本轮是手工静态分流与项目要求的
独立审阅，未取得完整 Codex Security 后台扫描；不把它称为完整扫描或 GitHub 正式 Review。

## 175 路径保留与去重

原始审计保留在私有证据中：175路径、80 Yarn ID、60个去重 GHSA、26模块。
[逐路径台账](FRONTEND_175_TRIAGE_20261014.csv)逐一保留全部175输入、声明根依赖角色、
原 dev 标志、产品可达性判断与候选版本匹配，未仅保留方便修复的条目。
根清单划分73开发路径、102声明 production 路径；后者大量是 CLI/构建工具，
不等于 Go 生产服务器可达。原 Yarn 的 dev=false 不作为生产可达性证明。
原175输入按当前证据有41条限定入口排除、134条产品可达性待核；其中包含版本维护
后移除的匹配。候选仍报告的113条有19限定排除、94待核。待核队列不是确认漏洞数。

三个维护项后的实际官方审计为113路径、53 Yarn ID、41去重 GHSA、23模块。
原始路径 key 零新增、移除62：fast-uri18、form-data4、brace-expansion40。
审计 JSON 为500830 bytes，SHA256
`502f9c31c0eb8072d7c0d3a1cb2741c5245e28c78aa0fd0cd99606402d3a60dc`。
命令退出14表示仍有匹配，不是审计通过。此数字只绑定本候选锁树，main 尚未合并这些维护项。

下表以模块汇总实际入口/静态证据和下一步；GHSA 跨模块共享，不能把各行公告数相加。
未建立不可信输入到受影响 API 的完整链时保持 needs_review；没有将库版本匹配写为
已证实产品可利用，也没有将限定入口排除写成库已修补。

| 模块 | 原路径 / GHSA | 实际入口或必要前提 | 当前处理 |
| --- | ---: | --- | --- |
| fast-uri | 18 / 9 | Conf→Ajv8固定schema，parse/serialize/resolve；不是网络授权器 | PR142版本维护；移名/suffix及其他授权边界保留 |
| form-data | 4 / 2 | 旧copy仅@types/node-fetch.d.ts；Axios已4.0.6、OpenAI用formdata-node、浏览器用原生API | PR143移除旧copy；原始header/contentType不通用净化 |
| brace-expansion | 40 / 8 | minimatch3、glob10→minimatch9的固定小型文件模式 | PR144两分支维护；截断/字面余项及rewrite差异保留 |
| minimatch | 15 / 3 | CLI/文件匹配；受影响模式与来源须区分 | 3.1.5仍生成旧嵌套extglob正则；不应用，9.x须另验 |
| picomatch | 12 / 2 | Sass/readdirp、ESLint、Vitest等不同模式/选项 | 既有4.x重复表示和分支/literal-space差异继续保留 |
| braces | 4 / 1 | 另一库的嵌套模式递归；非brace-expansion | 无已发布修复版本；实际输入边界待核 |
| flatted | 2 / 2 | ESLint→flat-cache→Keyv缓存加载 | 3.4.4实际缓存回归阻断，未提交/推送 |
| lodash / lodash-es | 9+3 / 3共享 | CLI差异删除/模板；路径表示和原型归属影响结果 | 4.18.1仍有原型路径绕过；未应用，当前HTTP链未证实 |
| immutable | 6 / 3 | Sass实际OrderedMap/List，键为Sass Value；pretty-format只遍历 | 4.3.9等价键新回归与残留已证实，未应用 |
| js-yaml | 5 / 5 | Lobe可选Markdown/gray-matter的v3；维护入口固定JSON | 3.15.2既有自有键/异常契约回归保留，不改为v4 |
| yaml | 2 / 1 | CLI配置/ESLint工具树；各自v1/v2别名与解析选项 | 输入、受影响API和调用方覆盖仍待核 |
| ajv | 4 / 1 | ESLint6/Conf8；公告$data为可选模式，固定schema不等于全部API | 尚缺不可信pattern进入$data调用证据，保留待核 |
| browserslist | 6 / 2 | Babel目标解析；项目查询固定，stats/动态查询为另一路径 | 具体可选模式和全部消费方覆盖待核 |
| glob | 2 / 1 | Vite paths与Lobe工具树；公告针对CLI --cmd | 库调用不能直接当CLI利用链；完整选项/入口待核 |
| langchain / @langchain/core | 1+3 / 1共享 | Lobe模型工具树；公告序列化/反序列化元数据 | 不能将模型字符串等同load入口，不做真实付费；保留待核 |
| langsmith | 9 / 3 | 跟踪、远端prompt/反序列化等分别有前提 | 配置与受影响调用链未全面证实，保留待核 |
| uuid | 4 / 1 | 已查v4/validate，v5仅name+namespace，无supplied buffer | 限当前调用排除，不是库修补 |
| diff | 6 / 1 | uvu只用diffArrays/Lines/Chars，不用parsePatch/applyPatch | 限这些路径排除，不是库修补 |
| ws | 4 / 2 | Ink调试路径；维护CLI入口DEV=false | 限固定入口排除，不是整个ws安全认证 |
| sprintf-js | 1 / 1 | Lobe传递工具，精度DoS需格式串/精度可控 | 无已发布修复；具体调用和输入信任仍待核 |
| Babel SystemJS plugin | 1 / 1 | 维护JSX/build/parser未选择SystemJS输出 | 限固定配置排除；改变输出模式重审 |
| react-router / @remix-run/router | 8+4 / 4及2 | 客户端BrowserRouter；SSR/Data与Link/navigation公告不同 | SSR入口排除；其他路径/来源未以库版本假定关闭 |
| dompurify | 2 / 2 | Monaco内嵌3.4.15，derived config不启用IN_PLACE | 限当前JSON editor前提排除；单改传递锁不能替换内嵌 |

docs 有独立锁树；Prettier3.2.5内嵌旧flatted/brace/minimatch；Monaco内嵌DOMPurify。
这些副本不能被 web 的 hoisted lock 升级冒充修补。本表未宣称整个依赖族清零。

## 不安全候选的实际反证

- flatted3.4.4裸库7叶子通过，但深扁平链在成功解析后进入 flat-cache 未等待的
  Keyv Promise，json-buffer再次递归而退出；空引用转0形成循环也退出。旧版在
  parse阶段抛错，由tryParse回退空缓存，正常退出。直接嵌套JSON/深reviver也未全面兼容。
- Lodash4.18.1对深路径使用根对象自有键guard；CJS monolith、单方法和ESM的隔离
  合成标记均被删除。不是生产标记，也未证明当前项目远程可利用。
- minimatch3.1.5和旧3.1.2在短嵌套extglob生成相同回溯正则；24字符正常/失败控制
  保留，不能根据patched版本号推断该类已消除。
- Immutable4.3.9在32个合成碰撞字符串键上，等价boxed key查不到、set使size32→33；
  旧4.3.5等价查找/size正常。数组型键仍改变返回对象原型、数值字符串setSize仍截断，
  普通constructor键会丢弃；不声称污染全局原型或旧无限循环已被新字符串参数证实。

这些候选仅用于隔离调查、反证和私有记录，没有推送成已通过的修复。

## 四项历史受限审阅的准确范围

| 原PR及head | base / 实际合并 | 本轮补验 |
| --- | --- | --- |
| #68 `4ddc5b3db94e58e996a09f07da81e5fa81f62f25`，6文件+366/-135 | base `1a89e9e28e884ebee417d54e647e012718b4c4e2`；merge `b53412b493b878532b7b5d377af6792b4256ccb0` | 独立读取准确范围、Rollup4.59官方源码和已有12拒绝/8合法generate/write回归；本轮未重跑这些历史测试，无新源码阻断 |
| #71 `b40c25510a6727ad70150423facea7dce93cddc2`，9文件+548/-886 | base为#68 merge；merge `1f6d08bf5ba4e77a566070ecf5f21c411bfc205e` | 独立读取实际Vite8.3.1 map边界及已有9拒绝/6正常回归；本轮未重跑这些历史测试，无新源码阻断 |
| #78 `f63d6e96b6800605f1a6dea7086694e907b799c4`，6文件+363/-0 | base为#71 merge；open/draft，未合并 | 独立读范围；同一race harness的144异常叶子失败、102正常通过，已知early-envelope缺口保留 |
| #79 `ad68cf003bb5de5f4dda4c1726dd95c868bccccc`，11文件+435/-5 | base为#78 head；open/draft，未合并 | 同一harness246叶子通过，无race warning；不开放功能或合并草稿 |

Realtime harness SHA256 `01e4b311b1eb8a12289bc3ec631e2252b3b8aa2158cfd75d399a406c24b929c0`；
两个历史源码都用Go1.25.14，临时SQLite/localhost、合成账号，无真实模型或生产数据。
覆盖created/done/progress、重复/大小写type、字段顺序、首/后帧、匿名ID、client text和
provider binary来源隔离。截断/不可识别帧、item-only、崩溃持久化及所有供应商未覆盖。
协议错误会关闭实际连接，不把quota层matching correction写成在线连接继续修复。

#79的`ba8656b1328c56f0f4894081e7c9768873e34b11`是合成合并提交，不是实际合并。
#68/#71原先中断的专项审阅及各自单项例外仍是历史事实，例外仅限各自准确候选；
本轮新手工补验不把旧中断改写为通过，也不替代完整专项扫描。
main仍保留#127在认证/连接/预留前返回501；补验不是开放Realtime的批准。

## 生产与最终发布边界

当前能够恢复的是2026-10-02只读快照和2026-09-22旧恢复点记录，非本轮生产复核。
真实站点入口和最新私有材料未在云端提供；网络策略enforced但无配置的生产VPN/
TCP目标或凭据。没有猜地址、绕过代理、读取生产密钥、重启、升级或导出数据。
具体已有路径、只读核对和未执行的新恢复点方案见
[生产只读复核](PRODUCTION_READONLY_RECHECK.md)。

现有Go源码/双架构程序、实际RC5隔离演练和浏览器证据仍绑定原76c5657a；本候选
依赖和生成资产变化后不能把旧程序SHA/镜像ID、旧升级或旧浏览器结果冒充新最终产物。
准确组合候选的CI和补验另记在PR；默认180限流下多次无缓存重载429及其他浏览器/
暗色/全部页面、真实客户端和付费验收仍未以234单元测试关闭。
新PR保持待合并。没有标签、镜像发布、部署、凭据轮换或真实付费授权。
