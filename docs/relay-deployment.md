# 部署 Relay 与家庭 Connector

Relay 服务端和家庭电脑上的 Connector 都位于本仓库。家庭电脑主动连接公网 Relay，Relay 通过已建立的数据连接转发 HTTP、上传下载和 SSE。

## 公网服务器

在公网服务器上启动 `hylo start relay`。`--join-key`（或环境变量 `HYLO_RELAY_JOIN_KEY`）是必填项；没有接入 key 时进程会直接退出，避免未授权主机向 Relay 注册：

```sh
hylo start relay \
  --listen 127.0.0.1:8080 \
  --state /var/lib/hylo-relay/state.json \
  --join-key "<your-relay-join-key>"
```

Relay 会在家庭 Connector 首次携带正确接入 key 连接时登记该家庭环境，并将凭据保存在权限为 `600` 的 state 文件中。

用 Caddy 或等效 TLS 入口将域名转发到本机端口，并允许 WebSocket 升级和流式响应：

```
relay.example.com {
    reverse_proxy 127.0.0.1:8080 {
        flush_interval -1
    }
}
```

公网只暴露 HTTPS 443。Relay 本身的 `:8080` 不应直接开放。`/healthz` 仅报告进程可用，不表示家庭电脑在线。

## 家庭电脑

Hylo 默认**不开启** Relay，也不会连接任何默认公网地址。启用远程访问需要在 `config.toml`（或在设置页 **Remote access**）中显式开启并填写自建 Relay 地址与接入 key：

```toml
[relay]
enabled = true
url = "https://relay.example.com"
join_key = "<your-relay-join-key>"
```

首次开启并保存/启动时，Hylo 会自动生成 `server_id`、`connector_token` 和 `access_token` 并写入权限为 `600` 的配置文件。本地开发测试时允许使用 loopback 上的 `http://127.0.0.1:<port>`；公网地址必须使用 `https://` 或 `wss://`。

Connector 仅连接本机 `127.0.0.1:server.port`，随 Hylo 启动并优雅停止。`connector_token` 和 `access_token` 不会通过配置 API 返回。

## 验证转发

Connector 在线后，可检查在线状态并访问转发接口：

```sh
curl \
  -H 'X-Hylo-Relay-Key: <access_token>' \
  https://relay.example.com/relay/v1/servers/<server_id>/status

curl -X POST \
  -H 'X-Hylo-Relay-Key: <access_token>' \
  -H 'X-Hylo-API-Key: <Hylo API key>' \
  https://relay.example.com/s/<server_id>/api/status
```

第一个 header 由 Relay 校验，第二个 header 仍由家庭 Hylo 校验（客户端也可以使用 `Authorization: Bearer <Hylo API key>` 供 Hylo 校验，与 `X-Hylo-Relay-Key` 互不冲突）。Relay 会移除公网请求携带的转发身份头和逐跳头，避免 Connector 的 loopback 信任边界被伪造。

Relay 仅转发移动端业务白名单内的 API 路由（默认拦截 `/api/config`、`/api/server/restart`、`/mcp` 等高权限本地入口），不支持 HTTP CONNECT 或业务 WebSocket Upgrade。它支持 Hylo 现有的普通 HTTP 和 SSE；响应会立即刷新，避免 agent 事件在公网端累积。
