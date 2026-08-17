# Private Chat 部署实施文档（install.md）

> 适用版本：v1.0.0
> 适用场景：关爱通风格私有聊天服务（单房间、WebSocket 实时通信、4 小时消息自动清理、Emoji/表情包、图片/文件上传）
> 目标环境：**2 核 4GB 云服务器（Linux x86_64）**

---

## 1. 产品与架构概述

| 项 | 说明 |
| --- | --- |
| 语言/框架 | Go 1.26 + Gin |
| 数据库 | SQLite（WAL 模式，纯 Go 驱动 `modernc.org/sqlite`，**无 CGO 依赖**） |
| 通信 | WebSocket（`/ws`），HTTP 长轮询回退由前端断线重连实现 |
| 进程模型 | **单一可执行文件** + 本地数据目录，无外部中间件 |
| 资源占用 | 空闲约 20–40 MB 内存；4GB 内存下可轻松承载数十人在线 |
| 对外端口 | `8080`（HTTP） |

**核心特性对应 PRD：**
- 登录/登出、管理员初始化、用户增删改查（管理后台 `/admin`）
- 文本消息 + Emoji 选择器 + 四类表情包（爱情/可通/趣味/情调）
- 图片上传（预览，≤10MB）、文件上传（≤50MB，类型白名单 + MIME 双重校验）
- **消息保留 4 小时后由后台 Worker 自动清理**（数据库 + 磁盘文件最终一致）
- 安全：Argon2id 口令哈希、HttpOnly Session Cookie、XSS 转义、参数化 SQL

> 整个前端（HTML 模板、CSS、JS、表情包 SVG）已通过 `//go:embed` **编译进二进制**，部署只需一个文件。

---

## 2. 服务器最低要求

| 资源 | 最低 | 推荐（本场景） |
| --- | --- | --- |
| CPU | 1 核 | 2 核 |
| 内存 | 512 MB | 4 GB |
| 磁盘 | 1 GB | 10 GB（消息 4h 清理，实际占用很小） |
| 系统 | Linux x86_64（CentOS 7+/Ubuntu 20.04+/Debian 11+） | 同左 |
| 运行方式 | Docker 或 直接运行二进制 | 二选一 |
| 外网 | 需开放 8080（或经反代 80/443） | 同左 |

---

## 3. 部署方式一：Docker Compose（推荐，最简单）

适合不想在服务器上装 Go 工具链、希望一条命令起停的场景。

### 3.1 安装 Docker 与 Compose

```bash
# Docker（以 Ubuntu/Debian 为例）
curl -fsSL https://get.docker.com | sudo sh
sudo systemctl enable --now docker

# docker compose 插件（Docker 24+ 自带；若缺则安装）
sudo apt-get install -y docker-compose-plugin
docker compose version   # 确认可用
```

### 3.2 准备配置与密钥

```bash
cd /opt
git clone <你的仓库地址> private-chat && cd private-chat
# 或：将本项目目录 scp 到服务器 /opt/private-chat

# 复制环境变量模板，必须修改密码
cp .env.example .env
vim .env
```

`.env` 内容示例（**切勿使用默认弱口令**）：

```dotenv
ADMIN_USERNAME=admin
ADMIN_PASSWORD=请改成强密码-不少于12位
SECURITY_COOKIE_SECURE=false   # 配好 HTTPS 反代后改 true
LOG_LEVEL=info
LOG_FORMAT=json
```

> 首次启动时，程序检测到数据库内无管理员，会用 `ADMIN_USERNAME/ADMIN_PASSWORD` **自动创建**管理员。此后即使改动 `.env` 也不再生效（需在 `/admin` 后台改密）。

### 3.3 构建并启动

```bash
docker compose up -d --build
docker compose logs -f          # 观察启动日志，看到 "admin created" / "http server listening" 即成功
```

