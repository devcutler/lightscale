import { ReactNode } from "react";

interface Props {
	label: string;
	hint?: string;
	children: ReactNode;
}

export function Field({ label, hint, children }: Props) {
	return (
		<label className="field">
			<span className="field-label">{label}</span>
			{children}
			{hint && <span className="field-hint">{hint}</span>}
		</label>
	);
}
