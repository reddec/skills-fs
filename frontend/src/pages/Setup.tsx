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

type Distro = "debian" | "fedora" | "arch" | "macos" | "windows";
type Mode = "run" | "systemd" | "launchd";
type Target = "agentic" | "claude";

const ORIGIN = typeof window !== "undefined" ? window.location.origin : "https://skills-fs.example";

const DISTROS: { value: Distro; label: string }[] = [
  { value: "debian", label: "Debian/Ubuntu" },
  { value: "fedora", label: "Fedora" },
  { value: "arch", label: "Arch" },
  { value: "macos", label: "macOS" },
  { value: "windows", label: "Windows" },
];

const MODES: Record<Distro, { value: Mode; label: string }[]> = {
  debian: [{ value: "run", label: "Just run" }, { value: "systemd", label: "systemd" }],
  fedora: [{ value: "run", label: "Just run" }, { value: "systemd", label: "systemd" }],
  arch: [{ value: "run", label: "Just run" }, { value: "systemd", label: "systemd" }],
  macos: [{ value: "run", label: "Just run" }, { value: "launchd", label: "launchd" }],
  windows: [{ value: "run", label: "Just run" }],
};

const UNIX_INSTALL: Record<Exclude<Distro, "windows">, string> = {
  debian: "sudo apt-get update\nsudo apt-get install -y rclone fuse3",
  fedora: "sudo dnf install -y rclone fuse",
  arch: "sudo pacman -S --noconfirm rclone fuse3",
  macos: "brew install rclone\nbrew install --cask macfuse  # grant the FUSE extension; reboot after first install",
};

function unixMount(target: Target): string {
  return target === "agentic" ? "$HOME/.agents/skills" : "$HOME/.claude/skills";
}

function windowsMount(target: Target): string {
  return target === "agentic" ? `%USERPROFILE%\\.agents\\skills` : `%USERPROFILE%\\.claude\\skills`;
}

// url embeds the token (if any) as basic-auth credentials the server already accepts.
function buildURL(token: string): string {
  const base = ORIGIN + "/fs/";
  return token ? ORIGIN.replace("://", `://skills:${token}@`) + "/fs/" : base;
}

function buildUnix(target: Target, distro: Exclude<Distro, "windows">, mode: Mode, url: string): string {
  const mount = unixMount(target);
  const header = `#!/usr/bin/env bash
set -euo pipefail

URL="${url}"
MOUNT="${mount}"

# --- install rclone + FUSE ---
${UNIX_INSTALL[distro]}

mkdir -p "$MOUNT"
`;

  if (mode === "run") {
    return (
      header +
      `
# Foreground read-only mount (Ctrl+C to stop). No disk cache: skills stay in RAM.
exec rclone mount :http: "$MOUNT" --http-url "$URL" --vfs-cache-mode off --read-only
`
    );
  }

  return (
    header +
    `
# --- systemd user service: auto-mounts at login ---
RCLONE_BIN="$(command -v rclone)"
mkdir -p "$HOME/.config/systemd/user"
cat > "$HOME/.config/systemd/user/skills-fs.service" <<UNIT
[Unit]
Description=Skills-FS mount
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
Environment=RCLONE_HTTP_URL=\${URL}
ExecStart=\${RCLONE_BIN} mount :http: \${MOUNT} --vfs-cache-mode off --read-only
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
UNIT

systemctl --user daemon-reload
systemctl --user enable --now skills-fs.service
echo "Enabled. Status: systemctl --user status skills-fs.service"
`
  );
}

function buildLaunchd(target: Target, url: string): string {
  const dir = target === "agentic" ? ".agents/skills" : ".claude/skills";
  return `#!/usr/bin/env bash
set -euo pipefail

URL="${url}"

brew install rclone
brew install --cask macfuse  # grant the FUSE extension; reboot after first install

mkdir -p "$HOME/${dir}"
RCLONE_BIN="$(command -v rclone)"
PLIST="$HOME/Library/LaunchAgents/com.reddec.skills-fs.plist"

cat > "$PLIST" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key><string>com.reddec.skills-fs</string>
    <key>ProgramArguments</key>
    <array>
        <string>\${RCLONE_BIN}</string>
        <string>mount</string>
        <string>:http:</string>
        <string>\${HOME}/${dir}</string>
        <string>--vfs-cache-mode</string><string>off</string>
        <string>--read-only</string>
    </array>
    <key>EnvironmentVariables</key>
    <dict>
        <key>RCLONE_HTTP_URL</key><string>\${URL}</string>
    </dict>
    <key>RunAtLoad</key><true/>
    <key>KeepAlive</key><true/>
</dict>
</plist>
PLIST

launchctl load "$PLIST"
echo "Loaded. Unload with: launchctl unload \\"$PLIST\\""
`;
}

