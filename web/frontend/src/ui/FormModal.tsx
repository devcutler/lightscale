import { ReactNode } from "react";
import { LucideIcon } from "lucide-react";
import { Modal } from "./Modal";
import { Button } from "./Button";
import { useSubmit } from "./useSubmit";

interface Props {
	title: string;
	onClose: () => void;
	onSubmit: () => Promise<void>;
	submitLabel: string;
	submitIcon: LucideIcon;
	canSubmit: boolean;
	dirty?: boolean;
	wide?: boolean;
	children: ReactNode;
}

export function FormModal({
	title,
	onClose,
	onSubmit,
	submitLabel,
	submitIcon,
	canSubmit,
	dirty,
	wide,
	children,
}: Props) {
	const { submit, busy, err } = useSubmit(onSubmit);

	return (
		<Modal
			title={title}
			onClose={onClose}
			onSubmit={submit}
			confirmCloseIfDirty={dirty}
			wide={wide}
			footer={
				<>
					<Button onClick={onClose}>Cancel</Button>
					<Button variant="primary" icon={submitIcon} type="submit" disabled={busy || !canSubmit}>
						{submitLabel}
					</Button>
				</>
			}
		>
			{err && <div className="error-banner">{err}</div>}
			{children}
		</Modal>
	);
}
