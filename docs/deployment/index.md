---
title: "部署说明"
layout: doc
outline: deep
lastUpdated: true
---

# 部署说明

## 配置说明

系统支持两种配置方式：

1. 环境变量
2. 配置文件 (config.yaml)

::: tip 配置优先级
环境变量 > 配置文件
:::

### 必要配置

- `USER_TOKEN_SECRET`: 必填，用于生成用户令牌的密钥
- `SESSION_SECRET`: 推荐填写，用于保持用户登录状态，如果不设置，每次重启后已登录用户需要重新登录

## 本 Fork 的 Docker Compose 部署

维护入口是本仓库的四个 `docker-compose*.yml` 文件。请从**同一个已审阅提交**复制四份文件到部署目录，不能只替换主文件。
使用 Docker Compose 插件（`docker compose`）；隔离验收固定使用 Compose 5.5.1。
模板使用 [extends](https://docs.docker.com/compose/how-tos/multiple-compose-files/extends/) 共享配置，仍允许普通 Compose override。

默认 `docker-compose.yml` 保持 MySQL + Redis，以及已有 `./data:/data`、`./data/mysql:/var/lib/mysql` 路径。
应用只接受 `ghcr.io/dreamvm/one-hub` 的明确 digest；没有 `latest` 或尚未发布的 RC 默认值。
MySQL/Redis 使用与隔离 smoke 相同的固定镜像；当前运行验收为 `linux/amd64`，不据此承诺 ARM 兼容。
这不是部署指令的自动授权；发布、现有数据库升级与生产切换遵循 [发布流程](../RELEASE_PROCESS.md)。

### 先准备配置

**已有部署**：备份 Compose、`.env`、`data/config.yaml` 和数据库，保留原数据路径、项目名称、会话密钥及令牌密钥。
填入现有 MySQL 应用和 root 密码；修改容器环境变量不会轮换已有数据库账号密码。
不要运行下面的新建密钥命令覆盖旧配置。应用密钥改变会影响既有登录和用户令牌。

**全新部署**：在部署目录生成独立随机值，命令拒绝覆盖已有 `.env`：

```bash
umask 077
(
  set -eC
  # 新实例建议 hex 密码，避免 DSN 与 .env 特殊字符转义问题。
  session_secret=$(openssl rand -hex 32)
  token_secret=$(openssl rand -hex 32)
  db_password=$(openssl rand -hex 32)
  root_password=$(openssl rand -hex 32)
  printf 'ONEHUB_IMAGE_DIGEST=\nONEHUB_SESSION_SECRET=%s\nONEHUB_USER_TOKEN_SECRET=%s\nONEHUB_MYSQL_PASSWORD=%s\nONEHUB_MYSQL_ROOT_PASSWORD=%s\n' \
    "$session_secret" "$token_secret" "$db_password" "$root_password" > .env
)
```

编辑 `.env` 的 `ONEHUB_IMAGE_DIGEST`，填入已发布且已验收镜像的 `sha256:` 加64位十六进制摘要。
不要把 CI 本地 image ID、Git SHA 或计划版本号当作 registry digest。
必填值为空时 Compose 拒绝解析；这个检查只保证非空，**不验证密钥强度**。
`.env` 不应提交或分享；本仓库 Git 和 Docker 上下文均排除该文件。
有特殊字符的既有密码需按 Compose `.env` 规则正确转义，并验证解析后的 DSN，不能为了适配示例擅自改密钥。

### 选择数据库与 Redis

| 组合 | 命令前缀 |
| --- | --- |
| MySQL + Redis（保留默认） | `docker compose` |
| SQLite，无 Redis | `docker compose -f docker-compose.sqlite.yml` |
| SQLite + Redis | `docker compose -f docker-compose.sqlite.yml -f docker-compose.redis.yml` |
| MySQL，无 Redis | `docker compose -f docker-compose.sqlite.yml -f docker-compose.mysql.yml` |

后续操作始终使用相同前缀。SQLite 组合不需要两个 MySQL 密码；它没有 `SQL_DSN` 环境变量。
同时必须核对 `./data/config.yaml`：SQLite 需要**省略** `sql_dsn`，不能设置为空字符串；
禁用 Redis 时省略 `redis_conn_string`。已有配置文件仍会被应用读取，不能只根据模板判断实际后端。
启用 Redis 时还要保留非零 `sync_frequency`（默认600）。
切换数据库类型不迁移数据，不应作为现有实例升级步骤。

### 验证与启动

以下以默认组合为例；运行前确认已经获得本次部署授权：

```bash
docker compose config --quiet
docker compose up -d --wait
docker compose ps
```

`config --quiet` 只检查配置，不输出插值后的秘密；不应把完整 `config` 或 `config --environment` 上传到工单。
MySQL 探针使用专用账号执行查询，Redis 检查实际 PING，应用依赖二者健康后启动。
默认不公开数据库/Redis主机端口，应用仍通过3000端口提供服务；对外服务应接入已有 HTTPS 反向代理并完成初始化。

模板的 MySQL/Redis 摘要是已测试的固定版本，不能据此推断你现有数据库可直接跨版本升级。
升级前核实运行版本、数据备份和回滚兼容性；回滚恢复原模板、镜像摘要与配置，数据库恢复单独决定。
PostgreSQL 或外部数据库实例需要另行准备对应配置和验收，不要启动默认 MySQL 后误认为使用了外部数据库。

隔离 CI 用四种组合实际启动候选；仅替换候选镜像为已加载本地 image ID，并隔离数据卷、容器名称、网络和主机端口。
模板健康探针与启动依赖原样执行；隔离测试关闭token编码器下载和自动价格更新。该测试不证明真实 registry digest 可拉取，也不构成生产部署验收。

## 手动部署

1. **获取源码**：从 [GitHub Releases](https://github.com/dreamvm/one-hub/releases) 下载最新的可执行文件，或者直接从源码编译。如果你选择编译源码，可以使用以下命令：

   ```shell
   git clone https://github.com/dreamvm/one-hub.git
   ```

2. **构建**：进入代码目录，构建：

   ```shell
   cd one-hub
   task build
   ```

3. **运行应用**：先配置独立密钥与实际数据库参数，再运行：

   ```shell
   ./_output/one-hub --port 3000 --log-dir ./logs
   ```

4. **访问应用**：在浏览器中访问 `http://localhost:3000` 并登录。初始账号用户名为 `root`，密码为 `123456`。

5. **重新编译**：如果需要重新编译，可以使用以下命令：

   ```shell
   task build
   ```

请确保在执行以上步骤时，你的环境已经安装了必要的工具，如 Git、Node.js、yarn 和 Go。

## 多机部署

### 准备工作

1. 确保所有服务器都安装了必要的组件：

- Docker 或 手动部署所需的组件
- Redis（如果需要使用缓存）
- MySQL 客户端（如果使用远程 MySQL）

2. 网络配置：

- 确保所有服务器能够访问主数据库
- 如果使用 Redis，确保可以访问 Redis 服务器
- 检查服务器间的防火墙设置

1. 所有服务器 `SESSION_SECRET` 设置一样的值。
2. 必须设置 `SQL_DSN`，使用 MySQL 数据库而非 SQLite，所有服务器连接同一个数据库。
3. 所有从服务器必须设置 `NODE_TYPE` 为 `slave`，不设置则默认为主服务器。
4. 设置 `SYNC_FREQUENCY` 后服务器将定期从数据库同步配置，在使用远程数据库的情况下，推荐设置该项并启用 Redis，无论主从。
5. 从服务器可以选择设置 `FRONTEND_BASE_URL`，以重定向页面请求到主服务器。
6. 从服务器上**分别**装好 Redis，设置好 `REDIS_CONN_STRING`，这样可以做到在缓存未过期的情况下数据库零访问，可以减少延迟。
7. 如果主服务器访问数据库延迟也比较高，则也需要启用 Redis，并设置 `SYNC_FREQUENCY`，以定期从数据库同步配置。
