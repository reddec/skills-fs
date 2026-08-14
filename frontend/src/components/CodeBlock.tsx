import { useState } from "react";
import { Check, Copy } from "lucide-react";
import { Button } from "./ui/button";
import { cn } from "../lib/utils";

// CodeBlock shows a command/script with a copy button. Long lines wrap (no horizontal
// overflow), so it stays readable inside narrow dialogs and on mobile.
export function CodeBlock({ code, className }: { code: string; className?: string }) {
  const [copied, setCopied] = useState(false);

  async function copy() {
    try {
      await navigator.clipboard.writeText(code);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      /* clipboard unavailable; the user can select the text */
    }
  }

  return (
    <div className={cn("relative flex items-start gap-2 rounded-md border bg-muted/40 p-3", className)}>
      <pre className="flex-1 overflow-x-auto whitespace-pre-wrap break-words font-mono text-xs leading-relaxed">
        <code>{code}</code>
      </pre>
      <Button variant="ghost" size="icon" className="size-8 shrink-0" onClick={copy} aria-label="Copy">
        {copied ? <Check /> : <Copy />}
      </Button>
    </div>
  );
}
