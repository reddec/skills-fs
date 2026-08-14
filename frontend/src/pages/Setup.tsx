import { useMemo, useState } from "react";
import { CodeBlock } from "../components/CodeBlock";
import { cn } from "../lib/utils";

type Distro = "debian" | "fedora" | "arch";
type Mode = "run" | "systemd";
type Target = "agentic" | "claude";

const ORIGIN = typeof window !== "undefined" ? window.location.origin : "https://skills-fs.example";

const INSTALL: Record<Distro, string> = {
  debian: "sudo apt-get update\nsudo apt-get install -y httpdirfs  # Debian 13+ / recent Ubuntu",
  fedora: [
    "# httpdirfs is not packaged for Fedora; build it from source.",
    "sudo dnf install -y git meson clang-format gcc \\",
    "    gumbo-parser-devel openssl-devel libcurl-devel expat-devel \\",
    "    libuuid-devel fuse3-devel pkgconf-pkg-config",
    "git clone --depth 1 https://github.com/fangfufu/httpdirfs /tmp/httpdirfs",
    "cd /tmp/httpdirfs && meson setup builddir && cd builddir && meson compile && sudo meson install",
  ].join("\n"),
  arch: "paru -S --noconfirm httpdirfs || yay -S --noconfirm httpdirfs  # from the AUR",
};

function mountPath(target: Target): string {
  return target === "agentic" ? "$HOME/.agents/skills" : "$HOME/.claude/skills";
}

function buildScript(target: Target, distro: Distro, mode: Mode): string {
  const header = `#!/usr/bin/env bash
set -euo pipefail

URL="${ORIGIN}/fs/"
TOKEN=""            # paste a token from the Tokens page (leave blank if mount-auth is none)
MOUNT="${mountPath(target)}"
BIN="$(command -v httpdirfs || echo /usr/bin/httpdirfs)"

# --- install httpdirfs ---
${INSTALL[distro]}

mkdir -p "$MOUNT"
`;

  if (mode === "run") {
    return (
      header +
      `
# Foreground mount (Ctrl+C to stop). Mount WITHOUT --cache so skills stay in RAM.
exec "$BIN" -f -u skills -p "$TOKEN" "$URL" "$MOUNT"
`
    );
  }

  return (
    header +
    `
# --- systemd user service: auto-mounts at login ---
mkdir -p "$HOME/.config/systemd/user"
cat > "$HOME/.config/systemd/user/skills-fs.service" <<UNIT
[Unit]
Description=Skills-FS mount
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=\${BIN} -f -u skills -p \${TOKEN} \${URL} \${MOUNT}
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
UNIT

systemctl --user daemon-reload
systemctl --user enable --now skills-fs.service
echo "Enabled. Check status with: systemctl --user status skills-fs.service"
`
  );
}

export function SetupPage() {
  const [target, setTarget] = useState<Target>("agentic");
  const [distro, setDistro] = useState<Distro>("debian");
  const [mode, setMode] = useState<Mode>("run");

  const script = useMemo(() => buildScript(target, distro, mode), [target, distro, mode]);

  return (
    <div className="grid gap-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Mount</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Skills are exposed as a read-only filesystem at <span className="font-mono">{ORIGIN}/fs/</span>. Mount it
          with <span className="font-mono">httpdirfs</span> wherever your agent reads skills. Mount{" "}
          <strong>without</strong> <span className="font-mono">--cache</span> so content is never written to disk.
        </p>
      </div>

      <section className="grid gap-3">
        <h2 className="text-lg font-medium">Quick mount</h2>
        <p className="text-sm text-muted-foreground">
          If <span className="font-mono">httpdirfs</span> is already installed. Replace{" "}
          <span className="font-mono">{"<TOKEN>"}</span> with a token from the Tokens page.
        </p>
        <div className="grid gap-3 sm:grid-cols-2">
          <div className="grid gap-1">
            <p className="text-sm font-medium">Agentic</p>
            <CodeBlock code={`httpdirfs -f -u skills -p <TOKEN> ${ORIGIN}/fs/ ~/.agents/skills`} />
          </div>
          <div className="grid gap-1">
            <p className="text-sm font-medium">Claude</p>
            <CodeBlock code={`httpdirfs -f -u skills -p <TOKEN> ${ORIGIN}/fs/ ~/.claude/skills`} />
          </div>
        </div>
      </section>

      <section className="grid gap-3">
        <h2 className="text-lg font-medium">Install &amp; mount script</h2>
        <div className="flex flex-wrap items-center gap-2">
          <Segmented
            label="Target"
            value={target}
            options={[
              { value: "agentic", label: "Agentic" },
              { value: "claude", label: "Claude" },
            ]}
            onChange={(v) => setTarget(v as Target)}
          />
          <Segmented
            label="Distro"
            value={distro}
            options={[
              { value: "debian", label: "Debian/Ubuntu" },
              { value: "fedora", label: "Fedora" },
              { value: "arch", label: "Arch" },
            ]}
            onChange={(v) => setDistro(v as Distro)}
          />
          <Segmented
            label="Mode"
            value={mode}
            options={[
              { value: "run", label: "Just run" },
              { value: "systemd", label: "systemd" },
            ]}
            onChange={(v) => setMode(v as Mode)}
          />
        </div>
        <CodeBlock code={script} className="min-h-0" />
        <p className="text-xs text-muted-foreground">
          Mounts to <span className="font-mono">{mountPath(target)}</span>. Review the script before running.
        </p>
      </section>
    </div>
  );
}

function Segmented<T extends string>({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: T;
  options: { value: T; label: string }[];
  onChange: (v: T) => void;
}) {
  return (
    <div className="flex items-center gap-1">
      <span className="mr-1 text-xs uppercase tracking-wide text-muted-foreground">{label}</span>
      <div className="inline-flex rounded-md border p-0.5">
        {options.map((opt) => (
          <button
            key={opt.value}
            type="button"
            onClick={() => onChange(opt.value)}
            className={cn(
              "rounded px-2.5 py-1 text-xs font-medium transition-colors",
              opt.value === value ? "bg-primary text-primary-foreground" : "text-muted-foreground hover:text-foreground"
            )}
          >
            {opt.label}
          </button>
        ))}
      </div>
    </div>
  );
}
