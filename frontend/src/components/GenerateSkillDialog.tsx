import { useState } from "react";
import { AlertCircle, CheckCircle2, Loader2, Sparkles } from "lucide-react";
import type { Generation } from "../lib/api";
import { Button } from "./ui/button";
import { Textarea } from "./ui/textarea";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "./ui/dialog";

export interface GenerateSkillDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** LLM model name for the hint, or null when unknown. */
  model: string | null;
  /** Live state of the running/completed job; null when nothing was started yet. */
  job: Generation | null;
  /** True while the create request is in flight. */
  submitting: boolean;
  /** Starts a generation job; rejects with the server's message on failure. */
  onSubmit: (idea: string) => Promise<void>;
  /** Clears the finished job so the dialog returns to the form. */
  onReset: () => void;
  /** Opens the created skill. */
  onViewSkill: (skillId: number) => void;
}

/**
 * Dialog for turning a raw idea or draft into a complete skill via the background
 * generation agent. Generation continues server-side after the dialog closes.
 */
export function GenerateSkillDialog({
  open,
  onOpenChange,
  model,
  job,
  submitting,
  onSubmit,
  onReset,
  onViewSkill,
}: GenerateSkillDialogProps) {
  const [idea, setIdea] = useState("");
  const [submitError, setSubmitError] = useState<string | null>(null);

  async function handleSubmit() {
    if (!idea.trim() || submitting) return;
    setSubmitError(null);
    try {
      await onSubmit(idea);
      setIdea("");
    } catch (err) {
      setSubmitError(err instanceof Error ? err.message : String(err));
    }
  }

  const running = job !== null && job.status === "running";
  const done = job !== null && job.status === "done";
  const failed = job !== null && job.status === "error";

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Sparkles className="h-4 w-4 text-muted-foreground" />
            Generate skill
          </DialogTitle>
          <DialogDescription>
            Dump an idea or draft — an agent will turn it into a complete skill in the
            background. You can close this window; it keeps running.
          </DialogDescription>
        </DialogHeader>

        {job === null && (
          <>
            <Textarea
              autoFocus
              value={idea}
              onChange={(e) => setIdea(e.target.value)}
              placeholder="e.g. A skill for writing conventional commit messages: analyze the staged diff, then suggest a message following the Conventional Commits format…"
              className="min-h-40"
            />
            {model && <p className="text-xs text-muted-foreground">Generating with {model}</p>}
            {submitError && <p className="text-sm text-destructive">{submitError}</p>}
            <DialogFooter>
              <Button variant="outline" onClick={() => onOpenChange(false)}>
                Cancel
              </Button>
              <Button onClick={handleSubmit} disabled={!idea.trim() || submitting}>
                {submitting && <Loader2 className="h-4 w-4 animate-spin" />}
                Generate
              </Button>
            </DialogFooter>
          </>
        )}

        {running && (
          <div className="flex flex-col items-center gap-3 py-4 text-center">
            <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
            <p className="text-sm font-medium">Generating…</p>
            <p className="text-xs text-muted-foreground">
              This may take a minute or two. Closing the dialog won't cancel it.
            </p>
          </div>
        )}

        {done && (
          <div className="flex flex-col items-center gap-3 py-4 text-center">
            <CheckCircle2 className="h-8 w-8 text-emerald-500" />
            <p className="text-sm font-medium">Skill “{job.skillName ?? "generated"}” is ready</p>
            <p className="text-xs text-muted-foreground">
              The skill was saved and is already served on the mount.
            </p>
            <DialogFooter>
              <Button variant="outline" onClick={onReset}>
                Close
              </Button>
              <Button onClick={() => job.skillId !== undefined && onViewSkill(job.skillId)}>
                View skill
              </Button>
            </DialogFooter>
          </div>
        )}

        {failed && (
          <div className="flex flex-col items-center gap-3 py-4 text-center">
            <AlertCircle className="h-8 w-8 text-destructive" />
            <p className="text-sm font-medium">Generation failed</p>
            <p className="max-w-full break-words text-xs text-muted-foreground">
              {job.error ?? "Unknown error"}
            </p>
            <DialogFooter>
              <Button variant="outline" onClick={() => onOpenChange(false)}>
                Close
              </Button>
              <Button onClick={onReset}>Try again</Button>
            </DialogFooter>
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}
