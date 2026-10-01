# 模型信息选项回调的 props 警告

2026-10-01 独立候选，基于 PR #95 合并 ca1b429c64c8fff6a80dc1f9463b434bcc85d143；尚无本项准确候选CI、PR和合并。

输入/输出模态的两个MUI renderOption局部参数名props，被当前eslint-plugin-react的通用props名称推断标记，进而将key归到父组件验证。源码核对MUI Autocomplete由getOptionProps构造选项回调参数；它不是EditModal组件的入参。仅将两个回调形参及对应解构右值明确命名为optionProps，key继续显式传给li，其余事件、id、aria属性原样展开。没有新增propTypes、禁用规则或改动业务行为。

旧目标ESLint --max-warnings0因两条key警告失败，新目标零警告且Prettier通过。一次独立只读审阅复用上下文、非fresh，审阅者未参与实现，无具体发现；审阅后源码未改变。完整16文件162项Vitest及已有依赖边界通过，lint零错误、剩运营设置getOptions一条警告，生产构建成功且保留既有大包提示。初次沙箱不允许绑定localhost而中止，工具权限批准后重跑通过，未视为产品失败。低影响局部改名不另增重复实现的测试。

Browser plugin不可用，缓存PlaywrightCLI0.1.22/Chromium154.0.8037.58；随机localhost生产构建、全部API为合成且只允许GET、外部请求拦截并对装饰图标用本地替身。1440×1000浅色与390×844深色，分别检查输入/输出模态鼠标选择、键盘高亮及Enter选择、唯一选项ID和aria-selected、自定义值输入、取消且无写入；零未处理页面异常及控制台错误/警告。初版脚本按钮名不完整、又多按Escape关闭了对话框，按真实DOM及菜单开闭状态修正后最终全部通过，不将脚本错误计入产品结果。

截图已目视核对：桌面完整显示，移动多行标签正常、较长对话框需纵向滚动访问底部操作，未声称所有内容同屏。浏览器与临时服务已关闭。未覆盖真实后端创建/更新、旧记录编辑或生产权限；没有提交、删除或真实付费调用。回滚局部改名仅恢复警告；其他页面、初始读取生命周期、RC8及发布门槛仍开放。

继承 PR #94 最终测试修正及已合并 PR #95 后，完整162项Vitest、依赖边界与lint再次通过，仍一条既存警告。整个前端与上述已构建/浏览器验收的副本逐字一致；本项应用源码在独立审阅后未改变。

## 2026-10-01 合并验收

[PR #96](https://github.com/dreamvm/one-hub/pull/96) head `e7f306279cb60d291b3467a1f2f4d711de515cf6` 的九项准确候选检查成功：[36864757032](https://github.com/dreamvm/one-hub/actions/runs/36864757032) / [36864757317](https://github.com/dreamvm/one-hub/actions/runs/36864757317)。实际smoke为SQLite12、MySQL35、PostgreSQL35、升级/两条回滚7项共89个PASS，含九轮并发，四种Compose启动通过。

最终runner镜像 `sha256:b150465e989f718a695798db32f46b19b319f9dee6275da57015dd51acdb29e2` 内的实际程序为Go1.25.14 / one-api / linux/amd64 / CGO1，binary SHA256 `1e81fe0b7d3825595f492e6362a00ebb1a8d7659d7bcafe0e62cc66613ef1078`。镜像未发布。

GitHub合成 `ca4b51f3d8dbd22674628eeddd5e10abf343aa6e`，实际合并 `e391ba683928f3958ba666477e39408884996188`；候选、合成、计算及实际树均为 `01fffa4993b1aa9c8746c4d6230b27ea6a06268d`。合并后main [36867428349](https://github.com/dreamvm/one-hub/actions/runs/36867428349) 待核对。

后续核实：main CI36867428349已成功，PR #96交付验收完成。
