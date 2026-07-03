# SimpleHPC 后端对接服务部署说明

更新时间：2026-06-24 22:07 CST

目标服务器：`10.10.38.152`

主机名：`cae`

系统版本：`CentOS Linux 7.9.2009 (Core)`

内核版本：`3.10.0-1160.el7.x86_64`

说明：本文件记录后端当前可对接服务的安装情况、监听端口、配置路径和验证结果。账号密码等敏感信息单独保存在 `docs/BACKEND_SERVICE_CREDENTIALS.local.md`，该文件已通过 `.gitignore` 排除。

## 总览

| 服务 | 部署方式 | 版本/镜像 | 监听地址 | 当前状态 | 用途 |
|---|---|---|---|---|---|
| PostgreSQL | Docker | `postgres:15-alpine` / PostgreSQL 15.18 | `127.0.0.1:5432` | 运行中 | SimpleHPC 业务数据库 |
| Redis | Docker | `redis:7-alpine` | `127.0.0.1:6379` | 运行中 | 缓存、会话、任务队列 |
| OpenLDAP | Docker | `osixia/openldap:1.5.0` | `127.0.0.1:389` | 运行中 | 用户/组织/团队目录服务 |
| MariaDB | Docker | `mariadb:10.11` / MariaDB 10.11.18 | `127.0.0.1:3306` | 运行中 | Slurm accounting 数据库 |
| Docker | 宿主机 | Docker 24.0.9 | 本机 daemon | 运行中/开机自启 | 容器运行时 |
| Munge | 宿主机 systemd | 0.5.11 | 本机认证服务 | 运行中/开机自启 | Slurm 认证 |
| Slurm controller | 宿主机 systemd | Slurm 26.05.1 | `*:6817` | 运行中/开机自启 | Slurm 控制服务 |
| Slurm node daemon | 宿主机 systemd | Slurm 26.05.1 | `*:6818` | 运行中/开机自启 | 单机计算节点 |
| slurmdbd | 宿主机 systemd | Slurm 26.05.1 | `*:6819` | 运行中/开机自启 | Slurm accounting API |
| Go | 宿主机 | Go 1.22.12 | 不监听 | 已安装 | 后端构建运行时 |
| Node.js/npm | 宿主机 | Node.js 20.11.1 / npm 10.2.4 | 不监听 | 已安装 | 前端构建运行时 |

## 存储目录

| 路径 | 说明 |
|---|---|
| `/data/docker` | Docker 数据根目录 |
| `/data/simplehpc/compose` | 容器环境变量和参考 compose 文件 |
| `/data/simplehpc/data/postgres` | PostgreSQL 数据目录 |
| `/data/simplehpc/data/redis` | Redis 数据目录 |
| `/data/simplehpc/data/openldap/database` | OpenLDAP 数据库目录 |
| `/data/simplehpc/data/openldap/config` | OpenLDAP 配置目录 |
| `/data/simplehpc/data/mariadb` | MariaDB/Slurm accounting 数据目录 |
| `/data/simplehpc/backups` | Slurm 状态备份目录 |
| `/opt/slurm/26.05.1` | Slurm 26.05.1 安装目录 |
| `/opt/slurm/current` | 指向当前 Slurm 版本的软链接 |
| `/etc/slurm` | Slurm 配置目录 |
| `/var/spool/slurmctld` | Slurm controller 状态目录 |
| `/var/spool/slurmd` | Slurm node daemon 状态目录 |
| `/var/log/slurm` | Slurm 日志目录 |

## 主机状态

检查时间：2026-06-24 22:05:59 CST

磁盘状态：

| 挂载点 | 容量 | 已用 | 可用 | 使用率 |
|---|---:|---:|---:|---:|
| `/` | 92G | 82G | 9.4G | 90% |
| `/data` | 200G | 97G | 104G | 49% |

注意：根分区 `/` 使用率已经到 90%，后续不要把业务数据、构建缓存、上传文件放到根分区，优先放到 `/data`。

## 容器服务

当前容器：

