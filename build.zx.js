const GOOS = "linux";
const GOARCH = "amd64";
const env = { ...process.env, GOOS, GOARCH };

usePwsh();
$.log = (entry) => { entry.kind === 'cmd' && console.log(entry.cmd); }

const targets = [
  { pkg: "./daemon", out: "lightscaled" },
  { pkg: "./cli", out: "lightscale" },
];

await $`mkdir -ea 0 bin`;

echo("Building web UI -> web/dist");
await $({ cwd: "web/frontend" })`pnpm install --frozen-lockfile`;
await $({ cwd: "web/frontend" })`pnpm run build`;

for (const { pkg, out } of targets) {
  echo(`Building ${pkg} -> bin/${out} (${GOOS}/${GOARCH})`);
  await $({ env })`go build -o bin/${out} ${pkg}`;
}