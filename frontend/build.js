const fs = require("fs");
const path = require("path");

// copyStatic copies the cleaner page into the folder Wails embeds.
const src = path.join(__dirname, "src");
const dist = path.join(__dirname, "dist");
fs.mkdirSync(dist, { recursive: true });
for (const name of ["index.html", "app.js", "style.css"]) {
  fs.copyFileSync(path.join(src, name), path.join(dist, name));
}
fs.copyFileSync(path.join(__dirname, "..", "build", "appicon.png"), path.join(dist, "icon.png"));
