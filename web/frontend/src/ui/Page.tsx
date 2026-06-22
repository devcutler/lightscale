import { ReactNode } from "react";

interface Props {
	title: string;
	actions?: ReactNode;
	error?: string | null;
	children?: ReactNode;
}

export function Page({ title, actions, error, children }: Props) {
	return (
		<div className="page">
			<div className="page-head">
				<h1>{title}</h1>
				{actions && <div className="page-actions">{actions}</div>}
			</div>
			{error && <div className="error-banner">{error}</div>}
			{children}
		</div>
	);
}
