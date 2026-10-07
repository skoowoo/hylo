# Hylo

桌面壳。窗口加载本机 `hylo` 进程吐出的页面，并通过 `window.hyloDesktop` 调用壳能力。Go 服务不在这里。

应用名 **Hylo**，bundle id `dev.hardhacker.hylo`。应用数据目录由 Tauri 的 `app_data_dir()` 决定（目录名取 bundle id）：macOS 上是 `~/Library/Application Support/dev.hardhacker.hylo`，里面有 `config.json`、server 的 pid 文件和日志。

## 开发

需要本机默认的 stable Rust 和 Node。

```bash
cd desktop-tauri
npm install
npm run dev
```

开发时启动页由 `scripts/dev-server.mjs` 提供。连上 `http://127.0.0.1:54321` 之后，窗口切到 Go 服务的 `/home`。本机没有在跑的 `hylo` 时，壳会按 PATH（含 `~/.local/bin`）拉起 `hylo start server`。

开发构建不带 CLI 安装包。发布用仓库根目录的 `make dist-dmg`，它会把当前平台的 `hylo` 归档打进 `Hylo.app/Contents/Resources/cli/hylo.tar.gz`，并把 DMG 复制成 `dist/Hylo-<version>.dmg`。

签名和原来的 Electron 包一样：从钥匙串里取 `Developer ID Application`（没有就退回 ad-hoc），hardened runtime，entitlements 用 electron-builder 那份默认项，签 `.app` 和 DMG。没有公证凭据，不会走公证。

## 这一版有什么

- 启动、健康检查、拉起 / 停止 / 重启 `hylo`
- 单实例
- 文件夹选择、收件箱系统通知、提示音
- macOS 隐藏标题栏、侧栏毛玻璃、应用内主题同步
- 拖拽区域由壳注入，不改 Go 模板
- 编辑菜单的撤销 / 重做走页面里的 `__hyloUndo` / `__hyloRedo`
- macOS 快捷截图（⌥⇧C）：系统选区、浮层面板、保存到图片库
