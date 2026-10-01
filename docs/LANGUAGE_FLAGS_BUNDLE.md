# 语言图标入口体积

2026-10-01 本地候选，基于 PR #98 的6088330297714fef5f8c2013873bd2b8ee7292b1隔离副本；交付时仅提取i18nButton.jsx的导入与四个组件映射，保留最新main其他修改。本项叠加于标题PR #101的23292f41851765e29669f9f80ee759be5f73be35，只提取已经审阅的图标增量；先交付前置再改基main核对。本项尚无PR、准确候选CI或合并。

现有country-flag-icons默认对象被动态索引，构建保留所有国家图标，而i18nList只使用CN/HK/US/JP。改为这四个具名导入及同名映射，菜单、语言选择、持久化和未知语言回退逻辑不变，依赖与构建配置不变。新增语言国家码时需同步导入/映射。

相同Node22.20.0/Yarn1.22.22/锁文件及生产配置，使用只读generateBundle观察插件。实际入口文件从1,243,993字节降至1,019,702字节；同一Python gzip.compress(mtime=0)从380,977降至329,861字节，减少224,291原始/51,116gzip字节。country-flag-icons模块renderedLength从314,801降至6,106字节。最终文件和模块统计口径不同，不混用。JsonEditorRuntime约4.09MB为动态chunk，不计作本次入口；既有大chunk警告仍存在。

完整182项Vitest及依赖边界、格式、零警告lint、生产构建通过。一次复用上下文、非fresh的独立只读审阅，审阅者未参与实现，无具体发现；独立核对具名/默认组件引用相同、四语言覆盖和未知语言CN回退，复算文件/gzip、目标lint/格式及补丁hash。未独立重跑完整测试、构建或浏览器；源码审阅后不变。补丁SHA256 302f421381f93506af14f9cd06499766f9bd5e8b85e0c99190f08f364a65fec1。

实际Chromium154.0.8037.58 localhost生产构建、合成账号/API、外部请求拦截和Iconify替身。旧新各8组正常控制：1440浅色与390深色各四种语言，菜单4项、选中SVG、localStorage语言值、菜单关闭通过；旧新每种国旗SVG完全一致，页面/控制台错误警告均0。稳定后的移动深色菜单截图目视核对一致。测试初版误用语言代码和按钮序号，按实际i18nList/响应式DOM修正验收脚本后重跑，不是产品修复。

统计页请求的17个唯一脚本合计1,939,813→1,715,522字节（localhost未压缩content-length），差224,291字节。非生产压缩传输测量，不代表LCP、网络耗时或所有路由验收；图标服务使用替身。浏览器与临时服务已关闭。回滚导入/映射将恢复全部图标入口体积。


## 最终叠加候选复核

提取到标题PR #101之后，全部前端文件复制到新隔离副本，复跑完整187项Vitest/依赖边界、零警告lint及生产构建通过。源码增量与独立审阅补丁相同。最终入口1,019,702原始字节、同口径gzip329,853字节；前置修复引起chunk引用变化，不能将此gzip数与旧基线直接归因比较，上述同基线差值仍为主要体积证据。最终构建又复跑四语言×桌面浅色/移动深色8组正常控制，国旗SVG与原始基线完全一致，页面/控制台错误警告0；统计页请求17个唯一脚本合计1,715,655未压缩字节。浏览器和服务均已关闭。


## 2026-10-01 合并验收

PR #102 headf33f9706ef2aeb656b2c01bb0585f6ee75032f97的手动准确[Compatibility36872486412](https://github.com/dreamvm/one-hub/actions/runs/36872486412)/[Isolated36872494696](https://github.com/dreamvm/one-hub/actions/runs/36872494696)九项成功。初始叠加基线不在main事件范围，使用workflow_dispatch，不声称自动PR检查通过。smoke110405471212实际89PASS（12SQLite/35MySQL/35PostgreSQL/7升级回滚）、九波并发、四种Compose通过。

镜像sha256:612d372e8b62e1c914b0ea1c1160dfd94e9a07f3eed4d04e4bbbbb1b7adbe8d5；程序SHA256 a8d266eff4c77647beee2546c249e1080c01126e6a9086c99fe68287ca5d92df，Go1.25.14/one-api/linux amd64/CGO1。仅隔离本地镜像，不是发布。

最新mainc65c53f39e789447b1e7fce08f359a33875cda14计算合并树、候选均为8cab756c340e185ed8a4305abf105ebc023d5390。合成84fe2a5500c2a41529e6e27fa6c13ee45efb24c9仍保留旧23292f41/f33f9706父节点，树相同，不声称祖先刷新。锁定head合并7860d0d3db79f7bfdb8b04587df56efe50ff4847，API核实实际父节点最新main/候选，实际树一致。[main CI36875419258](https://github.com/dreamvm/one-hub/actions/runs/36875419258)已成功。
