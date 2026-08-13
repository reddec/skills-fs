import { useState, type FormEvent } from "react";
import { type SkillWrite } from "../lib/api";
import { Button } from "./ui/button";
import { Input } from "./ui/input";
import { Textarea } from "./ui/textarea";
import { Label } from "./ui/label";

const NAME_RE = /^[a-z0-9]+(-[a-z0-9]+)*$/;

interface KV {
  key: string;
  value: string;
}

interface SkillFormProps {
  initial?: Partial<SkillWrite>;
  submitLabel: string;
  // onSubmit receives validated values and should throw on server error (message shown inline).
  onSubmit: (data: SkillWrite) => Promise<void>;
  onCancel?: () => void;
}

export function SkillForm({ initial, submitLabel, onSubmit, onCancel }: SkillFormProps) {
  const [name, setName] = useState(initial?.name ?? "");
  const [description, setDescription] = useState(initial?.description ?? "");
  const [body, setBody] = useState(initial?.body ?? "");
  const [license, setLicense] = useState(initial?.license ?? "");
  const [compatibility, setCompatibility] = useState(initial?.compatibility ?? "");
  const [allowedTools, setAllowedTools] = useState(initial?.allowedTools ?? "");
  const [kvs, setKvs] = useState<KV[]>(() =>
    Object.entries(initial?.metadata ?? {}).map(([key, value]) => ({ key, value }))
  );
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  function setKvField(index: number, field: keyof KV, value: string) {
    setKvs((rows) => rows.map((row, i) => (i === index ? { ...row, [field]: value } : row)));
  }

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    const trimmedName = name.trim();
    if (!NAME_RE.test(trimmedName) || trimmedName.length > 64) {
      setError("Name must be 1-64 lowercase letters, digits, or single hyphens.");
      return;
    }
    if (description.trim() === "" || description.length > 1024) {
      setError("Description must be 1-1024 characters.");
      return;
    }
    if (compatibility.length > 500) {
      setError("Compatibility must be at most 500 characters.");
      return;
    }
    const metadata: Record<string, string> = {};
    for (const { key, value } of kvs) {
      const k = key.trim();
      if (k) metadata[k] = value;
    }
    const data: SkillWrite = { name: trimmedName, description, body, license, compatibility, allowedTools, metadata };
    setBusy(true);
    setError(null);
    try {
      await onSubmit(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Something went wrong.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={handleSubmit} className="grid gap-4">
      <div className="grid gap-2">
        <Label htmlFor="name">Name</Label>
        <Input
          id="name"
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="code-review"
          autoCapitalize="none"
          autoCorrect="off"
          spellCheck={false}
        />
        <p className="text-xs text-muted-foreground">
          Lowercase letters, digits, single hyphens. Used as the directory and frontmatter name.
        </p>
      </div>

      <div className="grid gap-2">
        <Label htmlFor="description">Description</Label>
        <Input
          id="description"
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          placeholder="What this skill does and when to use it."
        />
      </div>

      <div className="grid gap-2">
        <Label htmlFor="body">SKILL.md body</Label>
        <Textarea id="body" value={body} onChange={(e) => setBody(e.target.value)} rows={10} placeholder="# Instructions&#10;Step-by-step guidance for the agent." />
      </div>

      <div className="grid gap-2 sm:grid-cols-2">
        <div className="grid gap-2">
          <Label htmlFor="license">License (optional)</Label>
          <Input id="license" value={license} onChange={(e) => setLicense(e.target.value)} placeholder="MIT" />
        </div>
        <div className="grid gap-2">
          <Label htmlFor="allowedTools">Allowed tools (optional)</Label>
          <Input id="allowedTools" value={allowedTools} onChange={(e) => setAllowedTools(e.target.value)} placeholder="Bash(git:*) Read" />
        </div>
      </div>

      <div className="grid gap-2">
        <Label htmlFor="compatibility">Compatibility (optional)</Label>
        <Input
          id="compatibility"
          value={compatibility}
          onChange={(e) => setCompatibility(e.target.value)}
          placeholder="Requires git, docker, and network access."
        />
      </div>

      <div className="grid gap-2">
        <div className="flex items-center justify-between">
          <Label>Metadata (optional)</Label>
          <Button type="button" variant="outline" size="sm" onClick={() => setKvs((rows) => [...rows, { key: "", value: "" }])}>
            Add field
          </Button>
        </div>
        <div className="grid gap-2">
          {kvs.map((row, index) => (
            <div key={index} className="grid grid-cols-[1fr_1fr_auto] gap-2">
              <Input
                value={row.key}
                onChange={(e) => setKvField(index, "key", e.target.value)}
                placeholder="key"
                autoCapitalize="none"
                autoCorrect="off"
                spellCheck={false}
              />
              <Input value={row.value} onChange={(e) => setKvField(index, "value", e.target.value)} placeholder="value" />
              <Button
                type="button"
                variant="ghost"
                size="icon"
                aria-label="Remove field"
                onClick={() => setKvs((rows) => rows.filter((_, i) => i !== index))}
              >
                ×
              </Button>
            </div>
          ))}
        </div>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
        {onCancel && (
          <Button type="button" variant="outline" onClick={onCancel} disabled={busy}>
            Cancel
          </Button>
        )}
        <Button type="submit" disabled={busy}>
          {busy ? "Saving…" : submitLabel}
        </Button>
      </div>
    </form>
  );
}

