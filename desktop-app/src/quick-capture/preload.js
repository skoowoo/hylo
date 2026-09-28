const { contextBridge, ipcRenderer } = require("electron");

contextBridge.exposeInMainWorld("quickCapture", {
  onInit: (cb) => ipcRenderer.on("quick-capture:init", (_e, data) => cb(data)),
  submit: (message) => ipcRenderer.send("quick-capture:submit", message),
  save: () => ipcRenderer.send("quick-capture:save"),
  cancel: () => ipcRenderer.send("quick-capture:cancel"),
});
