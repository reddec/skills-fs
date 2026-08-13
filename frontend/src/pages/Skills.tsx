import { useEffect, useMemo, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { Plus, Search } from "lucide-react";
import { api, type Skill, type SkillSummary } from "../lib/api";
import { shortDate } from "../lib/utils";
import { Button } from "../components/ui/button";
import { Input } from "../components/ui/input";
import { Card } from "../components/ui/card";
import { Skeleton } from "../components/ui/skeleton";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from "../components/ui/dialog";
import { SkillForm } from "../components/SkillForm";

export function SkillsPage() {
  const [params, setParams] = useSearchParams();
  const query = params.get("q") ?? "";
  const [skills, setSkills] = useState<SkillSummary[]>([]);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const [creating, setCreating] = useState(false);

  async function refresh() {
    setState("loading");
    try {
      setSkills(await api.listSkills());
      setState("ready");
    } catch {
      setState("error");
    }
  }

  useEffect(() => {
    refresh();
  }, []);

  const filtered = useMemo(() => {
    const q = query.toLowerCase();
    if (!q) return skills;
    return skills.filter(
      (s) => s.name.toLowerCase().includes(q) || s.description.toLowerCase().includes(q)
    );
  }, [skills, query]);

  return (
    <div className="grid gap-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <h1 className="text-2xl font-semibold tracking-tight">Skills</h1>
        <Button onClick={() => setCreating(true)}>
          <Plus /> New skill
        </Button>
      </div>

      {/* Search is always visible (AKB Design): the most common job here. */}
      <div className="relative">
        <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          value={query}
          onChange={(e) => setParams(e.target.value ? { q: e.target.value } : {}, { replace: true })}
          placeholder="Search skills by name or description…"
          className="pl-9"
        />
      </div>

      {state === "loading" && (
        <div className="grid gap-2">
          {Array.from({ length: 4 }).map((_, i) => (
            <Skeleton key={i} className="h-16 w-full" />
          ))}
        </div>
      )}

      {state === "error" && (
        <Card className="p-6 text-sm text-muted-foreground">
          Could not load skills. <Button variant="link" className="h-auto p-0 align-baseline" onClick={refresh}>Retry</Button>
        </Card>
      )}

      {state === "ready" && filtered.length === 0 && (
        <Card className="p-8 text-center">
          <p className="text-lg font-medium">{query ? "No skills match your search." : "No skills yet."}</p>
          <p className="mt-1 text-sm text-muted-foreground">
            {query ? "Try a different query." : "Create your first skill to get started."}
          </p>
          {!query && (
            <Button className="mt-4" onClick={() => setCreating(true)}>
              <Plus /> New skill
            </Button>
          )}
        </Card>
      )}

      {state === "ready" && filtered.length > 0 && (
        <div className="grid gap-2">
          {filtered.map((skill) => (
            <Link
              key={skill.id}
              to={`/skills/${skill.id}`}
              className="block rounded-lg border bg-card p-4 transition-colors hover:bg-accent"
            >
              <div className="flex items-baseline justify-between gap-3">
                <span className="font-mono font-medium">{skill.name}</span>
                <span className="shrink-0 text-xs text-muted-foreground">{shortDate(skill.updatedAt)}</span>
              </div>
              <p className="mt-1 line-clamp-1 text-sm text-muted-foreground">{skill.description}</p>
            </Link>
          ))}
        </div>
      )}

      <Dialog open={creating} onOpenChange={setCreating}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>New skill</DialogTitle>
            <DialogDescription>Create an Agent Skill directory with a SKILL.md.</DialogDescription>
          </DialogHeader>
          <SkillForm
            submitLabel="Create skill"
            onSubmit={async (data) => {
              const created = await api.createSkill(data);
              setCreating(false);
              setSkills((prev) => [...prev, toSummary(created)]);
            }}
            onCancel={() => setCreating(false)}
          />
        </DialogContent>
      </Dialog>
    </div>
  );
}

function toSummary(s: Skill): SkillSummary {
  return {
    id: s.id,
    name: s.name,
    description: s.description,
    license: s.license,
    compatibility: s.compatibility,
    allowedTools: s.allowedTools,
    metadata: s.metadata,
    createdAt: s.createdAt,
    updatedAt: s.updatedAt,
  };
}
