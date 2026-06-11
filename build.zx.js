const GOOS = "linux";
const GOARCH = "amd64";
const env = { ...process.env, GOOS, GOARCH };

usePwsh();

const targets = [
  { pkg: "./daemon", out: "lightscaled" },
  { pkg: "./cli", out: "lightscale" },
];

await $`mkdir -ea 0 bin`;

for (const { pkg, out } of targets) {
  echo(`Building ${pkg} -> bin/${out} (${GOOS}/${GOARCH})`);
  await $({ env })`go build -o bin/${out} ${pkg}`;
}