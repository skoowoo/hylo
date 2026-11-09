# Bots

Bot 是你在 **Settings → Bots** 里定义的 AI agent：一个名字、一段 system prompt、背后驱动它的 agent CLI，以及让它自己跑起来的触发器（可选）。

## 底层 Agent

Hylo 不自带模型。启动时会从 `PATH` 里自动发现这些 agent CLI，不需要任何配置：

- Claude Code
- OpenCode
- Codex CLI
- Cursor Agent
- GitHub Copilot CLI
- Pi

## Hylo bot

桌面端 onboarding 会创建主 bot **Hylo**。如果当时跳过了，或者 Hylo 部署在服务器上，打开 **Settings → Bots**，还没有 bot 时会带你走一遍同样的设置。它自带这些触发器：

| 触发器                          | 作用                                          |
| ------------------------------- | --------------------------------------------- |
| `compile_requested`             | 你在 app 里手动发起编译时编译笔记             |
| `note_created`（`/Web Clips/`） | 新剪藏自动编译                                |
| `scheduled` 每天 09:00          | 更新[个人记忆](./context_zh.md#个人记忆)      |

onboarding 之后的设置对话还会帮你建一个 **Recall** bot，把你写过的东西带回给你：每周一回顾上一周，或者每天下午在历史笔记里随机走一遭，也可以两个都要。

这些都可以修改或删除，也可以点 **New Bot** 自己建。

## 触发器

| 事件                 | 什么时候触发                     |
| -------------------- | -------------------------------- |
| `note_created`       | vault 里新建了笔记               |
| `note_updated`       | 有笔记被修改                     |
| `note_deleted`       | 有笔记被删除                     |
| `short_note_created` | 加了一条速记                     |
| `scheduled`          | 按间隔、每天某时或每周某时触发   |
| `compile_requested`  | 你在 app 里发起了编译            |

笔记类触发器可以用 **Path Prefixes** 限定目录（如 `/journal/`）。

## 微信、Discord、Telegram

每个 bot 可以接自己的一个联系人。那边发来的消息写进这个 bot 当前的对话，和桌面上是同一场。发 `/new` 会新开一场，桌面也会跟着切过去。

在 **Settings → Bots → Channels** 里设置：

- **微信**：点 Connect，扫码。一个微信号只能留在一个 bot 上，再次扫码会断开之前那个。
- **Discord**：在 [Developer Portal](https://discord.com/developers/applications) 建一个 Bot，打开 **Message Content Intent**，填 Bot token 和你的用户 ID（开发者模式 → 右键头像 → 复制用户 ID）。
- **Telegram**：给 [@BotFather](https://t.me/BotFather) 发 `/newbot`，粘贴 token，用手机打开链接并点 Start。

只有完成连接的那个账号能跟它说话。连上之后不用重启。

渠道连不上网络时，在 **Settings → Server → Config → Proxy** 填代理（例如 `http://127.0.0.1:7890`），然后重启。

## 自定义 prompt

全局 system prompt 是 Hylo 自带的。每个 bot 有两处可以自己写：

- **System Prompt**：附加在全局 prompt 之后，用 `---` 分隔。
- **Trigger Prompt**：触发器触发时发给 agent 的消息。变量：笔记事件用 `{Path}`、`{Name}`、`{Content}`；定时触发用 `{Now}`、`{Date}`、`{Time}`。

`hylo-compile-note`、`hylo-memory` 这类内置 skill 每次启动都会被重写，改了也留不住。想改变 bot 的做法，用上面两处 prompt，或者自己写一个 skill（见 [Skills](./skills_zh.md#管理-skill)）。