```text
simplehpc-mariadb    mariadb:10.11           Up    127.0.0.1:3306->3306/tcp
simplehpc-openldap   osixia/openldap:1.5.0   Up    127.0.0.1:389->389/tcp, 636/tcp
simplehpc-redis      redis:7-alpine          Up    127.0.0.1:6379->6379/tcp
simplehpc-postgres   postgres:15-alpine      Up    127.0.0.1:5432->5432/tcp
```

所有容器均设置为：

```text
restart=unless-stopped
```

容器端口目前都绑定在 `127.0.0.1`，只允许本机后端访问。若后端服务不在同一台机器，需要再做防火墙、TLS、账号权限和网络访问策略评估。

## PostgreSQL

用途：SimpleHPC 平台业务数据库。

验证结果：

```text
/var/run/postgresql:5432 - accepting connections
simplehpc|simplehpc|PostgreSQL 15.18 on x86_64-pc-linux-musl
```

当前主要角色：

```text
simplehpc|rolsuper=true|rolcreaterole=true|rolcreatedb=true|rolcanlogin=true
```

后端建议：

- 应用连接业务库 `simplehpc`。
- 当前 `simplehpc` 用户权限偏高，开发阶段可接受；生产环境建议拆分为迁移用户和运行时用户。
- 业务系统用户、角色、审计、配置等平台表放在 PostgreSQL。

## Redis

用途：缓存、会话、异步任务队列。

验证结果：

```text
PONG
```

后端建议：

- Redis 当前启用了密码。
- 只绑定 `127.0.0.1:6379`，后端本机连接即可。
- 后续可以用于登录态缓存、任务执行状态缓存、WebSocket 订阅状态等。

## OpenLDAP

用途：用户、单位、团队/组、组织关系目录服务。

基础配置：

```text
LDAP_ORGANISATION=SimpleHPC
LDAP_DOMAIN=simplehpc.local
LDAP_BASE_DN=dc=simplehpc,dc=local
LDAP_ADMIN_DN=cn=admin,dc=simplehpc,dc=local
```

验证结果：

```text
dn: dc=simplehpc,dc=local
objectClass: top
objectClass: dcObject
objectClass: organization
```

当前目录里只有基础 Base DN，还没有创建业务 OU、用户和团队。建议后端初始化时创建：

```text
ou=users,dc=simplehpc,dc=local
ou=groups,dc=simplehpc,dc=local
ou=teams,dc=simplehpc,dc=local
ou=units,dc=simplehpc,dc=local
```

## MariaDB / Slurm Accounting

用途：Slurm accounting 数据库，供 `slurmdbd` 存储作业历史、QOS、Account、Association 等调度管理数据。

数据库版本：

```text
10.11.18-MariaDB-ubu2204
```

数据库参数：

```text
innodb_buffer_pool_size=4294967296
innodb_log_file_size=67108864
innodb_lock_wait_timeout=900
max_allowed_packet=67108864
```

Slurm accounting 数据库：

```text
slurm_acct_db
```

已创建 Slurm 表，包括：

```text
simplehpc-dev_job_table
simplehpc-dev_job_env_table
simplehpc-dev_job_script_table
simplehpc-dev_step_table
simplehpc-dev_assoc_table
simplehpc-dev_event_table
```

验证结果：

```text
simplehpc-dev_job_table job_rows=1
```

后端建议：

- 可以只读查询 Slurm accounting 表以做报表和历史统计。
- 不建议后端直接写 Slurm accounting 表。
- QOS、Account、Association、用户关联等变更应通过 `sacctmgr` 或后续 Slurm 官方接口执行。

## Slurm

Slurm 版本：

```text
slurm 26.05.1
```

安装路径：

```text
/opt/slurm/26.05.1
/opt/slurm/current
```

核心配置：

