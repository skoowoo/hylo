# 远程部署 Hylo Server

```
桌面端 ──HTTPS──▶ 反向代理 (443, TLS) ──HTTP──▶ hylo (127.0.0.1:54321)
```

hylo 只监听本机，由反向代理对外提供 HTTPS。推荐用 Docker 部署，镜像里自带 Pi agent；也可以直接运行二进制。

> **安全提醒**：hylo 会在服务器上执行 agent，拿到 API key 等于拿到这台机器的 shell 权限。不要直接对公网暴露 54321。

## 方式一：Docker（推荐）

镜像包含 hylo 和 [Pi](https://github.com/earendil-works/pi)，不包含其他 agent CLI。

模型 key 写进 env 文件，权限设为 `600`。Pi 支持的各家模型及其变量名见 [providers.md](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/providers.md)，以 DeepSeek 为例：

```sh
# /etc/hylo/env
DEEPSEEK_API_KEY=sk-...
```

启动：

```sh
docker run -d --name hylo --restart unless-stopped \
  -p 127.0.0.1:54321:54321 \
  -v hylo-data:/data \
  --env-file /etc/hylo/env \
  ghcr.io/skoowoo/hylo:latest
```

查看 API key：

```sh
docker exec hylo hylo auth show
```

- 所有数据（配置、笔记库、Pi 的凭据和会话）都在 `hylo-data` 卷里。
- 也可以不用 env 文件，执行 `docker exec -it hylo pi`，在 Pi 里用 `/login` 登录，凭据会存进卷里。

## 方式二：二进制

```sh
curl -fsSL https://raw.githubusercontent.com/skoowoo/hylo/main/install-cli.sh | sh
```

用运行 hylo 的同一个系统用户，自行安装并登录要用的 agent CLI（Pi、Claude Code、Codex 等）。然后启动，并查看 API key：

```sh
hylo start server
hylo auth show
```

默认监听 `127.0.0.1:54321`，不需要额外配置。

## 反向代理

任选能自动签发证书的反向代理，转发到 `127.0.0.1:54321`，并关闭响应缓冲，否则 agent 对话的 SSE 流会卡住。Caddy 示例：

```
hylo.example.com {
    reverse_proxy 127.0.0.1:54321 {
        flush_interval -1
    }
}
```

没有域名时，可以改用 Tailscale 或 Cloudflare Tunnel。

## 连接

在桌面端侧栏左下角点 server 名称 → **Manage servers…** → **Add server**，填入 `https://hylo.example.com` 和 API key。
