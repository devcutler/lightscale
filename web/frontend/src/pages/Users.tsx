import { useEffect, useMemo, useState } from "react";
import QRCode from "qrcode";
import { Check, Download, FileText, Pencil, Plus } from "lucide-react";
import { Button } from "../ui/Button";
import { useData } from "../ui/useData";
import { useNewParam } from "../ui/useNewParam";
import { del, get, getText, patch, post } from "../api";
import { Peer, User } from "../types";
import { Table, Column } from "../ui/Table";
import { Page } from "../ui/Page";
import { Modal } from "../ui/Modal";
import { FormModal } from "../ui/FormModal";
import { Field } from "../ui/Field";
import { ConfirmButton } from "../ui/ConfirmButton";
import { CopyButton } from "../ui/CopyButton";
import { useAction, useToast } from "../ui/toast";
import { ago, ipKey, NEVER } from "../format";

export function Users() {
	const toast = useToast();
	const run = useAction();
	const users = useData<User[]>(() => get("/users"));
	const peers = useData<Peer[]>(() => get("/peers"), 15000);

	const [creating, setCreating] = useState(false);
	const [editing, setEditing] = useState<User | null>(null);
	const [showConfig, setShowConfig] = useState<User | null>(null);

	useNewParam(() => setCreating(true));

	const peerByUser = useMemo(() => {
		const m = new Map<number, Peer>();
		for (const p of peers.data ?? []) {
			if (!p.user_id) continue;
			const existing = m.get(p.user_id);
			const cur = p.last_handshake_ago_sec ?? Infinity;
			const prev = existing?.last_handshake_ago_sec ?? Infinity;
			if (!existing || cur < prev) m.set(p.user_id, p);
		}
		return m;
	}, [peers.data]);

	const cols: Column<User>[] = [
		{ key: "name", header: "Name", value: (u) => u.name, sortable: true },
		{ key: "ip", header: "IP", value: (u) => u.ip_address ?? "", sortable: true, compare: (a, b) => ipKey(a.ip_address) - ipKey(b.ip_address), render: (u) => u.ip_address || "-" },
		{ key: "email", header: "Email", value: (u) => u.email ?? "", sortable: true, render: (u) => u.email || "-" },
		{
			key: "hs",
			header: "Last handshake",
			value: (u) => peerByUser.get(u.id)?.last_handshake_ago_sec ?? NEVER,
			sortable: true,
			render: (u) => ago(peerByUser.get(u.id)?.last_handshake_ago_sec),
		},
		{
			key: "actions",
			header: "",
			render: (u) => (
				<div className="row-actions">
					<Button variant="ghost" icon={FileText} onClick={() => setShowConfig(u)}>
						Config
					</Button>
					<Button variant="ghost" icon={Pencil} onClick={() => setEditing(u)}>
						Edit
					</Button>
					<ConfirmButton
						title="Delete user?"
						target={u.name}
						onConfirm={() =>
							run(async () => {
								await del(`/users/${u.id}`);
								toast.ok(`Deleted ${u.name}`);
								users.reload();
							})
						}
					/>
				</div>
			),
		},
	];

	return (
		<Page
			title="Users"
			error={users.error}
			actions={
				<Button variant="primary" icon={Plus} onClick={() => setCreating(true)}>
					Add user
				</Button>
			}
		>
			<Table
				columns={cols}
				rows={users.data ?? []}
				rowKey={(u) => u.id}
				defaultSort={[{ key: "ip" }]}
				filterPlaceholder="Filter users..."
				empty="No users yet. Add one to get started."
			/>

			{creating && (
				<UserModal
					onClose={() => setCreating(false)}
					onSaved={(u) => {
						setCreating(false);
						users.reload();
						setShowConfig(u);
					}}
				/>
			)}
			{editing && (
				<UserModal
					user={editing}
					onClose={() => setEditing(null)}
					onSaved={() => {
						setEditing(null);
						users.reload();
					}}
				/>
			)}
			{showConfig && (
				<ConfigModal user={showConfig} onClose={() => setShowConfig(null)} />
			)}
		</Page>
	);
}