function buildWindows(target: Target, url: string): string {
  return `:: Install rclone + WinFsp (run in an elevated terminal)
winget install Rclone.Rclone
winget install WinFsp.WinFsp

:: Read-only mount with no disk cache. Ctrl+C to stop.
rclone mount :http: "${windowsMount(target)}" --http-url "${url}" --vfs-cache-mode off --read-only

:: To auto-start, schedule the command above in Task Scheduler.
`;
}

function buildScript(target: Target, distro: Distro, mode: Mode, token: string): string {
  const url = buildURL(token);
  if (distro === "windows") return buildWindows(target, url);
  if (mode === "launchd") return buildLaunchd(target, url);
  return buildUnix(target, distro, mode, url);
}

export function SetupPage() {
  const [target, setTarget] = useState<Target>("agentic");
  const [distro, setDistro] = useState<Distro>("debian");
  const [mode, setMode] = useState<Mode>("run");
  const [token, setToken] = useState("");
  const [prefix, setPrefix] = useState("");
  const [creating, setCreating] = useState(false);

  useEffect(() => {
    const ua = (typeof navigator !== "undefined" && navigator.userAgent) || "";
    const platform = (typeof navigator !== "undefined" && navigator.platform) || "";
    if (/Win/i.test(platform) || /Windows/i.test(ua)) setDistro("windows");
    else if (/Mac/i.test(platform) || /Macintosh/i.test(ua)) setDistro("macos");
    else if (/Arch/i.test(ua)) setDistro("arch");
    else if (/Fedora/i.test(ua)) setDistro("fedora");
    else if (/Linux/i.test(ua)) setDistro("debian");
  }, []);

  // Keep the mode valid for the chosen distro.
  useEffect(() => {
    if (!MODES[distro].some((m) => m.value === mode)) {
      setMode(MODES[distro][0].value);
    }
  }, [distro, mode]);

  const script = useMemo(() => buildScript(target, distro, mode, token), [target, distro, mode, token]);
  const urlPreview = buildURL(token || "<TOKEN>");
  const mountLabel = distro === "windows" ? windowsMount(target) : unixMount(target).replace("$HOME", "~");

  return (
    <div className="mx-auto grid max-w-5xl gap-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Mount</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Skills are exposed as a read-only filesystem at <span className="font-mono">{ORIGIN}/fs/</span>. Mount them
          with <a className="underline underline-offset-4" href="https://rclone.org" target="_blank" rel="noreferrer">rclone</a>{" "}
          wherever your agent reads skills. <strong>Without</strong> <span className="font-mono">--vfs-cache-mode</span>{" "}
          caching, content is never written to disk.
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
              <>Create a token to auto-fill the commands below, or get one from the Tokens page.</>
            )}
          </p>
        </div>
        <Button onClick={() => setCreating(true)} variant={token ? "outline" : "default"}>
          Create new
        </Button>
      </section>

      <section className="grid gap-3">
        <h2 className="text-lg font-medium">Quick mount (macOS / Linux)</h2>
        <p className="text-sm text-muted-foreground">If <span className="font-mono">rclone</span> is already installed.</p>
        <div className="grid gap-3 sm:grid-cols-2">
          <div className="grid gap-1">
            <p className="text-sm font-medium">Agentic</p>
            <Bash code={`rclone mount :http: ~/.agents/skills --http-url '${urlPreview}' --vfs-cache-mode off --read-only`} />
          </div>
          <div className="grid gap-1">
            <p className="text-sm font-medium">Claude</p>
            <Bash code={`rclone mount :http: ~/.claude/skills --http-url '${urlPreview}' --vfs-cache-mode off --read-only`} />
          </div>
        </div>
      </section>

      <section className="grid gap-3">
        <h2 className="text-lg font-medium">Install &amp; mount script</h2>
        <div className="flex flex-wrap items-center gap-2">
          <Segmented label="Target" value={target} options={[{ value: "agentic", label: "Agentic" }, { value: "claude", label: "Claude" }]} onChange={(v) => setTarget(v as Target)} />
          <Segmented label="System" value={distro} options={DISTROS} onChange={(v) => setDistro(v as Distro)} />
          <Segmented label="Mode" value={mode} options={MODES[distro]} onChange={(v) => setMode(v as Mode)} />
        </div>
        <Bash code={script} />
        <p className="text-xs text-muted-foreground">
          Mounts to <Badge variant="muted" className="font-mono">{mountLabel}</Badge>. Review the script before running.
          {mode === "run" ? " Just-run mounts stop when the terminal closes." : " Auto-starts at login."}
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