- 数据持久化：容器内 `/data` 已挂载到宿主机 `./data`，包含 `chat.db` 与 `uploads/`
- 健康检查：容器内置 `/health` 探活（busybox wget）
- 访问：浏览器打开 `http://<服务器IP>:8080`

### 3.4 常用命令

```bash
docker compose ps
docker compose restart
docker compose down             # 停止并移除容器（数据保留在 ./data）
docker compose down -v          # 连同数据卷删除（危险！）
```

---

## 4. 部署方式二：二进制 + systemd（最轻量，最契合 2C4G）

适合希望资源占用最小、用原生 systemd 管理开机自启的场景。

### 4.1 在任意机器构建静态二进制

> 中国大陆构建请先设置代理：`export GOPROXY=https://goproxy.cn,direct && export GOSUMDB=off`

```bash
# 在装有 Go 1.26+ 的机器上
git clone <仓库> private-chat && cd private-chat
make build
# 产物：./private-chat （CGO_ENABLED=0 的静态可执行文件，约 26MB）
```

将二进制与配置上传到服务器：

```bash
scp private-chat configs/config.yaml user@<服务器IP>:/opt/private-chat/
```

### 4.2 在服务器上部署

```bash
ssh user@<服务器IP>
sudo mkdir -p /opt/private-chat /opt/private-chat/data
sudo mv ~/private-chat /opt/private-chat/ 2>/dev/null
sudo cp configs/config.yaml /opt/private-chat/config.yaml
sudo chmod +x /opt/private-chat/private-chat
```

创建 systemd 服务（`/etc/systemd/system/private-chat.service`）：

```ini
[Unit]
Description=Private Chat Service
After=network.target

[Service]
Type=simple
User=private
WorkingDirectory=/opt/private-chat
ExecStart=/opt/private-chat/private-chat -config /opt/private-chat/config.yaml
# 首次启动用环境变量初始化管理员（仅当库内无管理员时生效）
Environment=ADMIN_USERNAME=admin
Environment=ADMIN_PASSWORD=请改成强密码-不少于12位
Environment=SECURITY_COOKIE_SECURE=false
# 低配服务器建议限制 P 核数，避免与系统争抢
Environment=GOMAXPROCS=2
Restart=on-failure
RestartSec=3

[Install]
WantedBy=multi-user.target
```

```bash
sudo useradd -r -s /usr/sbin/nologin private 2>/dev/null
sudo chown -R private:private /opt/private-chat
sudo systemctl daemon-reload
sudo systemctl enable --now private-chat
sudo systemctl status private-chat
```

- 访问：`http://<服务器IP>:8080`
- 日志：`journalctl -u private-chat -f`

---

## 5. 首次使用与初始化

1. 浏览器打开 `http://<服务器IP>:8080` → 自动跳转 `/login`
2. 用 `.env` / systemd 中设置的 **管理员账号** 登录
3. 进入 `/admin` 后台：
   - 创建聊天用户（普通成员），可重置其会话、删除用户
   - 查看在线统计
4. 成员用各自账号登录后进入 `/chat` 即可实时聊天

> 默认只有一个房间（PRD 单房间模型），所有登录用户共享同一对话。

---

## 6. 配置说明

### 6.1 配置文件 `configs/config.yaml`

命令行默认加载 `-config configs/config.yaml`，**环境变量可覆盖任意字段**。

```yaml
server:
  host: "0.0.0.0"
  port: 8080
database:
  path: "./data/chat.db"
storage:
  upload_dir: "./data/uploads"
  max_image_size: 10485760     # 10MB
  max_file_size: 52428800     # 50MB
chat:
  message_retention_hours: 4   # 4 小时后自动清理
session:
  expiration_hours: 24
security:
  cookie_secure: false         # HTTPS 反代后改 true
log:
  level: "info"                # debug|info|warn|error
  format: "json"               # json|text
```

### 6.2 环境变量（优先级高于配置文件）

