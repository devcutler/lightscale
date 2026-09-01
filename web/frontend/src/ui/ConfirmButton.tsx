import { ReactNode, useState } from "react";
import { Trash2 } from "lucide-react";
import { Button } from "./Button";
import { Modal } from "./Modal";

interface Props {
	onConfirm: () => void | Promise<void>;
	target?: ReactNode;
	label?: ReactNode;
	title?: string;
}

export function ConfirmButton({ onConfirm, target, label = "Delete", title }: Props) {
	const [open, setOpen] = useState(false);
	const [busy, setBusy] = useState(false);

	const confirm = async () => {
		setBusy(true);
		try {
			await onConfirm();
			setOpen(false);
		} finally {
			setBusy(false);
		}
	};

	return (
		<>
			<Button variant="ghost-danger" icon={Trash2} onClick={() => setOpen(true)}>
				{label}
			</Button>
			{open && (
				<Modal
					title={title ?? "Delete?"}
					onClose={() => setOpen(false)}
					footer={
						<>
							<Button onClick={() => setOpen(false)}>Cancel</Button>
							<Button variant="danger" icon={Trash2} disabled={busy} onClick={confirm}>
								Delete
							</Button>
						</>
					}
				>
					<div>
						{target != null ? <>Delete {target}?</> : "Are you sure?"} This cannot be undone.
					</div>
				</Modal>
			)}
		</>
	);
}
