const { contextBridge, ipcRenderer } = require("electron");

contextBridge.exposeInMainWorld("hyloDesktop", {
  platform: process.platform,
  checkServer: (url) => ipcRenderer.invoke("check-server", url),
  startHyloServerDetached: (opts) => ipcRenderer.invoke("start-hylo-server-detached", opts),
  stopServer: () => ipcRenderer.invoke("stop-hylo-server"),
  getServerProcessStatus: () => ipcRenderer.invoke("get-server-process-status"),
  getShellDebugPaths: () => ipcRenderer.invoke("get-shell-debug-paths"),
  getServerUrl: () => ipcRenderer.invoke("get-server-url"),
  setServerUrl: (url) => ipcRenderer.invoke("set-server-url", url),
  restartServer: () => ipcRenderer.invoke("restart-hylo-server"),
  syncVaultDataAcrossSections: () => ipcRenderer.invoke("sync-vault-data-across-sections"),
  setViewBgColor: (color, theme) => ipcRenderer.send("set-view-bg-color", color, theme),
  setWindowButtonVisibility: (visible) => ipcRenderer.send("set-window-button-visibility", visible),
  pickFolder: (opts) => ipcRenderer.invoke("pick-folder", opts),
  inboxNotify: {
    getSettings: ()           => ipcRenderer.invoke("inbox-notify:get-settings"),
    setSettings: (settings)   => ipcRenderer.invoke("inbox-notify:set-settings", settings),
    previewSound: (sound)     => ipcRenderer.invoke("inbox-notify:preview-sound", sound),
  },
});