| 变量 | 作用 | 示例 |
| --- | --- | --- |
| `ADMIN_USERNAME` | 首次启动创建管理员用户名（仅首次） | `admin` |
| `ADMIN_PASSWORD` | 首次启动创建管理员口令（仅首次） | `强密码` |
| `SERVER_HOST` | 监听地址 | `0.0.0.0` |
| `SERVER_PORT` | 监听端口 | `8080` |
| `DATABASE_PATH` | SQLite 文件路径 | `/data/chat.db` |
| `STORAGE_UPLOAD_DIR` | 上传根目录 | `/data/uploads` |
| `STORAGE_MAX_IMAGE_SIZE` | 图片上限（字节） | `10485760` |
| `STORAGE_MAX_FILE_SIZE` | 文件上限（字节） | `52428800` |
| `CHAT_RETENTION_HOURS` | 消息保留小时数 | `4` |
| `SESSION_EXPIRATION_HOURS` | 会话有效期（小时） | `24` |
| `SECURITY_COOKIE_SECURE` | Cookie 仅 HTTPS（`true`/`1`） | `false` |
| `LOG_LEVEL` / `LOG_FORMAT` | 日志级别 / 格式 | `info` / `json` |

---

## 7. 生产加固：HTTPS 反向代理

程序本身只提供 HTTP。对外暴露 80/443 必须由前置反代完成（推荐 Nginx 或 Caddy）。

### 7.1 Nginx 示例

```nginx
server {
    listen 80;
    server_name chat.example.com;
    # HTTP 强制跳转 HTTPS（可选）
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl;
    server_name chat.example.com;

    ssl_certificate     /etc/nginx/certs/fullchain.pem;
    ssl_certificate_key /etc/nginx/certs/privkey.pem;

    client_max_body_size 60m;   # 须大于最大文件 50MB

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;        # WebSocket 必需
        proxy_set_header Connection "upgrade";        # WebSocket 必需
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 3600s;                      # 保活长连接
    }
}
```

完成后，将 `SECURITY_COOKIE_SECURE` 改为 `true`（Docker：`.env`；systemd：`Environment=`），并重启服务，使 Session Cookie 仅经 HTTPS 传输。

### 7.2 证书自动续期（可选）

使用 Caddy 可零配置自动申请 Let's Encrypt 证书：

```caddyfile
chat.example.com {
    reverse_proxy 127.0.0.1:8080
}
```

---

## 8. 2 核 4GB 资源调优建议

| 项目 | 建议 |
| --- | --- |
| Go 调度 | 设置 `GOMAXPROCS=2`，避免与系统进程争抢 CPU |
| 交换分区 | 建议新增 1–2GB swap，防止上传大文件时瞬时 OOM |
| SQLite | 已启用 WAL + `busy_timeout=5000ms` + `synchronous=NORMAL`，并发安全；连接池上限 4，契合低配 |
| 上传内存 | 文件以流式读取、上限 50MB，不会一次性占满内存 |
| 反向代理 body | Nginx `client_max_body_size` 调到 60MB 以上，否则大文件被拒 |
| 防火墙 | 只开放 80/443（反代）或 8080（直连）；其余端口关闭 |

新增 swap 示例：

```bash
sudo fallocate -l 2G /swapfile
sudo chmod 600 /swapfile
sudo mkswap /swapfile
sudo swapon /swapfile
echo '/swapfile none swap sw 0 0' | sudo tee -a /etc/fstab
```

---

## 9. 数据备份与恢复

数据全部位于数据目录（Docker：`./data`；二进制：`/opt/private-chat/data`）。

**备份（建议每日定时任务）：**

```bash
# 停止写入或低峰期执行；SQLite 也可在线热备
cp -a /opt/private-chat/data /backup/private-chat-data-$(date +%F)
# 或仅备份数据库（WAL 模式下需连同 -wal/-shm）
sqlite3 data/chat.db ".backup '/backup/chat-$(date +%F).db'"
```

