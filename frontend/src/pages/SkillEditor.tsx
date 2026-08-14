import { useEffect, useMemo, useState, type ReactNode } from "react";
import { useNavigate, useParams } from "react-router-dom";
import CodeMirror from "@uiw/react-codemirror";
import { markdown, markdownLanguage } from "@codemirror/lang-markdown";
import { ArrowLeft, FileText, Settings2 } from "lucide-react";
import { api, type SkillWrite } from "../lib/api";
import { Button } from "../components/ui/button";
import { Input } from "../components/ui/input";
import { Label } from "../components/ui/label";
import { Skeleton } from "../components/ui/skeleton";
import { toast } from "sonner";
import { cn } from "../lib/utils";

const NAME_RE = /^[a-z0-9]+(-[a-z0-9]+)*$/;
const isDark = typeof window !== "undefined" && window.matchMedia?.("(prefers-color-scheme: dark)").matches;

interface KV {
  key: string;
  value: string;
}

type Page = "overview" | "content";

export function SkillEditorPage({ mode }: { mode: "create" | "edit" }) {
  const { id } = useParams();
  const navigate = useNavigate();
  const [page, setPage] = useState<Page>("overview");

  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [body, setBody] = useState("");
  const [license, setLicense] = useState("");
  const [compatibility, setCompatibility] = useState("");
  const [allowedTools, setAllowedTools] = useState("");
  const [kvs, setKvs] = useState<KV[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [loading, setLoading] = useState(mode === "edit");

  useEffect(() => {
    if (mode !== "edit" || !id) return;
    let alive = true;
    api
      .getSkill(Number(id))
      .then((skill) => {
        if (!alive) return;
        setName(skill.name);
        setDescription(skill.description);
        setBody(skill.body);
        setLicense(skill.license);
        setCompatibility(skill.compatibility);
        setAllowedTools(skill.allowedTools);
        setKvs(Object.entries(skill.metadata).map(([key, value]) => ({ key, value })));
        setLoading(false);
      })
      .catch((err) => {
        if (!alive) return;
        setError(err instanceof Error ? err.message : "Failed to load skill.");
        setLoading(false);
      });
    return () => {
      alive = false;
    };
  }, [mode, id]);

  const metadata = useMemo(() => {
    const m: Record<string, string> = {};
    for (const { key, value } of kvs) {
      const k = key.trim();
      if (k) m[k] = value;
    }
    return m;
  }, [kvs]);

  async function save() {
    const trimmedName = name.trim();
    if (!NAME_RE.test(trimmedName) || trimmedName.length > 64) {
      setError("Name must be 1-64 lowercase letters, digits, or single hyphens.");
      setPage("overview");
      return;
    }
    if (description.trim() === "" || description.length > 1024) {
      setError("Description must be 1-1024 characters.");
      setPage("overview");
      return;
    }
    if (compatibility.length > 500) {
      setError("Compatibility must be at most 500 characters.");
      setPage("overview");
      return;
    }
    const data: SkillWrite = { name: trimmedName, description, body, license, compatibility, allowedTools, metadata };
    setBusy(true);
    setError(null);
    try {
      const saved = mode === "edit" && id ? await api.updateSkill(Number(id), data) : await api.createSkill(data);
      toast.success("Skill saved.");
      navigate(`/skills/${saved.id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Something went wrong.");
    } finally {
      setBusy(false);
    }
  }

  if (loading) {
    return (
      <div className="grid gap-4">
        <Skeleton className="h-10 w-full" />
        <Skeleton className="h-[60vh] w-full" />
      </div>
    );
  }

  return (
    <div className="grid gap-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <Button variant="ghost" size="sm" onClick={() => navigate(-1)}>
          <ArrowLeft /> Back
        </Button>
        <div className="flex gap-2">
          <Button variant="outline" onClick={() => navigate(-1)} disabled={busy}>
            Cancel
          </Button>
          <Button onClick={save} disabled={busy}>
            {busy ? "Saving…" : "Save skill"}
          </Button>
        </div>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      <div className="flex flex-col gap-4 md:flex-row md:items-start">
        {/* Left menu on desktop, top tabs on mobile. */}
        <nav className="flex gap-1 md:w-48 md:flex-col">
          <MenuButton active={page === "overview"} onClick={() => setPage("overview")}>
            <Settings2 className="size-4" /> Overview
          </MenuButton>
          <MenuButton active={page === "content"} onClick={() => setPage("content")}>
            <FileText className="size-4" /> Content
          </MenuButton>
        </nav>

        <div className="min-w-0 flex-1">
          {page === "overview" ? (
            <div className="grid gap-4">
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
              <div className="grid gap-3 sm:grid-cols-3">
                <Field label="License" value={license} onChange={setLicense} placeholder="MIT" />
                <Field label="Allowed tools" value={allowedTools} onChange={setAllowedTools} placeholder="Bash(git:*) Read" />
                <Field label="Compatibility" value={compatibility} onChange={setCompatibility} placeholder="Requires git" />
              </div>
              <div className="grid gap-2">
                <div className="flex items-center justify-between">
                  <Label>Metadata</Label>
                  <Button type="button" variant="outline" size="sm" onClick={() => setKvs((rows) => [...rows, { key: "", value: "" }])}>
                    Add field
                  </Button>
                </div>
                <div className="grid gap-2">
                  {kvs.map((row, index) => (
                    <div key={index} className="grid grid-cols-[1fr_1fr_auto] gap-2">
                      <Input
                        value={row.key}
                        onChange={(e) => setKvs((rows) => rows.map((r, i) => (i === index ? { ...r, key: e.target.value } : r)))}
                        placeholder="key"
                        autoCapitalize="none"
                        autoCorrect="off"
                        spellCheck={false}
                      />
                      <Input
                        value={row.value}
                        onChange={(e) => setKvs((rows) => rows.map((r, i) => (i === index ? { ...r, value: e.target.value } : r)))}
                        placeholder="value"
                      />
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
            </div>
          ) : (
            <div className="h-[calc(100vh-11rem)] min-h-[50vh] overflow-hidden rounded-md border">
              <CodeMirror
                value={body}
                onChange={setBody}
                theme={isDark ? "dark" : "light"}
                extensions={[markdown({ base: markdownLanguage })]}
                height="100%"
                className="h-full text-sm"
                placeholder="# Instructions&#10;Step-by-step guidance for the agent."
              />
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

function MenuButton({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "flex items-center gap-2 rounded-md px-3 py-2 text-sm font-medium transition-colors",
        active ? "bg-accent text-accent-foreground" : "text-muted-foreground hover:bg-accent/60 hover:text-foreground"
      )}
    >
      {children}
    </button>
  );
}

function Field({
  label,
  value,
  onChange,
  placeholder,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
}) {
  return (
    <div className="grid gap-2">
      <Label>{label}</Label>
      <Input value={value} onChange={(e) => onChange(e.target.value)} placeholder={placeholder} />
    </div>
  );
}
