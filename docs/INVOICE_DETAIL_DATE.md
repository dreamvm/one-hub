# 账单详情返回日期字段

2026-10-01 独立候选，叠加返回文案 PR #105 的 `004de21839769039301df836915b3cf87dcc65b1`；先交付前置，再改 main 核对合并树。本项准确候选CI、PR和合并尚未完成。

真实临时SQLite月账单的列表返回日期正常，详情两行date却为空。当前前端从已校验路由显示月份，因此模型、金额和用户可见月份仍正常；接口字段缺失不能作为全部响应正确。原因是GetUserInvoiceDetail把方言日期表达式作为Select的问号参数传入，实际选择了字符串常量，未返回名为date的列。

仅将已有固定日期表达式拼接为Select列：SQLite使用strftime，PostgreSQL使用TO_CHAR，MySQL保留原date列。输入用户ID和日期继续使用参数绑定；不改变过滤、分组、求和、排序、schema、余额或日期合法性规则。不是动态用户SQL拼接，不扩展输入范围。

新增回归自动进入既有 `^TestQuotaTransaction` 三数据库CI入口：合成两模型月账单、另一用户和另一月份记录，核对金额/token/请求/时长及范围；日期字段保留所选日期；空月和非法输入保持原语义。旧SQLite两个控制通过、日期子项失败；修复后SQLite三个子项全部通过。固定Go1.25.14的规定离线model/types/Gemini/Claude/requester race与model vet通过。

一次复用上下文、非fresh独立只读审阅，审阅者未参与实现。应用查询改动未发现问题，独立SQLite race三子项通过；发现P2测试夹具直接AutoMigrate旧模型datetime标签，不兼容标准PostgreSQL。父任务核对后改为三种测试引擎支持的SQL DATE建表，拒绝替换既有同名表，仍只使用既有显式选择的临时SQLite或固定localhost一次性数据库，清理自身表后关闭连接。最终SQLite专项重跑通过，格式通过；该测试夹具修正在审阅之后，无第二轮复审。

MySQL/PostgreSQL实际专项仍待本候选CI；本地无相应服务，不能将方言源码审阅写成运行通过。夹具不验证生产schema迁移或月账单生成，不改变生产模型类型。MySQL原datetime字符串后缀保持，不额外规范所有数据库响应表示。回滚恢复日期字段缺失，已存数据不受影响。无标签、发布、生产写入或付费调用。
