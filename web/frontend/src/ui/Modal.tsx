import { ReactNode, useEffect } from "react";
import { X } from "lucide-react";
import { Button } from "./Button";

interface Props {
	title: string;
	onClose: () => void;
	children: ReactNode;
	footer?: ReactNode;
	wide?: boolean;
	onSubmit?: () => void;
	confirmCloseIfDirty?: boolean;
}

export function Modal({
	title,
	onClose,
	children,
	footer,
	wide,
	onSubmit,
	confirmCloseIfDirty,
}: Props) {
	useEffect(() => {
		const onKey = (e: KeyboardEvent) => {
			if (e.key === "Escape") onClose();
		};
		window.addEventListener("keydown", onKey);
		return () => window.removeEventListener("keydown", onKey);
	}, [onClose]);

	const onBackdrop = () => {
		if (
			confirmCloseIfDirty &&
			!window.confirm("Discard your changes and close?")
		) {
			return;
		}
		onClose();
	};

	const head = (
		<div className="modal-head">
			<h2>{title}</h2>
			<Button variant="ghost" icon={X} onClick={onClose} aria-label="Close" />
		</div>
	);

	const body = <div className="modal-body">{children}</div>;
	const foot = footer && <div className="modal-foot">{footer}</div>;
	const content = (
		<>
			{head}
			{body}
			{foot}
		</>
	);

	return (
		<div className="modal-backdrop" onClick={onBackdrop}>
			<div
				className={"modal" + (wide ? " modal-wide" : "")}
				onClick={(e) => e.stopPropagation()}
			>
				{onSubmit ? (
					<form
						onSubmit={(e) => {
							e.preventDefault();
							onSubmit();
						}}
					>
						{content}
					</form>
				) : (
					content
				)}
			</div>
		</div>
	);
}
