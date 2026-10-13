// Copies the app's vendored Alpine, fonts and agent logos into the shell
// page, which the Hylo server does not serve. Design tokens come from a Go test instead
// (TestDesktopShellTokens), since they live in Go source.
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const here = path.dirname(fileURLToPath(import.meta.url));
const vendor = path.resolve(here, "../../internal/server/static/vendor");
const agents = path.resolve(here, "../../internal/server/static/agents");
const ui = path.resolve(here, "../ui");
const families = ["Inter", "JetBrains Mono"];

fs.mkdirSync(path.join(ui, "vendor"), { recursive: true });
for (const f of ["alpine.min.js", "alpine-focus.min.js"]) {
  fs.copyFileSync(path.join(vendor, f), path.join(ui, "vendor", f));
}

const css = fs.readFileSync(path.join(vendor, "fonts.css"), "utf8");
const blocks = css.split(/(?=\/\*[^*]*\*\/\s*@font-face)/);
const kept = blocks.filter((b) => families.some((f) => b.includes(`font-family: '${f}'`)));
const fontsDir = path.join(ui, "fonts");
fs.rmSync(fontsDir, { recursive: true, force: true });
fs.mkdirSync(fontsDir);
const out = kept
  .map((b) =>
    b.replace(/url\(\/static\/vendor\/fonts\/([^)]+)\)/g, (_, file) => {
      fs.copyFileSync(path.join(vendor, "fonts", file), path.join(fontsDir, file));
      return `url(fonts/${file})`;
    })
  )
  .join("");
fs.writeFileSync(path.join(ui, "fonts.css"), out);

fs.rmSync(path.join(ui, "agents"), { recursive: true, force: true });
fs.cpSync(agents, path.join(ui, "agents"), { recursive: true });
