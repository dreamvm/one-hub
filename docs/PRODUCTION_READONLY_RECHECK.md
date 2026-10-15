# 升级前的生产只读复核与恢复点准备

2026-10-15范围更新。本文件是公开的复核模板，不是生产验收结果；具体站点核验摘要
只保存在私有验收记录。本云端不执行生产操作，不请求生产原件或恢复副本。
生产备份、数据及隔离恢复副本只允许保留原服务器，由本地在既有授权范围内处理。
本文件不撤销或重复申请已经明确的本地备份/隔离恢复授权。新PR合并批准不等于
标签、镜像发布、部署、停写、凭据轮换、重启或升级批准；新增生产变更另核专项范围。

## 已有具体材料

以下路径供本地定位既有私有记录，保留原有权限，不请求复制到云端；正文、地址、
账号、秘密值及私有生产核验摘要均不放入公共PR：

- `artifacts/site-readonly-inventory-20261002.json`；历史SHA256
  `799dbf314e27994a2fc489665727c986567705ca991eede164e9c3d1da463d14`。
- `artifacts/site-accounting-readonly-20261002.json`；该数据库的历史只读快照。
- 当时引用的2026-09-22备份目录、七项SHA256清单、两份network=none恢复记录，
  以及后来备份/轮换/维护窗口记录的真实路径。
- 原 durable 任务保留的 `/workspace/work/cloud-handoff-20261014/HANDOFF.md`、manifest/
  FILES/SHA256SUMS。此路径属于旧任务，不是本云端已迁移目录。

此前RC5镜像/两证据zip已恢复不等于上述站点记录及559266 bytes私有快照已迁移。
本轮可用工具未提供 read_thread；不向原任务或过渡任务发消息，也不声称已读未取得的材料。

## 运行版本与镜像：只输出选定字段

由本地按既有已认证连接填写真实容器名及状态URL，不猜主机、不改变SSH身份/TLS/代理。
以下是可审阅命令模板，不是在云端已执行的生产命令；变量须来自本地已有记录。

```sh
docker inspect --format 'container={{.Id}} image={{.Image}} started={{.State.StartedAt}} restarts={{.RestartCount}}' "$ONEHUB_CURRENT_CONTAINER"
onehub_current_image_id="$(docker inspect --format '{{.Image}}' "$ONEHUB_CURRENT_CONTAINER")"
docker image inspect --format 'image={{.Id}} digests={{json .RepoDigests}} revision={{index .Config.Labels "org.opencontainers.image.revision"}}' "$onehub_current_image_id"
curl --fail --silent --show-error --max-time 10 "$ONEHUB_STATUS_URL" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("data",{}).get("version","version_absent"))'
```

记录核对时间及检查前后identity/started/restarts；不输出整个inspect、Env、Compose、
连接串或私有状态响应。版本接口、image ID、registry RepoDigest和revision label分别记录。
旧快照的`a50cc418...`是本地image ID，无RepoDigest；不能改称registry manifest digest。
如果真实版本/digest不同，先界定差异，不能套用旧RC5隔离演练。

## 备份、恢复与凭据：只读现有记录

先确认真实最新恢复点时间、文件权限、七项/后续清单身份及恢复日志对应同一备份。
本节命令只校验已有文件；新备份或恢复由本地按既有专项授权单独执行，原件和副本
均留原服务器。已有备份目录内可用
`sha256sum --check --status --strict SHA256SUMS.txt`；先人工核对清单只指向该目录的
已知文件，不跟随陌生绝对路径或外链。只回报数量/一致性；路径和内容保留私有。

旧七项哈希匹配只证明和清单一致；历史两次18表恢复日志不证明本轮恢复已重跑。
2026-09-22恢复点不能保证保留之后写入，必须另外确认最近一次成功备份与恢复验证。

凭据清单仅回报类别、轮换时间、旧值撤销证据、实例切换证明、回退兼容和缺口：
管理员/会话、数据库/Redis、模型渠道、支付商户/回调、OIDC client、曾暴露的Git/registry
或备份配置凭据分别核对。不开启轮换、不主动尝试旧密钥、不输出值或秘密值哈希。
运行实例健康和初始化ENV不证明轮换完整；已有1045历史说明须使用当前实际可用连接。

## 账务与OIDC：一致性只读汇总

继续使用服务器内既有认证连接，不在命令行/日志传连接串。每条SELECT最多2秒，
总进程最多25秒；只输出表/列元数据及数量/状态，结束ROLLBACK，不读账号、令牌、
请求正文或逐条记录。先确认受检表存在且为InnoDB；缺表记absent，不建表或回填。

可审阅的MySQL会话设置和schema查询：

```sql
SET SESSION MAX_EXECUTION_TIME = 2000;
SET SESSION TRANSACTION ISOLATION LEVEL REPEATABLE READ;
START TRANSACTION WITH CONSISTENT SNAPSHOT, READ ONLY;
SELECT @@version AS version, @@transaction_isolation AS isolation;
SELECT table_name, engine FROM information_schema.tables
 WHERE table_schema = DATABASE()
 AND table_name IN ('users','orders','tasks','midjourneys','quota_reservations','order_payment_claims');
SELECT table_name, column_name FROM information_schema.columns
 WHERE table_schema = DATABASE()
 AND table_name IN ('users','orders','tasks','midjourneys')
 AND column_name IN ('oidc_id','oidc_issuer','oidc_identity_key','settled_at','reservation_id','status','gateway_no');
ROLLBACK;
```

元数据确认后在同一设置的新一致快照汇总存在的表：orders按status及是否settled
计数、非空gateway_no重复组；tasks/midjourneys按status及reservation字段是否存在；
users非空旧oidc_id与重复组、已知/空issuer数量；新账本按终局/未结算状态计数。
旧版没有新字段时只查旧字段，不把缺失账本当零行或全部旧请求已结算。
若出现历史支付/任务/未知OIDC绑定，先输出数量和需要核实的归属类型，不猜费用、
重放、退款、补余额或subject-only自动关联。历史零值只适用于2026-10-02快照。

## 恢复点与维护方案的职责和门槛

以下是本地执行方案的检查顺序，不是云端已执行记录。已批准的本地备份与隔离恢复
继续按原范围办理；升级、停写、重启、轮换及新增生产写入须先给出真实旧/新image
identity、配置差异、维护窗口起止、恢复点、回退责任人和具体动作，再核对对应专项批准。

1. 固定最终已审阅源码、实际新镜像版本/digest及架构；核对旧镜像和不兼容字段。
2. 明确如何停写、通知范围、超时与取消条件；支付回调和后台任务不能被普通页面维护替代。
3. 本地按已有或对应新范围的批准，在原服务器私有0700目录生成一致性数据库/配置/挂载
   备份，文件0600；核对清单、时间、数据库状态和旧镜像identity，不公开或上传生产内容。
4. 由本地在原服务器的隔离环境恢复对应恢复点并核对schema/data/readback；恢复副本
   不离开原服务器，生产写入保持由已批准窗口计划控制。
5. 升级后只在固定检查和回退触发条件下继续；失败时区分仅镜像回退与完整恢复。
   完整恢复会丢弃恢复点后写入；旧RC5会重新开放Realtime，不能自动降级。

本云端没有执行上述生产步骤；本地进度及具体证据仅按获准范围在私有记录维护，
不能从本模板推断本地未执行或已经通过。发布前仍须由对应责任人确认维护窗口、
最新恢复点及对应恢复证明、凭据轮换与旧值撤销完整性、历史账务/OIDC边界和回退约束。
CI及合成演练不能代替这些事实；不通过公开PR索取原件、逐条业务数据或秘密值。