```text
ClusterName=simplehpc-dev
SlurmctldHost=cae(10.10.38.152)
SlurmUser=slurm
AuthType=auth/munge
StateSaveLocation=/var/spool/slurmctld
SlurmdSpoolDir=/var/spool/slurmd
SlurmctldPort=6817
SlurmdPort=6818
SchedulerType=sched/backfill
SelectType=select/cons_tres
SelectTypeParameters=CR_Core_Memory
AccountingStorageType=accounting_storage/slurmdbd
AccountingStorageHost=127.0.0.1
AccountingStoragePort=6819
JobAcctGatherType=jobacct_gather/linux
JobAcctGatherFrequency=30
NodeName=cae CPUs=20 Boards=1 SocketsPerBoard=20 CoresPerSocket=1 ThreadsPerCore=1 RealMemory=30000 State=UNKNOWN
PartitionName=debug Nodes=cae Default=YES MaxTime=INFINITE State=UP
```

当前节点：

```text
NODELIST   PARTITION  STATE  CPUS  MEMORY
cae        debug*     idle   20    30000
```

当前队列：

```text
squeue: no running or pending jobs
```

已验证作业历史：

```text
JobID=2
JobName=simplehpc-accounting-smoke.sbatch
Partition=debug
Account=simplehpc
AllocCPUS=1
State=COMPLETED
Elapsed=00:00:01
Submit=2026-06-24T21:59:25
Start=2026-06-24T21:59:25
End=2026-06-24T21:59:26
```

当前 Slurm account/QOS：

```text
Accounts:
root
simplehpc

Users:
root

Associations:
simplehpc-dev/root
simplehpc-dev/root/root
simplehpc-dev/simplehpc
simplehpc-dev/simplehpc/root

QOS:
normal
```

## slurmdbd

配置文件：

```text
/etc/slurm/slurmdbd.conf
```

核心配置：

```text
AuthType=auth/munge
DbdHost=cae
DbdAddr=127.0.0.1
DbdPort=6819
SlurmUser=slurm
StorageType=accounting_storage/mysql
StorageHost=127.0.0.1
StoragePort=3306
StorageLoc=slurm_acct_db
StorageUser=slurm
```

服务状态：

```text
slurmdbd=active
slurmctld=active
slurmd=active
```

说明：接入 slurmdbd 时曾出现一次 `CLUSTER ID MISMATCH`，原因是 `slurmctld` 在未接 accounting 前已经生成本地 ClusterID。已备份 `/var/spool/slurmctld` 后修正为数据库中的 ClusterID，当前已经恢复正常。

备份文件：

```text
/data/simplehpc/backups/slurmctld-state-before-accounting-20260624215851.tar.gz
```

## 系统用户

宿主机服务用户：

```text
slurm:x:986:980::/var/lib/slurm:/sbin/nologin
munge:x:987:981:Runs Uid 'N' Gid Emporium:/var/run/munge:/sbin/nologin
```

说明：

- `slurm` 用于运行 `slurmctld` 和 `slurmdbd`。
- `slurmd` 当前由 root 启动，这是常见部署方式，便于作业进程切换用户。
- `munge` 用于 Munge 认证服务。

## 后端对接建议

第一阶段建议后端这样接：

| 功能 | 推荐对接方式 |
|---|---|
| 平台用户、角色、配置、审计 | PostgreSQL |
| 登录、用户目录、团队/单位目录 | OpenLDAP + PostgreSQL 映射表 |
| 缓存、会话、任务状态 | Redis |
| Slurm 当前节点/队列/作业 | `sinfo`、`squeue`、`scontrol` |
| 作业提交/取消/详情 | `sbatch`、`scancel`、`scontrol show job` |
| 作业历史 | `sacct` 或只读查询 MariaDB Slurm accounting 表 |
| QOS/Account/Association 管理 | `sacctmgr` |
| 文件目录和上传下载 | 后续接 NFS/GPFS 或本机指定存储路径 |

## 待补事项

1. `slurmrestd` 未构建：当前缺 `http-parser/json-c` 等开发包。第一版可以先走 Slurm CLI 和 `sacctmgr`，后续如需要 REST API 再补依赖重编。
2. NFS/GPFS 尚未接入：需要确认实际共享存储挂载路径后再做目录权限和用户 Home/项目目录创建策略。
3. OpenLDAP 尚未初始化业务 OU 和用户：后端初始化脚本需要补。
4. 根分区空间偏紧：构建缓存和上传文件必须放 `/data`。
5. PostgreSQL 当前业务用户权限较高：生产前建议拆分权限。
