import { useEffect, useMemo, useState } from "react";
import { Bash } from "../components/Bash";
import { Button } from "../components/ui/button";
import { Input } from "../components/ui/input";
import { Label } from "../components/ui/label";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from "../components/ui/dialog";
import { Badge } from "../components/ui/badge";
import { api } from "../lib/api";
import { cn } from "../lib/utils";
import { toast } from "sonner";

type Distro = "debian" | "fedora" | "arch" | "macos";
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
  macos: [
    "brew install gumbo-parser openssl@3 curl expat meson pkg-config ossp-uuid",
    "brew install --cask macfuse   # grants the FUSE extension; reboot after first install",
    'export PKG_CONFIG_PATH="$(brew --prefix openssl@3)/lib/pkgconfig:$(brew --prefix)/lib/pkgconfig:$PKG_CONFIG_PATH"',
    "git clone --depth 1 https://github.com/fangfufu/httpdirfs /tmp/httpdirfs",
    "cd /tmp/httpdirfs && meson setup builddir && cd builddir && meson compile && sudo meson install",
  ].join("\n"),
};

function mountPath(target: Target): string {
  return target === "agentic" ? "$HOME/.agents/skills" : "$HOME/.claude/skills";
}

function buildScript(target: Target, distro: Distro, mode: Mode, token: string): string {
  const tokenComment = token
    ? "auto-created; revoke from the Tokens page"
    : "paste a token from the Tokens page (leave blank if mount-auth is none)";
  const header = `#!/usr/bin/env bash
set -euo pipefail

URL="${ORIGIN}/fs/"
TOKEN="${token}"            # ${tokenComment}
MOUNT="${mountPath(target)}"

# --- install httpdirfs ---
${INSTALL[distro]}

mkdir -p "$MOUNT"
# Detected after install so a freshly built binary is found (e.g. /usr/local/bin on Fedora/macOS).
BIN="$(command -v httpdirfs || echo /usr/bin/httpdirfs)"
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
  const [token, setToken] = useState("");
  const [prefix, setPrefix] = useState("");
  const [creating, setCreating] = useState(false);

  // Prefer the OS the browser reports.
  useEffect(() => {
    const ua = (typeof navigator !== "undefined" && navigator.userAgent) || "";
    const platform = (typeof navigator !== "undefined" && navigator.platform) || "";
    if (/Mac/i.test(platform) || /Macintosh/i.test(ua)) setDistro("macos");
    else if (/Arch|Linux/i.test(ua) && /Arch/i.test(ua)) setDistro("arch");
    else if (/Linux/i.test(ua)) setDistro("debian");
  }, []);

  const script = useMemo(() => buildScript(target, distro, mode, token), [target, distro, mode, token]);
  const tokenPlaceholder = token || "<TOKEN>";

  return (
    <div className="mx-auto grid max-w-5xl gap-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Mount</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Skills are exposed as a read-only filesystem at <span className="font-mono">{ORIGIN}/fs/</span>. Mount it
          with <span className="font-mono">httpdirfs</span> wherever your agent reads skills, and{" "}
          <strong>without</strong> <span className="font-mono">--cache</span> so content is never written to disk.
        </p>
      </div>

      <section className="flex flex-col gap-3 rounded-lg border bg-card p-4 sm:flex-row sm:items-center sm:justify-between">
        <div className="grid gap-1">
          <h2 className="font-medium">Token</h2>
          <p className="text-sm text-muted-foreground">
            {token ? (
              <>
                Using <span className="font-mono">{prefix}…</span> — revoke it on the Tokens page.
              </>
            ) : (
              <>
                Create a token to auto-fill the commands below, or get one from the Tokens page.
              </>
            )}
          </p>
        </div>
        <Button onClick={() => setCreating(true)} variant={token ? "outline" : "default"}>
          Create new
        </Button>
      </section>

      <section className="grid gap-3">
        <h2 className="text-lg font-medium">Quick mount</h2>
        <p className="text-sm text-muted-foreground">If <span className="font-mono">httpdirfs</span> is already installed.</p>
        <div className="grid gap-3 sm:grid-cols-2">
          <div className="grid gap-1">
            <p className="text-sm font-medium">Agentic</p>
            <Bash code={`httpdirfs -f -u skills -p ${tokenPlaceholder} ${ORIGIN}/fs/ ~/.agents/skills`} />
          </div>
          <div className="grid gap-1">
            <p className="text-sm font-medium">Claude</p>
            <Bash code={`httpdirfs -f -u skills -p ${tokenPlaceholder} ${ORIGIN}/fs/ ~/.claude/skills`} />
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
              { value: "macos", label: "macOS" },
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
        <Bash code={script} />
        <p className="text-xs text-muted-foreground">
          Mounts to <Badge variant="muted" className="font-mono">{mountPath(target)}</Badge>. Review the script before
          running. {mode === "run" ? "Just-run mounts stop when the terminal closes." : "systemd mounts auto-start at login."}
        </p>
      </section>

      <CreateTokenDialog
        open={creating}
        onOpenChange={setCreating}
        onCreated={(created) => {
          setToken(created.token);
          setPrefix(created.prefix);
          setCreating(false);
          toast.success("Token created and added to the commands.");
        }}
      />
    </div>
  );
}

function CreateTokenDialog({
  open,
  onOpenChange,
  onCreated,
}: {
  open: boolean;
  onOpenChange: (v: boolean) => void;
  onCreated: (created: { token: string; prefix: string }) => void;
}) {
  const [label, setLabel] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submit() {
    setBusy(true);
    setError(null);
    try {
      const created = await api.createToken(label.trim());
      setLabel("");
      onCreated({ token: created.token, prefix: created.prefix });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create token.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        setLabel("");
        setError(null);
        onOpenChange(v);
      }}
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Create mount token</DialogTitle>
          <DialogDescription>The token will be filled into the commands on this page.</DialogDescription>
        </DialogHeader>
        <div className="grid gap-2">
          <Label htmlFor="setup-token-label">Label</Label>
          <Input
            id="setup-token-label"
            value={label}
            onChange={(e) => setLabel(e.target.value)}
            placeholder="laptop"
            autoCapitalize="none"
            autoCorrect="off"
            spellCheck={false}
            onKeyDown={(e) => {
              if (e.key === "Enter" && !busy) submit();
            }}
          />
          {error && <p className="text-sm text-destructive">{error}</p>}
        </div>
        <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={busy}>
            Cancel
          </Button>
          <Button onClick={submit} disabled={busy}>
            {busy ? "Creating…" : "Create & fill"}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
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
      <div className="inline-flex flex-wrap rounded-md border p-0.5">
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
