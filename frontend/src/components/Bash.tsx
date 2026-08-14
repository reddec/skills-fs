import { useMemo, useState } from "react";
import hljs from "highlight.js/lib/core";
import bash from "highlight.js/lib/languages/bash";
import { Check, Copy } from "lucide-react";
import { Button } from "./ui/button";

hljs.registerLanguage("bash", bash);

// Bash renders a bash snippet with syntax highlighting and a copy button. Long lines wrap so
// the block never overflows on narrow screens. Token colors are defined in index.css (.hljs).
export function Bash({ code }: { code: string }) {
  const [copied, setCopied] = useState(false);
  const html = useMemo(() => hljs.highlight(code, { language: "bash" }).value, [code]);

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
    <div className="relative overflow-hidden rounded-md border bg-[#282c34]">
      <Button
        variant="ghost"
        size="icon"
        className="absolute right-1.5 top-1.5 z-10 size-8 text-[#9da7b3] hover:bg-white/10 hover:text-white"
        onClick={copy}
        aria-label="Copy"
      >
        {copied ? <Check /> : <Copy />}
      </Button>
      <pre className="overflow-x-auto whitespace-pre-wrap break-words p-3 pr-12 text-xs leading-relaxed">
        <code className="hljs" dangerouslySetInnerHTML={{ __html: html }} />
      </pre>
    </div>
  );
}
