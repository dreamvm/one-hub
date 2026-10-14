# 2026-10-14 新云端网络恢复回执

新one-hub实例独立恢复，未继承旧任务/workspace/work。正式命令网络授权、平台代理及TLS保留；没有直连、公开私有材料或分片规避。

平台源配置版本cecfgver_6acf472f842c8195986faccc68280df2，最新观测revision5、observed_revision5、observations_current=true、running、restricted/enforced。包管理器预设及八条额外规则均生效，含vuln.go.dev、*.oaiusercontent.com和Dropbox两条规则。启动早期revision3的unknown保留，不把有效列表当时误记为enforced；最新平台观测和实际下载分别记录。

- Go官方index/db.json完整35bytes，数据库更新时间2026-10-08T22:31:09Z；Git正式远端查询成功，main b783f95a、PR137分支d684d49c。
- GitHub连接器重新取得临时地址，centralus和westus两个*.oaiusercontent.com实际完整下载成功。
- PR136 amd64 ZIP74163453bytes，SHA256 e1f5615ff654459a543db6106a52eb2aafc022460b2e8f27a70c39bde626e708；arm64 ZIP70599773bytes，SHA2567728b0be5680dd977febc715d10ebae5f7b1938ad068e0e1d51957a2a96430b2。
- 两ZIP CRC、固定程序哈希、manifest和实际buildinfo全部吻合：源码4bf131868ae8d4431ddff2f508a453e7e615fa31、Go1.25.14、one-api、Linux/对应架构、CGO1。不当作当前main重建产物或registry镜像。
- 获授权Dropbox四文件完整恢复，三归档大小/SHA256与用户交接一致；ZIP CRC通过。RC5镜像全部7blob逐字节哈希、5层身份、config a50cc418、manifest60ed6a6c及linux/amd64核对通过，Docker正常加载为config ID。不含生产数据库、配置或挂载。

私有日志、材料、审计JSON及校验回执保存于本任务/workspace/work；未假称原任务559266bytes私有快照已迁移。当前工具目录没有read_thread能力，两条旧任务未发消息，使用用户交接、授权Dropbox与已推送Git/GitHub恢复并独立重验；历史原始证据仍以原durable任务为准。

实际RC5到PR136程序的本轮本地基线演练7项通过、真实模型调用0、production_data_copied=false。独立本地运行镜像含同一已验证程序，但不是CI原整镜像或最终RC；新工具链/依赖候选必须另行演练。合并PR137及新PR、标签、发布、部署和真实付费均保持待专项批准。
