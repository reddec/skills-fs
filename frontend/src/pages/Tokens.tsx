import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { Copy, Check, KeyRound, Plus, Trash2 } from "lucide-react";
import { api, type Token, type TokenCreated } from "../lib/api";
import { shortDate } from "../lib/utils";
import { Button } from "../components/ui/button";
import { Input } from "../components/ui/input";
import { Label } from "../components/ui/label";
import { Card } from "../components/ui/card";
import { Skeleton } from "../components/ui/skeleton";
import { Badge } from "../components/ui/badge";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from "../components/ui/dialog";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "../components/ui/alert-dialog";
import { toast } from "sonner";

export function TokensPage() {
  const [tokens, setTokens] = useState<Token[]>([]);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const [creating, setCreating] = useState(false);
  const [issued, setIssued] = useState<TokenCreated | null>(null);

  async function refresh() {
    setState("loading");
    try {
      setTokens(await api.listTokens());
      setState("ready");
    } catch {
      setState("error");
    }
  }

  useEffect(() => {
    refresh();
  }, []);

  async function handleDelete(token: Token) {
    try {
      await api.deleteToken(token.id);
      setTokens((prev) => prev.filter((t) => t.id !== token.id));
      toast.success("Token revoked.");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to revoke token.");
    }
  }

  return (
    <div className="grid gap-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Mount tokens</h1>
          <p className="text-sm text-muted-foreground">
            Tokens authenticate <span className="font-mono">httpdirfs</span> mounts. See the{" "}
            <Link to="/setup" className="underline underline-offset-4">
              Mount
            </Link>{" "}
            page for setup instructions.
          </p>
        </div>
        <Button onClick={() => setCreating(true)}>
          <Plus /> New token
        </Button>
      </div>

      {state === "loading" && (
        <div className="grid gap-2">
          {Array.from({ length: 2 }).map((_, i) => (
            <Skeleton key={i} className="h-16 w-full" />
          ))}
        </div>
      )}

      {state === "error" && (
        <Card className="p-6 text-sm text-muted-foreground">
          Could not load tokens.{" "}
          <Button variant="link" className="h-auto p-0 align-baseline" onClick={refresh}>
            Retry
          </Button>
        </Card>
      )}

      {state === "ready" && tokens.length === 0 && (
        <Card className="p-8 text-center">
          <p className="text-lg font-medium">No tokens yet.</p>
          <p className="mt-1 text-sm text-muted-foreground">Issue a token to mount the skills filesystem.</p>
        </Card>
      )}

      {state === "ready" &&
        tokens.map((token) => (
          <Card key={token.id} className="p-4">
            <div className="flex items-center justify-between gap-3">
              <div className="grid gap-1">
                <div className="flex items-center gap-2">
                  <KeyRound className="h-4 w-4 text-muted-foreground" />
                  <span className="font-medium">{token.label || "Untitled"}</span>
                  <Badge variant="muted" className="font-mono">
                    {token.prefix}…
                  </Badge>
                </div>
                <p className="text-xs text-muted-foreground">
                  Last used {shortDate(token.lastUsedAt)} · Created {shortDate(token.createdAt)}
                </p>
              </div>
              <AlertDialog>
                <AlertDialogTrigger asChild>
                  <Button variant="outline" size="sm">
                    <Trash2 /> Revoke
                  </Button>
                </AlertDialogTrigger>
                <AlertDialogContent>
                  <AlertDialogHeader>
                    <AlertDialogTitle>Revoke token for {token.label || "Untitled"}?</AlertDialogTitle>
                    <AlertDialogDescription>
                      Mounts using this token (prefix <span className="font-mono">{token.prefix}…</span>) will
                      immediately stop working. This cannot be undone.
                    </AlertDialogDescription>
                  </AlertDialogHeader>
                  <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
                    <AlertDialogCancel>Cancel</AlertDialogCancel>
                    <AlertDialogAction onClick={() => handleDelete(token)}>Revoke token</AlertDialogAction>
                  </div>
                </AlertDialogContent>
              </AlertDialog>
            </div>
          </Card>
        ))}

      <CreateTokenDialog
        open={creating}
        onOpenChange={setCreating}
        onIssued={(created) => {
          setIssued(created);
          setCreating(false);
        }}
      />
      <RevealTokenDialog issued={issued} onClose={() => setIssued(null)} onDone={refresh} />
    </div>
  );
}

function CreateTokenDialog({
  open,
  onOpenChange,
  onIssued,
}: {
  open: boolean;
  onOpenChange: (v: boolean) => void;
  onIssued: (created: TokenCreated) => void;
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
      onIssued(created);
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
          <DialogTitle>New mount token</DialogTitle>
          <DialogDescription>Give it a label to identify where it is used.</DialogDescription>
        </DialogHeader>
        <div className="grid gap-2">
          <Label htmlFor="token-label">Label</Label>
          <Input
            id="token-label"
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
            {busy ? "Creating…" : "Create token"}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}

function RevealTokenDialog({
  issued,
  onClose,
  onDone,
}: {
  issued: TokenCreated | null;
  onClose: () => void;
  onDone: () => void;
}) {
  const open = issued !== null;
  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        if (!v) {
          onClose();
          onDone();
        }
      }}
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Token created</DialogTitle>
          <DialogDescription>
            Copy it now. For security it is stored only as a hash and cannot be shown again.
          </DialogDescription>
        </DialogHeader>
        {issued && (
          <div className="flex items-center gap-2 rounded-md border bg-muted/40 p-3">
            <code className="flex-1 break-all font-mono text-xs">{issued.token}</code>
            <CopyButton text={issued.token} />
          </div>
        )}
        <p className="text-sm text-muted-foreground">
          See the{" "}
          <Link to="/setup" className="underline underline-offset-4">
            Mount
          </Link>{" "}
          page for ready-to-copy install and mount commands.
        </p>
        <div className="flex justify-end">
          <Button
            onClick={() => {
              onClose();
              onDone();
            }}
          >
            Done
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);
  return (
    <Button
      variant="ghost"
      size="sm"
      className="shrink-0"
      onClick={async () => {
        try {
          await navigator.clipboard.writeText(text);
          setCopied(true);
          setTimeout(() => setCopied(false), 1500);
        } catch {
          toast.error("Copy failed.");
        }
      }}
    >
      {copied ? <Check /> : <Copy />}
      {copied ? "Copied" : "Copy"}
    </Button>
  );
}
