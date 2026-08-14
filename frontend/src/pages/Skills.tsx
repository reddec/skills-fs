import { useCallback, useEffect, useMemo, useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { Plus, Search, Sparkles } from "lucide-react";
import { toast } from "sonner";
import { api, type Generation, type ServerConfig, type SkillSummary } from "../lib/api";
import { shortDate } from "../lib/utils";
import { Button } from "../components/ui/button";
import { Input } from "../components/ui/input";
import { Card } from "../components/ui/card";
import { Skeleton } from "../components/ui/skeleton";
import { GenerateSkillDialog } from "../components/GenerateSkillDialog";

export function SkillsPage() {
  const navigate = useNavigate();
  const [params, setParams] = useSearchParams();
  const query = params.get("q") ?? "";
  const [skills, setSkills] = useState<SkillSummary[]>([]);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");

  // Agent-based generation is conditional: visible only when the server was started with
  // an LLM API key.
  const [llm, setLlm] = useState<ServerConfig["llm"] | null>(null);
  const [genOpen, setGenOpen] = useState(false);
  const [genJob, setGenJob] = useState<Generation | null>(null);
  const [genSubmitting, setGenSubmitting] = useState(false);

  const refresh = useCallback(async () => {
    setState("loading");
    try {
      setSkills(await api.listSkills());
      setState("ready");
    } catch {
      setState("error");
    }
  }, []);

  useEffect(() => {
    refresh();
    api
      .getConfig()
      .then((cfg) => setLlm(cfg.llm))
      .catch(() => setLlm(null));
  }, [refresh]);

  // Poll the background generation job until it finishes — even with the dialog closed,
  // so completion surfaces as a toast with a link to the new skill.
  useEffect(() => {
    if (!genJob || genJob.status !== "running") return;
    let cancelled = false;
    const tick = async () => {
      try {
        const job = await api.getGeneration(genJob.id);
        if (cancelled) return;
        if (job.status === "done") {
          setGenJob(job);
          if (!genOpen) {
            toast.success("Skill generated", {
              description: `"${job.skillName ?? "Skill"}" was created.`,
              action: {
                label: "Open",
                onClick: () => job.skillId !== undefined && navigate(`/skills/${job.skillId}`),
              },
            });
            refresh();
          }
        } else if (job.status === "error") {
          setGenJob(job);
          if (!genOpen) {
            toast.error("Skill generation failed", {
              description: job.error ?? "Unknown error",
            });
          }
        }
      } catch {
        // transient network error: keep polling
      }
    };
    const timer = setInterval(tick, 1500);
    return () => {
      cancelled = true;
      clearInterval(timer);
    };
  }, [genJob, genOpen, navigate, refresh]);

  async function handleGenerate(idea: string) {
    setGenSubmitting(true);
    try {
      const created = await api.createGeneration(idea);
      setGenJob({ id: created.id, status: "running", createdAt: "", updatedAt: "" });
    } finally {
      setGenSubmitting(false);
    }
  }

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
        <div className="flex gap-2">
          {llm?.enabled && (
            <Button variant="outline" onClick={() => setGenOpen(true)}>
              <Sparkles /> Generate
            </Button>
          )}
          <Button onClick={() => navigate("/skills/new")}>
            <Plus /> New skill
          </Button>
        </div>
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
          Could not load skills.{" "}
          <Button variant="link" className="h-auto p-0 align-baseline" onClick={refresh}>
            Retry
          </Button>
        </Card>
      )}

      {state === "ready" && filtered.length === 0 && (
        <Card className="p-8 text-center">
          <p className="text-lg font-medium">{query ? "No skills match your search." : "No skills yet."}</p>
          <p className="mt-1 text-sm text-muted-foreground">
            {query ? "Try a different query." : "Create your first skill to get started."}
          </p>
          {!query && (
            <div className="mt-4 flex justify-center gap-2">
              {llm?.enabled && (
                <Button variant="outline" onClick={() => setGenOpen(true)}>
                  <Sparkles /> Generate
                </Button>
              )}
              <Button onClick={() => navigate("/skills/new")}>
                <Plus /> New skill
              </Button>
            </div>
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

      <GenerateSkillDialog
        open={genOpen}
        onOpenChange={setGenOpen}
        model={llm?.enabled ? llm.model : null}
        job={genJob}
        submitting={genSubmitting}
        onSubmit={handleGenerate}
        onReset={() => setGenJob(null)}
        onViewSkill={(id) => {
          setGenJob(null);
          setGenOpen(false);
          navigate(`/skills/${id}`);
        }}
      />
    </div>
  );
}
