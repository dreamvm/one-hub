# 实际 RC5 镜像的本机升级与回退演练

2026-10-02，实际旧版 `v0.14.27-dreamvm.1-rc.5-gemini49-fix1` 到已合并
main `62486f76e7edd0558ee631d2b6142fc9f6f28e22` 的隔离演练完成：
**7 项通过，进程退出 0，清理结果另行核对通过。**
这是当前主分支的准备性验收，不是最终 RC8 验收、镜像发布或部署批准。

## 输入身份

候选由该提交的 `git archive` 副本和原 Dockerfile 构建；只在副本注入
`v0.0.0-smoke-62486f76e7edd0558ee631d2b6142fc9f6f28e22`。
使用 Node22.20 / Yarn1.22.22 frozen lockfile、Go1.25.14，未改写依赖文件。
从最终镜像提取 `/one-api`，通过仓库 `buildinfo` 检查：
`Go=go1.25.14 main=one-api GOOS=linux GOARCH=amd64 CGO_ENABLED=1`。

| 对象 | SHA256（完整值） |
| --- | --- |
| 实际旧镜像 config ID | `a50cc418252e0309e7b15b427279801eb1ef303e50e9cb26dc824469e148beac` |
| 实际旧镜像本机 OCI manifest ID | `60ed6a6c94ae117f270c10543b756ef3978aa9de1f1b279e412ced46fa6f9115` |
| 候选本机 build index ID | `6464fe04e3e65078191d75ad2d747d2f8c0771b3ac32e4f2e6a2cbe26d2b6b51` |
| 候选 amd64 manifest ID | `b865728d0a523a89350fd28d17894bae0818694e21d78479070df5715718a64c` |
| 最终候选 `/one-api` | `e04a2fdad46f897323ac54c651357a7ec8374623910ec01bc03bcb7e15d27e74` |

旧镜像从实际实例只读导出，未复制运行配置、挂载或数据库。原始归档保留；
为本机导入增加标签的派生归档仅改索引/标签元数据，config 和 layer blob 未改变。
旧版 `--version` 和演练中的运行状态版本均吻合。
以上 config、manifest、index 和程序哈希各有含义，不能互换，也不代表已发布 registry digest。

MySQL 固定为 CI 的 `mysql@sha256:212fe73edca5df6ff14826d5eb975c914bfb91f82a2e923f9050568f99525da1`；
Redis 固定为 `redis@sha256:2838d5524559494f6f1cd66e97e76b200d64a633a8614200620755ed395daf32`。
分别核对 amd64 manifest `5ba9d31938cfbfbcd6b29977181cfc246ce3f4b4923efc2af89c028d872fcc41`
和 `c904002d182255b6db3cbe3a1e8ce6c187d15390c39500b59fc07181aabff7bf`。
模拟上游由同一源码、Go1.25.14 和 CI 的静态编译参数构建。

## 隔离环境和执行方式

本机 Apple Silicon，Docker Desktop4.93.0 / Engine29.8.1，amd64 仿真。
Docker 上限4CPU、6GiB内存、24GiB磁盘；保留脚本原有逐容器CPU、内存、PID限制。
仅创建合成账号、内部 Docker 网络和临时数据库，不开放宿主端口，不连接真实供应商。
沿用源码中的 `.github/smoke/upgrade.py`，没有修改断言。
单命令默认90秒，全程另设20分钟超时，超时先发SIGTERM并给清理留出时间；本次未触发。

本机 containerd 镜像存储需区分索引与平台镜像：默认 `image inspect` 的架构字段为空，
`image inspect --platform linux/amd64` 可取得平台身份；子 manifest ID 在本机不能直接
`docker create`。实际使用已加载的不可变索引 ID，并为整次执行固定
`DOCKER_DEFAULT_PLATFORM=linux/amd64`。其他 Docker 存储实现可能直接使用 config ID，
不能复制本机 ID 就假定另一台机器已有相同镜像。

复用入口时先加载并检查全部镜像、从最终候选提取程序并执行 `buildinfo`，再运行：

```sh
# SOURCE_DIR 是已核对提交的源码副本；MOCK_BINARY 是同源码的 Linux/amd64 模拟上游。
# *_ID 均为该 Docker 引擎已加载并核对平台的完整 sha256 ID，不能使用浮动标签。
export DOCKER_DEFAULT_PLATFORM=linux/amd64
python3 "$SOURCE_DIR/.github/smoke/upgrade.py" \
  --old-image "$OLD_IMAGE_ID" \
  --old-version v0.14.27-dreamvm.1-rc.5-gemini49-fix1 \
  --candidate-image "$CANDIDATE_IMAGE_ID" \
  --candidate-version "$CANDIDATE_VERSION" \
  --mysql-image "$MYSQL_IMAGE_ID" --redis-image "$REDIS_IMAGE_ID" \
  --mock-binary "$MOCK_BINARY"
```

这段展示脚本入口；实际执行仍须外层超时与清理检查，不能据此省略上面的准备要求。
macOS 还须把 Docker 官方 CLI/凭据助手目录加入 PATH。
准备阶段的 Python3.9 解包参数、CLI PATH 和索引寻址失败均发生在最终完整演练前，
不计入通过结果；原始过程记录保留在私有证据中。

## 验收结果

| 阶段 | 实际断言 |
| --- | --- |
| 旧镜像 | 登录、启用/禁用令牌、中文JSON/SSE、HTTP/TLS、普通用户权限通过 |
| 备份 | 停止网关写入后导出合成数据库，校验SHA256 |
| 升级 | 原记录保留，旧凭据/权限可用，工具签名往返和新写入通过 |
| 仅回退镜像 | 旧版使用迁移后数据库和升级后新增行，登录/令牌/权限通过 |
| 恢复备份 | 完整schema/data dump与升级前相同；升级后写入按预期移除 |
| 恢复后旧版 | 密码、令牌、渠道、权限和Redis缓存检查通过 |
| 清理 | 合成备份及临时资源清理；另行核对本轮前缀下容器、数据卷和网络均为空 |

迁移新增31个列路径、删除0个；完整清单保存在日志中。
合成样本的旧版兼容不证明任意历史数据可降级；恢复备份会丢弃恢复点之后的写入。
尤其不改变[旧版OIDC回退约束](OIDC_IDENTITY_BOUNDARY.md#升级回退与保留事项)。
实际模型调用为0，生产数据复制为false。旧服务未升级、重启或改配置。
本机镜像和构建缓存保留，未执行全局Docker清理。

## 证据与未完成事项

私有补充证据 `artifacts/actual-rc5-local-upgrade-20261002.zip`，SHA256
`1fe04f3b8023975fd4035110a605073216efb4f579b9e15025dc222055b70634`，
包含构建日志、镜像身份、程序检查、7项验收日志及清理核对，不包含旧镜像归档或生产数据库。

本次仿真结果不能替代原生amd64性能或最终候选的双架构验收；最终源码/镜像变更后重验。
Realtime #78/#79的缺失审阅、依赖保留项、最终候选审计、新恢复点和专项发布批准继续开放。
本记录尚无独立事实审阅结论；自身文档候选CI及合并需另行记录，不能把本地演练写成文档已交付。