function UserModal({
	user,
	onClose,
	onSaved,
}: {
	user?: User;
	onClose: () => void;
	onSaved: (u: User) => void;
}) {
	const toast = useToast();
	const editing = !!user;

	const [name, setName] = useState(user?.name ?? "");
	const [email, setEmail] = useState(user?.email ?? "");
	const [ip, setIp] = useState(user?.ip_address ?? "");
	const [endpoint, setEndpoint] = useState(user?.endpoint ?? "");

	const onSubmit = async () => {
		if (editing) {
			await patch(`/users/${user!.id}`, { name, email, endpoint });
			toast.ok(`Updated ${name}`);
			onSaved(user!);
		} else {
			const u = await post<User>("/users", { name, email, ip, endpoint });
			toast.ok(`Created ${u.name}`);
			onSaved(u);
		}
	};

	return (
		<FormModal
			title={editing ? `Edit ${user!.name}` : "Add user"}
			onClose={onClose}
			onSubmit={onSubmit}
			submitLabel={editing ? "Save" : "Create"}
			submitIcon={editing ? Check : Plus}
			canSubmit={!!name}
			dirty={!editing && !!(name || email || ip || endpoint)}
		>
			<Field label="Name">
				<input autoFocus value={name} onChange={(e) => setName(e.target.value)} />
			</Field>
			<Field label="Email" hint="optional">
				<input value={email} onChange={(e) => setEmail(e.target.value)} />
			</Field>
			<Field label="IP address" hint={editing ? "the assigned IP can't be changed after creation" : "leave blank to auto-assign the next free 10.6.0.x"}>
				<input value={ip} disabled={editing} onChange={(e) => setIp(e.target.value)} />
			</Field>
			<Field label="Endpoint" hint="optional override of the public endpoint">
				<input value={endpoint} onChange={(e) => setEndpoint(e.target.value)} />
			</Field>
		</FormModal>
	);
}

function ConfigModal({ user, onClose }: { user: User; onClose: () => void; }) {
	const cfg = useData<string>(() => getText(`/users/${user.id}/config`));
	const [qr, setQr] = useState<string | null>(null);
	const [qrFailed, setQrFailed] = useState(false);

	useEffect(() => {
		if (!cfg.data) return;
		let cancelled = false;
		setQrFailed(false);
		QRCode.toDataURL(cfg.data, { margin: 1, width: 320 })
			.then((url) => {
				if (!cancelled) setQr(url);
			})
			.catch(() => {
				if (!cancelled) setQrFailed(true);
			});
		return () => {
			cancelled = true;
		};
	}, [cfg.data]);

	const download = () => {
		if (!cfg.data) return;
		const blob = new Blob([cfg.data], { type: "text/plain" });
		const url = URL.createObjectURL(blob);
		const a = document.createElement("a");
		a.href = url;
		a.download = `${user.name.replace(/\s+/g, "-")}.conf`;
		a.click();
		URL.revokeObjectURL(url);
	};

	return (
		<Modal
			title={`WireGuard config - ${user.name}`}
			onClose={onClose}
			wide
			footer={
				<>
					<Button onClick={onClose}>Close</Button>
					{cfg.data && <CopyButton text={cfg.data} label="Copy config" />}
					{cfg.data && (
						<Button variant="primary" icon={Download} onClick={download}>
							Download .conf
						</Button>
					)}
				</>
			}
		>
			{cfg.error && (
				<div className="error-banner">Couldn't load the config: {cfg.error}</div>
			)}
			<div className="config-grid">
				<div className="qr-box">
					{qr ? (
						<img src={qr} alt="WireGuard QR" />
					) : qrFailed || cfg.error ? (
						<div className="qr-loading">QR unavailable</div>
					) : (
						<div className="qr-loading">generating...</div>
					)}
					<div className="field-hint">Scan with the WireGuard app</div>
				</div>
				<pre>
					{cfg.data ?? (cfg.error ? "Failed to load config." : "loading...")}
				</pre>
			</div>
			{cfg.data && (
				<div className="field-hint">
					This config contains the device's private key - share it securely.
				</div>
			)}
		</Modal>
	);
}
