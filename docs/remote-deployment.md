# 远程部署 Hylo Server

把 Hylo server 跑在远程服务器上，用浏览器、桌面端或笔记本上的 CLI 访问。

整体结构：

```
浏览器 / 桌面端 / CLI ──HTTPS──▶ Caddy (443, 自动证书) ──HTTP──▶ hylo (127.0.0.1:54321)
```

- hylo 只监听本机，公网只暴露 Caddy。
- TLS 证书由 Caddy 自动签发和续期，hylo 不处理证书。
- 不使用 Docker：agent CLI（claude、codex 等）需要和 hylo 在同一台机器上执行，容器里没有这些环境。

> **安全提醒**：Hylo 的 agent 接口会在服务器上执行 agent CLI，拿到 API key 等于拿到这台机器的 shell 权限。请妥善保管 key，不要把服务暴露在没有 TLS 的公网上。

## 1. 前置条件

- 一台 Linux 服务器，有 sudo 权限
- 一个解析到该服务器的域名（例如 `hylo.example.com`）
- 防火墙只放行 22、80、443，**不要放行 54321**

## 2. 安装 hylo

```sh
curl -fsSL https://raw.githubusercontent.com/skoowoo/hylo/main/install-cli.sh | sh
hylo --version
```

## 3. 配置

`~/.hylo/config.toml`：

```toml
[server]
host = "127.0.0.1"
port = 54321
# agent 长对话是 SSE 流，写超时要足够长（秒）
write_timeout = 1800

[vault]
path = "~/.hylo/root"

[plugins.git_sync]
enabled = true
remote = "git@github.com:you/vault.git"
```

无论本地还是远程，server 首次启动都会自动生成 API key 并写入 `config.toml`（权限 0600），日志里只提示执行 `hylo auth show`，不会打印 key 本身。

```sh
hylo auth show     # 查看 key 和登录地址
hylo auth rotate   # 重新生成（需重启服务生效）
```

也可以改用环境变量 `HYLO_API_KEY`（见 systemd 一节），此时 key 不会写入磁盘。保持 `host = "127.0.0.1"` 并走 Caddy，不要直接暴露 54321。

## 4. 安装 agent CLI

在服务器上安装并登录你要用的 agent CLI（例如 claude、codex），确认和运行 hylo 的是同一个系统用户：

```sh
claude --version
```

hylo 会在启动时读取该用户的登录 shell 环境，所以 PATH 和登录状态要在该用户下可用。

## 5. systemd

`/etc/systemd/system/hylo.service`（把 `youruser` 换成实际用户）：

```ini
[Unit]
Description=Hylo server
After=network-online.target
Wants=network-online.target

[Service]
User=youruser
ExecStart=/usr/local/bin/hylo start server --pid-file ""
Restart=on-failure
# 可选：用环境变量提供 key，不写入磁盘
# EnvironmentFile=/etc/hylo/env

[Install]
WantedBy=multi-user.target
```

使用 `EnvironmentFile` 时，`/etc/hylo/env` 内容为 `HYLO_API_KEY=...`，权限设为 `600`。

```sh
sudo systemctl daemon-reload
sudo systemctl enable --now hylo
journalctl -u hylo -f
```

## 6. Caddy

安装 Caddy 后，`/etc/caddy/Caddyfile`：

```
hylo.example.com {
    reverse_proxy 127.0.0.1:54321 {
        flush_interval -1
    }
}
```

```sh
sudo systemctl reload caddy
```

`flush_interval -1` 关闭响应缓冲，保证 agent 对话和收件箱通知（SSE）实时到达。Caddy 会自动设置 `X-Forwarded-For` 和 `X-Forwarded-Proto`，hylo 据此记录真实 IP，并给登录 cookie 加上 `Secure`。

## 7. 连接

### 浏览器

打开 `https://hylo.example.com`，会跳转到登录页，粘贴 `hylo auth show` 里的 key。登录后 cookie 保存 30 天；退出访问 `/logout`。

### 桌面端

桌面端可以同时连接多个 server（每个 server 是一个独立的笔记库），随时切换。应用自己部署的本地 server 会自动登录，不需要输入 key；它通过同机的 `hylo auth show --raw` 读取 key。

1. 侧栏左下角点当前 server 名称，选择 **Manage servers…**，进入应用自带的 server 管理页（它不属于任何一个 server）。点 **Add server**。
2. 填入地址 `https://hylo.example.com` 和 `hylo auth show` 里的 API key，名称可留空（默认用域名）。
3. 提交后应用会先检查地址是不是 Hylo、key 是否正确，通过后才保存。
4. 在管理页点 **Open**，或回到任意 server 后在侧栏切换器里选择它。管理页也用来改名、更换 key 和删除。key 保存在桌面端自己的配置文件里，之后切换不需要再登录。

启动页进入当前选中的 server。本机 server 没在运行时，应用会自己把它拉起来。远程 server 连不上时，启动页列出已经添加的 server，等用户改选。

允许添加 `http://` 地址，但表单会提示 key 将以明文发送，公网上请使用 `https://`。

### 本机 CLI

CLI 只访问本机的 server，通过本机 `config.toml` 里的 `host`、`port` 和 `api_key` 连接，不支持直接连远程 server。需要操作远程笔记库时请使用桌面端或浏览器。

### Clip 扩展

在扩展设置里填入服务地址和 API Key。

## 8. 日常运维

| 操作 | 命令 |
|---|---|
| 查看 key | `hylo auth show`（`--raw` 只输出 key，供脚本使用） |
| 更换 key | `hylo auth rotate`，然后 `sudo systemctl restart hylo`。旧 key 和所有浏览器会话立即失效 |
| 升级 | 重新执行安装脚本，再重启服务 |
| 日志 | `journalctl -u hylo -f` |

## 9. 备份

- 笔记内容：启用 `git_sync` 并配置 remote，vault 会自动提交和同步。
- 不在 git 里的数据：`~/.hylo/` 下的 `meta.db`、`mate.db` 和 `config.toml`，需要自行备份（例如定时 `rsync` 或快照）。搜索索引可以重建。

## 10. 安全行为说明

- 所有请求都需要 key，包括来自本机和反向代理的请求，没有「本地免认证」的例外。
- cookie 为 `HttpOnly`、`SameSite=Lax`，内容是由 key 派生的令牌，不含 key 本身。
- `PATCH /api/config` 不接受 `server.*`（监听地址、API key、证书路径、超时）。这些项决定进程暴露在哪，以及谁能在这台机器上执行命令，设置页也没有对应的编辑入口。无论请求来自本机还是远程，都返回 403；请直接改配置文件，或执行 `hylo auth rotate`，然后重启。
- `GET /api/config` 永远不返回 `api_key`（带 `reveal_secrets` 也一样），用 `hylo auth show` 查看。
- 登录没有失败限速。如果需要，请在反向代理（Caddy、Cloudflare 等）上限制 `/login` 的请求频率。
- `/healthz` 和 `/version` 不需要认证。

## 11. 不使用域名时

可以用 Tailscale 或 Cloudflare Tunnel：hylo 保持监听 `127.0.0.1`，由它们提供加密访问，不需要 Caddy 和公网端口。桌面端添加 server 时填它们提供的地址即可。
