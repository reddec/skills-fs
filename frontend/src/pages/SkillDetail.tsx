import { useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import ReactMarkdown from "react-markdown";
import { ArrowLeft, Pencil, Trash2 } from "lucide-react";
import { api, type Skill } from "../lib/api";
import { shortDate } from "../lib/utils";
import { Button } from "../components/ui/button";
import { Card } from "../components/ui/card";
import { Skeleton } from "../components/ui/skeleton";
import { Badge } from "../components/ui/badge";
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

export function SkillDetailPage() {
  const { id } = useParams();
  const navigate = useNavigate();
  const [skill, setSkill] = useState<Skill | null>(null);
  const [state, setState] = useState<"loading" | "ready" | "error" | "missing">("loading");

  async function refresh() {
    if (!id) return;
    setState("loading");
    try {
      setSkill(await api.getSkill(Number(id)));
      setState("ready");
    } catch (err) {
      setState(err instanceof Error && err.message.includes("not found") ? "missing" : "error");
    }
  }

  useEffect(() => {
    refresh();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id]);

  async function handleDelete() {
    if (!skill) return;
    try {
      await api.deleteSkill(skill.id);
      navigate("/skills");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to delete skill.");
    }
  }

  if (state === "loading") {
    return (
      <div className="grid gap-4">
        <Skeleton className="h-8 w-40" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }

  if (state === "missing" || state === "error" || !skill) {
    return (
      <Card className="p-8 text-center">
        <p className="text-lg font-medium">{state === "missing" ? "Skill not found." : "Could not load skill."}</p>
        <Button variant="link" asChild className="mt-2">
          <Link to="/skills">Back to skills</Link>
        </Button>
      </Card>
    );
  }

  const entries = Object.entries(skill.metadata);

  return (
    <div className="grid gap-6">
      <div className="flex items-center justify-between gap-3">
        <Button variant="ghost" size="sm" asChild>
          <Link to="/skills">
            <ArrowLeft /> Skills
          </Link>
        </Button>
        <div className="flex gap-2">
          <Button variant="outline" onClick={() => navigate(`/skills/${skill.id}/edit`)}>
            <Pencil /> Edit
          </Button>
          <AlertDialog>
            <AlertDialogTrigger asChild>
              <Button variant="destructive">
                <Trash2 /> Delete
              </Button>
            </AlertDialogTrigger>
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>Delete skill {skill.name}?</AlertDialogTitle>
                <AlertDialogDescription>
                  This permanently removes the skill directory <span className="font-mono">{skill.name}</span> and its
                  SKILL.md. Agents will no longer see it. This cannot be undone.
                </AlertDialogDescription>
              </AlertDialogHeader>
              <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
                <AlertDialogCancel>Cancel</AlertDialogCancel>
                <AlertDialogAction onClick={handleDelete}>Delete skill</AlertDialogAction>
              </div>
            </AlertDialogContent>
          </AlertDialog>
        </div>
      </div>

      <div>
        <h1 className="font-mono text-3xl font-semibold tracking-tight">{skill.name}</h1>
        <p className="mt-2 text-muted-foreground">{skill.description}</p>
      </div>

      <dl className="grid gap-3 text-sm sm:grid-cols-2">
        {skill.license && <Field label="License" value={skill.license} />}
        {skill.compatibility && <Field label="Compatibility" value={skill.compatibility} />}
        {skill.allowedTools && <Field label="Allowed tools" value={skill.allowedTools} />}
        {entries.length > 0 && (
          <div className="grid gap-1">
            <dt className="font-medium">Metadata</dt>
            <dd className="flex flex-wrap gap-2">
              {entries.map(([key, value]) => (
                <Badge key={key} variant="muted">
                  {key}: {value}
                </Badge>
              ))}
            </dd>
          </div>
        )}
        <Field label="Updated" value={shortDate(skill.updatedAt)} />
      </dl>

      <Card className="p-6">
        <article className="prose prose-sm max-w-none dark:prose-invert">
          <ReactMarkdown>{skill.body || "_(No body.)_"}</ReactMarkdown>
        </article>
      </Card>
    </div>
  );
}

function Field({ label, value }: { label: string; value: string }) {
  return (
    <div className="grid gap-1">
      <dt className="font-medium">{label}</dt>
      <dd className="text-muted-foreground">{value}</dd>
    </div>
  );
}