需备份的关键文件：
- `chat.db`、`chat.db-wal`、`chat.db-shm`（数据库）
- `uploads/`（所有上传的图片与文件）

**恢复：** 停止服务 → 用备份覆盖整个 `data/` 目录 → 启动服务。

---

## 10. 升级流程

```bash
cd /opt/private-chat
git pull                      # 或重新 scp 新二进制
docker compose up -d --build  # Docker 方式
# 或二进制方式：
sudo systemctl stop private-chat
cp new-private-chat /opt/private-chat/private-chat
sudo systemctl start private-chat
```

> 数据库向后兼容：迁移脚本在启动时自动执行（`migrations/*.sql`，含 `IF NOT EXISTS`），升级无需手动改表。

---

## 11. 日志与监控

- 默认 JSON 格式输出到 stdout：
  ```
  {"level":"info","msg":"http server listening","addr":"0.0.0.0:8080","ts":"..."}
  {"level":"info","msg":"admin created from env","username":"admin","ts":"..."}
  ```
- Docker：`docker compose logs -f`
- systemd：`journalctl -u private-chat -f`
- 探活接口：`GET /health` → `200 {"code":0,"message":"ok","data":{"status":"healthy"}}`
- 可用 `curl -f http://127.0.0.1:8080/health` 接入节点监控（如 Prometheus blackbox、Zabbix）。

---

## 12. 常见问题排查（FAQ）

| 现象 | 原因 / 处理 |
| --- | --- |
| 启动报 `address already in use` | 8080 被占用；改 `SERVER_PORT` 或停掉冲突进程 |
| 首次启动未创建管理员 / 登录失败 | 确认 `ADMIN_USERNAME/ADMIN_PASSWORD` 已设置且库内无管理员；已存在管理员则改密需在 `/admin` |
| 上传大文件被拒（413） | 反代 `client_max_body_size` 过小，调到 60MB+ |
| WebSocket 连上即断 | 反代未透传 `Upgrade/Connection` 头或未设 `proxy_read_timeout` |
| HTTPS 下登录循环 | `SECURITY_COOKIE_SECURE` 未设为 `true`，Cookie 被浏览器拒 |
| 数据目录无写入权限 | 确保运行用户对 `data/` 有写权限（`chown`） |
| 表情包/静态资源 404 | 资源已编译进二进制，无需额外拷贝；确认使用的是新构建的二进制 |
| 国内 `go mod download` 超时 | 设置 `GOPROXY=https://goproxy.cn,direct` 与 `GOSUMDB=off` |

---

## 13. 安全提示

- **务必修改默认管理员口令**，且长度 ≥12 位。
- 生产环境一律走 HTTPS，并设 `SECURITY_COOKIE_SECURE=true`。
- 上传目录禁止执行：确保 `uploads/` 不被 Web 服务器直接当作脚本目录执行（本程序通过数据库记录 + 随机文件名服务，不暴露原始路径）。
- 防火墙仅开放必要端口；数据库文件 `chat.db` 不应直接暴露公网。
- 消息与文件 4 小时自动清理，敏感内容不会长期留存（符合 PRD 隐私设计）。

---

## 附：仓库交付物清单

```
private-chat/
├── Dockerfile              # 多阶段构建，CGO_ENABLED=0 静态镜像
├── docker-compose.yml      # 一键部署（含健康检查、数据卷）
├── .env.example            # 环境变量模板
├── .dockerignore
├── .gitignore
├── Makefile                # build / run / test / docker / compose-*
├── configs/config.yaml     # 配置样例
├── cmd/server/main.go      # 程序入口（含优雅关闭）
├── internal/               # 业务代码（config/db/repo/auth/ws/file/cleanup/server）
├── migrations/             # （编译进二进制）数据库迁移
└── install.md              # 本文档
```
