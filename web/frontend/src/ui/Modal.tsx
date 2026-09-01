import { ReactNode, useCallback, useEffect } from "react";
import { X } from "lucide-react";
import { Button } from "./Button";
import { escapeHandled } from "./useDismiss";

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
	const requestClose = useCallback(() => {
		if (confirmCloseIfDirty && !window.confirm("Discard your changes and close?")) {
			return;
		}
		onClose();
	}, [confirmCloseIfDirty, onClose]);

	useEffect(() => {
		const onKey = (e: KeyboardEvent) => {
			if (e.key === "Escape" && !escapeHandled(e)) requestClose();
		};
		window.addEventListener("keydown", onKey);
		return () => window.removeEventListener("keydown", onKey);
	}, [requestClose]);

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
		<div className="modal-backdrop" onClick={requestClose}>
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
