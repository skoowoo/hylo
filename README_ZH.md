# Hylo：由你的笔记驱动的个人 AI agent，全天在线，兼容 Obsidian。

你只管写。Hylo 从笔记里了解你是谁，然后全天候为你做事：把笔记整理成知识、基于笔记回答问题，也做笔记之外的事，比如挑出你会感兴趣的新闻。

## 目录

- [它能做什么](#它能做什么)
- [设计](#设计)
- [架构](#架构)
- [安装](#安装)
- [Obsidian 兼容](#obsidian-兼容)
- [编辑器](#编辑器)
- [Bots](#bots)
- [微信](#微信)
- [Discord](#discord)
- [Telegram](#telegram)
- [LLM-Wiki 编译器](#llm-wiki-编译器)
- [个人记忆（Memory）](#个人记忆memory)
- [Skills](#skills)（[完整指南](./docs/skills_zh.md)）
- [自定义 AI 行为](#自定义-ai-行为)

## 它能做什么

#### 📝 完整的笔记软件
- **内置 WYSIWYG 编辑器**：富文本和原始 Markdown 随时切换
- **Wikilinks 和 Wiki 图片**：完全兼容 Obsidian
- **Shorts 速记流**：随手记的日常捕捉流，随时打开
- **即时搜索**，Raycast 那种体验
- **Clip 浏览器扩展**，一键把网页存成 Markdown

#### 🤖 事件驱动的多 Agent 系统
- **集成了 6 个 agent CLI**，直接复用你本地的 Agent CLI
- **事件触发自动化**，笔记一创建、消息一来，agent 自动跑起来
- **微信、Discord & Telegram 直连**，直接在微信、Discord 或 Telegram 私信里和 agent 对话

#### 🧠 LLM-Wiki 编译器
- 把笔记编译成**结构化知识**，构建互联的 **LLM Wiki 网络**

#### 🪞 个人记忆（Memory）
- 从你的笔记中自动生成**个人记忆**——身份、偏好、目标、信念、人际关系、当前状态，写进结构化文件，随时间持续更新
- 让 agent 真正**记住你、认识你**，每次对话都有记忆，不再是陌生人

#### ☁️ 全天在线，自主部署
- 把 Hylo 部署到家里的服务器或云主机上，agent **7×24 小时在线**，合上笔记本也不停
- 定时 bot 照常运行，微信、Discord、Telegram 随时找得到它
- 数据始终在你自己的机器上：Markdown 文件、SQLite 元数据、全文索引。参见[远程部署](./docs/remote-deployment.md)

#### 🌐 多端都能用
- **CLI**、**桌面 App**、**微信**、**Discord**、**Telegram**，随便选

## 架构

```
┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐
│              │  │              │  │              │  │              │
│  Desktop App │  │    WeChat    │  │     CLI      │  │  Clip (Ext)  │
│              │  │              │  │              │  │              │
└──────┬───────┘  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘
       │                 │                 │                 │
       └─────────────────┴─────────────────┴─────────────────┘
                                  │
                                  ▼
┌────────────────────────────────────────────────────────────────────┐
│                           Hylo Server                              │
└──────┬─────────────────┬─────────────────┬─────────────────┬───────┘
       │                 │                 │                 │
       ▼                 ▼                 ▼                 ▼
┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐
│  Filesystem  │  │    SQLite    │  │    Bleve     │  │    Agents    │
│   (Markdown) │  │  (Metadata)  │  │  (FTS Index) │  │  (CLI/MCP)   │
└──────────────┘  └──────────────┘  └──────────────┘  └──────────────┘
```

**Hylo Server** 是一个独立的 Go 二进制程序，没有外部依赖。SQLite 和 Bleve 都直接内嵌其中，整个服务端就是一个可执行文件，放哪里跑哪里。

它本质上是一个普通的 HTTP 服务，可以跑在本地，也可以部署到远程云服务器或容器里。所有客户端（桌面 App、CLI、微信桥接）通过网络连接，方式完全一样。

#### ⚠️ 使用 Hylo，你必须知道的那些事

- **不推荐嵌套目录**。Hylo 推荐单层扁平目录结构，不要在分类目录下再建子目录（技术上虽然可以，但强烈不建议）。
- **文件名最好唯一**。Hylo 用 Wiki Link `[[stem]]` 格式引用笔记，链接的是文件名而非路径，文件名重复会导致引用歧义。
- **下划线前缀是系统目录**。`_knowledge/`、`_shorts/`、`_memory/` 这类以 `_` 开头的目录是 Hylo 内部使用的，你自己的分类目录不要用下划线前缀。
- **Hylo 本身不内置 AI Agent**。需要你的电脑上已安装 agent 环境（如 Claude Code、OpenCode、Codex 等）。Hylo 会自动从 PATH 中发现它们，无需额外配置。查看[支持的底层 Agent](#底层-agent)。

## 安装

> **上手只需要安装桌面 App 和 Clip 扩展**。桌面 App 首次启动时会自动检测 CLI，未安装时提供一键安装。想让 agent 7×24 小时在线，再把 Server 部署到远程或家里的主机上（参见[远程部署](./docs/remote-deployment.md)）。

### 💻 桌面 App

1. 去 [最新发布页](https://github.com/skoowoo/hylo/releases/latest) 下载 `.dmg`
2. 打开 dmg，把 Hylo 拖进应用程序文件夹
3. 第一次启动 macOS 可能拦截（签名了但没公证）

   解决也很简单：**系统设置 → 隐私与安全性**，找到拦截提示点**仍要打开**就好

4. App 会把 `hylo` CLI 装到 `~/.local/bin/hylo`。把这个路径加到你的 shell 配置里，agent 才能正常调用它：

   ```sh
   # zsh — 加到 ~/.zshrc 或 ~/.zprofile
   export PATH="$HOME/.local/bin:$PATH"

   # bash — 加到 ~/.bash_profile 或 ~/.bashrc
   export PATH="$HOME/.local/bin:$PATH"
   ```

   加完重启终端（或者执行 `source ~/.zshrc` / `source ~/.bash_profile`）。

### 🧩 浏览器扩展（Clip）

1. 同样去 [最新发布页](https://github.com/skoowoo/hylo/releases/latest) 下载 `hylo-clip-*.zip`
2. 解压
3. 打开 Chrome/Edge，地址栏输 `chrome://extensions/`
4. 右上角打开**开发者模式**
5. 点**加载已解压的扩展程序**，选解压后的文件夹
6. 装好了，工具栏里能看到图标

### ⌨️ Remote Server

一行命令搞定：

```sh
curl -sL https://raw.githubusercontent.com/skoowoo/hylo/main/install-cli.sh | sh
```

要在远程主机上通过 HTTPS 提供服务，按[远程部署](./docs/remote-deployment.md)操作。然后在桌面 App 里打开服务器切换器 → **Manage servers…** → **Add a remote server**，填入那台主机上 `hylo auth show` 输出的 key。

## Obsidian 兼容

Hylo 与 Obsidian **完全兼容**，二者可以在**同一个 vault 上同时并存**——不需要迁移，也不必二选一。

- **同一份笔记，两个工具共用。** Hylo 直接读写你现有的 Markdown 文件和目录结构。继续用 Obsidian 当主编辑器，完全没问题。
- **原生 Obsidian 语法。** Wikilink（`[[笔记名]]`）、别名（`[[页面|别名]]`）、Wiki 图片（`![[图片.png]]`）开箱即用。
- **无需格式转换。** Hylo 创建或修改的内容都是标准 Markdown，在 Obsidian 里打开毫无异样。
- **Hylo 做 AI 补充。** 把你的 Obsidian vault 交给 Hylo，让它负责 AI 能力：LLM Wiki 编译、个人记忆、事件驱动的 Agent、微信接入、全文搜索。你照常用 Obsidian 写作，Hylo 在上面跑自动化。

#### 用现有 vault 开始使用

把 `hylo init` 指向你的 Obsidian vault 目录：

```sh
# 在当前目录初始化
hylo init

# 或者指定路径
hylo init /path/to/your/obsidian-vault
```

跑完之后，Hylo 会在目录里创建 `.hylo/` 文件夹、扫描所有 Markdown 文件、建好全文搜索索引。如果 `.hylo/` 已经存在，命令什么都不会动。你的 Obsidian vault 本身不会被改动。

然后打开 **Hylo App** 完成最后一步：

1. **Settings → Server → Config**
2. **Vault** 那里选你的 vault 目录
3. 保存

笔记会出现在 Hylo 里——Obsidian 照旧可以读写同一个 vault。

## 编辑器

Hylo 内置 WYSIWYG Markdown 编辑器，打开笔记默认进入富文本模式，随时可以切换为原始 Markdown。

#### ✍️ 写作

编辑器支持完整的 CommonMark 和 GFM 语法：标题、加粗、斜体、删除线、行内代码、代码块、表格、任务列表。YAML frontmatter 原样保留，不会被修改。

#### 🔗 Wikilinks 与 Wiki 图片

输入 `[[` 插入 wikilink，别名写法与 Obsidian 一致：`[[页面名|别名]]`。图片用 `![[文件名.png]]` 嵌入。

## Bots

Bot 就是你在 **Settings → Bots** 里养的私人 AI。每个 bot 有自己的名字、系统 prompt，以及背后驱动它的 agent CLI。

#### 🔔 事件触发器

Bot 最有意思的地方是事件驱动：配好触发器，你什么都不用管，它自己跑。内置的事件有这些：

| 事件                 | 什么时候触发                 |
| -------------------- | ---------------------------- |
| `note_created`       | vault 里新建了笔记           |
| `note_updated`       | 有笔记被改了                 |
| `note_deleted`       | 有笔记被删了                 |
| `short_note_created` | 加了一条速记                 |
| `scheduled`          | 按你设定的时间或间隔定时触发 |
| `compile_requested`  | 手动触发了 LLM Wiki 编译     |

#### 🧩 Hylo bot

桌面端在 onboarding 时会自动创建一个 **Hylo** bot。如果当时跳过了，或者是部署在服务器上的 Hylo，打开 **Settings → Bots**：还没有 bot 时会带你走一遍同样的设置流程。这个 bot 自带这些触发器：

| 触发器                              | 作用                                     |
| ----------------------------------- | ---------------------------------------- |
| `compile_requested`                 | 手动发起编译时，把笔记编译成知识单元     |
| `note_created`（`/Web Clips/`）     | 剪藏的网页自动编译                       |
| `scheduled` 每天 09:00              | 更新个人记忆                             |

可以在 **Settings → Bots** 里修改或删除，也可以点 **New Bot** 自己建。onboarding 之后的设置对话还会帮你建一个 **Recall** bot，把你写过的东西带回给你，三种方式任选：每周一回顾上一周写下的内容，每周六随机翻出几条可能已经忘了的旧笔记，新速记自动与知识库交叉比对。

#### 底层 Agent

> **Hylo 直接从本地 `PATH` 发现可用的 agent CLI，不用任何额外配置。** 你已经在用 Claude Code 写代码？在终端里跑 Codex 或 Copilot？Hylo 启动时自动找到它们，直接拿来用。你的工具，你的习惯，Hylo 不折腾你。

开箱集成 **6 个 agent CLI**：

- Claude Code
- OpenCode
- Codex CLI
- Cursor Agent
- GitHub Copilot CLI
- Pi

## 微信、Discord、Telegram

每个 bot 可以接自己的一个联系人。那边发来的消息写进这个 bot 当前的对话，和在桌面上聊天是同一场。发 `/new` 会新开一场，桌面也会跟着换到这场。

在 **Settings → Bots** 里打开这个 bot，找到 **Channels**。

- **微信**：点 Connect，扫码。一个微信号只能留在一个 bot 上，再次扫码会把之前那个断开。
- **Discord**：在 [Developer Portal](https://discord.com/developers/applications) 建一个 Bot，打开 **Message Content Intent**，填 Bot token 和你的用户 ID（开发者模式 → 右键头像 → 复制用户 ID）。访问不通时可以填代理，例如 `http://127.0.0.1:7890`。
- **Telegram**：给 [@BotFather](https://t.me/BotFather) 发 `/newbot`，粘贴 token，用手机打开链接并点 Start。访问不了 `api.telegram.org` 时先填代理。

一个联系人只能绑一个 Hylo bot。只有完成连接的那个账号能跟它说话，其他人的消息会被丢掉。连上之后不用重启。

## LLM-Wiki 编译器

让 AI 自动把笔记编译成 `/_knowledge` 下的结构化知识单元，构建互联的 LLM Wiki 网络，不用自己整理。

#### 第一步：打开编译器

**Settings → Server → Config → Compile** 开启，默认就是开的。

#### 第二步：用 Bot 编译

onboarding 创建的 **Hylo** bot 已经带了两个编译触发器：

| 触发器              | 什么时候触发                                         |
| ------------------- | ---------------------------------------------------- |
| `compile_requested` | 你在 app 里手动发起编译（如笔记操作菜单）            |
| `note_created`      | `/Web Clips/` 下有新笔记，剪藏的网页自动编译         |

想让别的 bot 来编译，在 **Settings → Bots** 里给它加上这两个触发器即可。

每次运行会先用 `hylo-compile-note` skill 编译笔记，再用 `hylo-index-knowledge` 更新知识索引，最后用 `humanizer` 润色新生成的知识单元。

想让其他目录也自动编译，编辑 `note_created` 触发器的 **Path Prefixes**（比如 `/journal/`）即可。

## 个人记忆（Memory）

Hylo 能从你的笔记里提取个人记忆，生成七个结构化文件（身份、偏好、目标、信念、人际关系、当前状态、文风），存在 `/_memory/` 下。用 AI 帮你"认识自己"，听起来有点玄，但挺实用的。

默认只扫**速记**（`/_shorts`）和**知识库**（`/_knowledge`）。如果你想多扫几个目录，在 prompt 里说一句就行。

#### ⏰ 方式一：每日定时（推荐）

onboarding 创建的 **Hylo** bot 已经有一个每天 09:00 的 `scheduled` 触发器来更新记忆。想改时间或多扫几个目录，在 **Settings → Bots** 里编辑这个触发器，在 prompt 后面加一句：

```
请根据我最近的笔记更新个人记忆。另外请扫描 /journal/ 目录。
```

之后每天定时自动更新，你什么都不用管。

#### 💬 方式二：直接在 Chat 里让 agent 跑

对着任意 agent 说一句话就能触发：

```
请更新我的 memory。我是 XXX，目前在做 YYY 项目，…（简单自我介绍）
```

Agent 会自己调用 `hylo-memory` skill 完成提取。首次运行扫最近 90 天，之后每次增量只扫最近 2 天，很快。

## Skills

Hylo 内置了一套 skill，agent 执行任务时会自动调用。内置 skill 打包在程序里，每次启动时写入 `~/.hylo/skills/`，和程序版本保持一致。你也可以从 GitHub 安装外部 skill，或者把自己写的 skill 扔进 `~/.hylo/skills/` 来扩展。

每个 skill 就是一个带 `SKILL.md` 的目录：

```
~/.hylo/skills/
└── your-skill/
    └── SKILL.md
```

#### 用 CLI 管理 skill

skill 先从 GitHub **添加（add）**到 `~/.hylo/skills/`，再为某个 bot **启用（enable）**，这个 bot 的 agent 才能用到。每个 bot 在 vault 里有自己的工作目录（主 bot Hylo 在 vault 根目录），skill 启用在 bot 的目录里。内置 skill 对主 bot 始终开启。

**查看 Hylo 推荐的 skill**

```sh
hylo skills catalog --table
hylo skills catalog --category news
```

每一条都列出了添加它所需的仓库和名字。

**从 GitHub 仓库添加 skill**

```sh
hylo skills add okou-ai/okou-skills --skill hackernews
hylo skills add https://github.com/hardhackerlabs/podwise-cli --skill podwise
```

按 `--skill` 在仓库的常见位置里查找 skill，保存到 `~/.hylo/skills/<name>/`；不加 `--skill` 时用仓库名。同名 skill 已存在时，加 `--force` 才会替换。

**为 bot 启用或停用 skill**

```sh
hylo bot list
hylo skills enable hackernews --bot-id <bot-id>
hylo skills disable hackernews --bot-id <bot-id>
```

在 app 里，可以在 **设置 → Bots** 中为每个 bot 选择 skill。

**查看已添加的 skill**

```sh
hylo skills list --table
```

**删除 skill**

```sh
hylo skills remove podwise
```

删除 skill 的文件并停用，无法撤销。

以上操作也都可以在 **Settings → Skills** 里完成。

每个内置 skill 的具体功能和提示词示例，见 [Skills 使用指南](./docs/skills_zh.md)。

## 自定义 AI 行为

Hylo 的每一层 AI 输出都可以自定义：

#### 1. 全局 Agent System Prompt

**设置 → Server → Config → Agent** — 填写 `agent.system_prompt` 替换内置的全局 system prompt，该 prompt 会拼接在每次 agent 运行的最前面。留空时 Hylo 使用内置默认值，该默认值会告知 agent vault 的目录结构、wiki-link 语法以及个人记忆文件的位置。

#### 2. 每个 Bot 的 System Prompt 与 Trigger Prompt

在**设置 → Bots** 中，每个 Bot 有两个自定义点：

- **System Prompt**：Bot 专属指令，附加在全局 system prompt 之后（用 `---` 分隔）。
- **Trigger Prompt 模板**：Trigger 触发时发送给 agent 的用户消息，支持变量：vault 事件用 `{Path}`、`{Name}`、`{Content}`；定时触发用 `{Now}`、`{Date}`、`{Time}`。

#### 3. 重写 LLM-Wiki 编译 Skill

LLM Wiki 编译行为由 `~/.hylo/skills/hylo-compile-note/SKILL.md` 定义。编辑此文件即可改变原始笔记被编译成结构化知识、并链接进 LLM Wiki 网络的方式。注意：Hylo 每次启动都会用内置版本覆盖它，改动需要自己保存好。

#### 4. 重写记忆抽取 Skill

个人记忆抽取行为由 `~/.hylo/skills/hylo-memory/SKILL.md` 定义。编辑此文件即可改变抽取内容与记忆文件的组织结构。和编译 skill 一样，每次启动都会被内置版本覆盖。

#### 5. 安装或编写自定义 Skill

将任意 skill 目录放入 `~/.hylo/skills/`，然后在**设置 → Skills** 中启用即可。Agent 通过目录名来引用 skill。
