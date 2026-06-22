import { useState } from "react";
import { Check, Copy } from "lucide-react";
import { Button } from "./Button";

interface Props {
	text: string;
	label?: string;
}

type State = "idle" | "copied" | "failed";

export function CopyButton({ text, label = "Copy" }: Props) {
	const [state, setState] = useState<State>("idle");

	const copy = async () => {
		try {
			if (navigator.clipboard?.writeText) {
				await navigator.clipboard.writeText(text);
			} else {
				const ta = document.createElement("textarea");
				ta.value = text;
				ta.style.position = "fixed";
				ta.style.opacity = "0";
				document.body.appendChild(ta);
				ta.select();
				const ok = document.execCommand("copy");
				document.body.removeChild(ta);
				if (!ok) throw new Error("copy command rejected");
			}
			setState("copied");
			setTimeout(() => setState("idle"), 1500);
		} catch {
			setState("failed");
			setTimeout(() => setState("idle"), 2500);
		}
	};

	return (
		<Button variant="ghost" icon={state === "copied" ? Check : Copy} onClick={copy}>
			{state === "copied" ? "Copied" : state === "failed" ? "Copy failed - use Download" : label}
		</Button>
	);
}
