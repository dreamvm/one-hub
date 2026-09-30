# Monaco 实际运行版本边界

## 旧版与修复范围

基线 `3a74e083f806c5440afe1cb6e1663496862c80a3`。三个入口为渠道编辑的
`custom_parameter`、`MapInput` 和 `ListInput`，均使用 JSON 模式。
已发布的 `@monaco-editor/loader` 1.7.0 默认从 jsDelivr 加载 Monaco 0.55.1；
浏览器旧版对照也实际发出了这些请求。即使升级锁文件，未配置的 loader 仍独立选择 CDN
版本。因此验收不变量是三个入口的编辑器、JSON worker 和核心 worker 都来自锁定的本地构建。

候选精确锁定 Monaco 0.57.0，其官方包内嵌及依赖声明的 DOMPurify 均为 3.4.15。
共享 `ui-component/JsonEditor.jsx` 沿用项目 Loadable/lazy 模式，只在编辑器实际挂载时
加载 `JsonEditorRuntime.jsx`。运行模块在 Editor mount 前调用 `loader.config({ monaco })`，
使用 Vite 的本地 worker 入口；三个调用方只改变 import，表单逻辑、配置、主题和回调保持。
没有添加远程 schema、自定义 Markdown provider 或新的 sanitizer 参数。
官方 ESM export map 已改变，worker 使用该版本支持的子路径。AMD 在上游已弃用支持。

这次关闭的是锁文件与实际运行版本脱节；**没有证明当前应用可利用的 XSS**。
普通 JSON 进入文本模型。库中存在 hover → Markdown → sanitizer 路径，但现有 JSON
配置是空 schema 列表、禁止 schema 请求；未发现应用使用 `IN_PLACE` 或相关 hook。
不为复现公告而引入原应用没有的危险选项。

参考：[官方发布](https://github.com/microsoft/monaco-editor/releases/tag/v0.57.0)、
[包定义](https://github.com/microsoft/monaco-editor/blob/v0.57.0/package.json)、
[变更记录](https://github.com/microsoft/monaco-editor/blob/v0.57.0/CHANGELOG.md)。
loader 的 Git tag 默认版本与已发布包不同，运行证据以安装包和实际请求为准。

## 本地验证

- Node 22.20.0 / Yarn 1.22.22，隔离副本 frozen install 成功；锁文件仅更换 Monaco
  与 DOMPurify 两个条目，保留其他依赖和既有 Rollup 4.53.3，未依赖草稿 PR #68。
- `yarn test`：30 Vite、20 Axios、16 PostCSS、28 Babel 叶子检查，Node 汇总 31/67
  含父节点；62 项 Vitest 通过。新增两个检查使用真实 loader，验证本地实例初始化没有
  注入远程脚本，以及 JSON/core worker 的选择；它们不替代实际浏览器验收。
- `yarn lint` 零错误、9 条既存警告；`yarn build` 通过。懒加载 Monaco chunk 约
  4.09 MB（gzip 1.06 MB），JSON/core worker 约 407/276 kB；大 chunk 警告仍开放。
  完整 Monaco 包还生成其他语言的资源，本应用受检 JSON 路径不请求那些 worker。
- Chrome 154.0.8037.58，1440×1000，localhost 随机端口；全部 API 为合成夹具，
  外部资源除模拟图标外拒绝。真实三个编辑器正常加载；worker 为同源构建文件，旧 CDN
  请求为零，未观察到 worker fallback 或 pageerror。
- 先清空选区、再以 70 ms 间隔键入：额外参数的输入、撤销、格式化通过；映射和列表
  正常输入、格式化、提交通过。畸形 JSON 与列表的对象输入被拒绝，修正后可继续。
  最后模拟渠道 POST 恰好一次，保存的额外参数、模型映射、禁用流式列表与输入一致。
- 通过实际主题按钮切到深色，再重新打开空表单；额外参数的键入、撤销和格式化再次
  通过，截图确认深色编辑器和文本可见。此为桌面有限流程，不替代全键盘和移动验收。
- 初次测试使用 Escape 关闭了 MUI 对话框，导致测试定位超时；更正操作后复跑。
  控制台记录了 MUI 对话框焦点/aria-hidden 警告，不宣称零警告。
- 最终测试首次因沙箱禁止监听 localhost 返回 EPERM；保留失败记录，以允许本地监听的
  权限重跑，不将环境拒绝写成测试通过。

## 独立审阅与修正

全新只读审阅核对直接加载候选，未找到其他源码可证实的运行版本绕过或编辑阻断。
确认一项资源回归：渠道列表静态依赖链会在编辑窗口打开前加载完整 Monaco。
父任务采用既有 Loadable/lazy 修正；浏览器重新验证列表阶段 Monaco runtime/worker
请求为零，首次打开才加载本地 runtime 和 JSON/core worker，随后重跑三个编辑器控制。

本次遵守单次审阅周期。审阅者结束时没有覆盖随后加入的懒加载包装及运行模块移位，
这些修改由父任务验证；不宣称最终调整已独立复审。审阅者未执行浏览器或独立复核测试数字。

旧版正常间隔键入同样通过；直接覆盖选区会受自动包围选区行为影响，零间隔合成输入
仍出现异常。这些现象没有被当成 React 缺陷，也没有靠修改编辑器输入选项隐藏。
`keyboard.insertText` 不是操作系统剪贴板粘贴；不据此宣称真实粘贴或全键盘验收通过。

## 审计与开放项

按用户明确授权复查 Yarn 官方审计端点，仅发送公开依赖元数据。候选结果为 205 条
匹配路径、72 个唯一 GHSA，DOMPurify 剩 `GHSA-p98j-92pf-mc4p`（修复下限 3.4.16）。
其前提是 `IN_PLACE` 与移除节点的 afterSanitize hook；受检调用没有这些配置。
Monaco 内嵌 3.4.15 不能由单独覆盖传递依赖替换，因此保留公告与上游版本跟踪。
其他依赖公告、Rollup 独立审阅门槛和 Axios 新公告继续开放。

诊断 hover 被现有 overflow 容器裁剪的问题留在页面行为专项；此处未修改容器或悬浮层。
移动端、Safari/Firefox、原生 Windows、真实触屏/剪贴板、生产保存与性能验收未完成。
准确候选 CI 与合并尚待完成；不把本地通过写成交付或生产验收。

回滚应同时回退集成模块、三个 import 和两个依赖条目。回退会恢复旧 CDN 版本脱节，
不能当作安全边界继续有效。没有创建标签、发布镜像、部署或真实付费调用。
