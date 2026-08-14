import { useState } from "react";
import { Highlight, themes } from "prism-react-renderer";
import { Check, Copy } from "lucide-react";
import { Button } from "./ui/button";

const isDark = typeof window !== "undefined" && window.matchMedia?.("(prefers-color-scheme: dark)").matches;

// Bash renders a bash snippet with syntax highlighting and a copy button. Long lines wrap so
// the block never overflows on narrow screens.
export function Bash({ code }: { code: string }) {
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
    <div className="relative rounded-md border bg-[#011627]">
      <Button
        variant="ghost"
        size="icon"
        className="absolute right-1.5 top-1.5 z-10 size-8 text-muted-foreground hover:text-foreground"
        onClick={copy}
        aria-label="Copy"
      >
        {copied ? <Check /> : <Copy />}
      </Button>
      <Highlight theme={isDark ? themes.nightOwl : themes.nightOwlLight} code={code} language="bash">
        {({ style, tokens, getLineProps, getTokenProps }) => (
          <pre className="overflow-x-auto whitespace-pre-wrap break-words p-3 pr-12 text-xs leading-relaxed" style={style}>
            {tokens.map((line, i) => {
              const lineProps = getLineProps({ line });
              return (
                <div key={i} {...lineProps}>
                  {line.map((token, key) => {
                    const tokenProps = getTokenProps({ token });
                    return <span key={key} {...tokenProps} />;
                  })}
                </div>
              );
            })}
          </pre>
        )}
      </Highlight>
    </div>
  );
}
