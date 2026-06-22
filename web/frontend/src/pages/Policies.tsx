import { useState } from "react";
import { ArrowRight, Plus } from "lucide-react";
import { Button } from "../ui/Button";
import { useData } from "../ui/useData";
import { del, get, post } from "../api";
import { ApiObject, Policy, Principal } from "../types";
import { Table, Column } from "../ui/Table";
import { Page } from "../ui/Page";
import { FormModal } from "../ui/FormModal";
import { Select } from "../ui/Select";
import { Field } from "../ui/Field";
import { ConfirmButton } from "../ui/ConfirmButton";
import { useAction, useToast } from "../ui/toast";

const TYPE_LABEL: Record<string, string> = {
	user: "user",
	user_group: "user group",
	service: "service",
	service_group: "service group",
};

function TypeBadge({ type }: { type: string; }) {
	return <span className={`type-badge tb-${type}`}>{TYPE_LABEL[type] ?? type}</span>;
}

export function Policies() {
	const toast = useToast();
	const run = useAction();
	const policies = useData<Policy[]>(() => get("/policies"));
	const [creating, setCreating] = useState(false);

	const cols: Column<Policy>[] = [
		{
			key: "subject",
			header: "Subject",
			value: (p) => p.subject_name,
			sortable: true,
			render: (p) => (
				<span>
					<TypeBadge type={p.subject_type} /> {p.subject_name}
				</span>
			),
		},
		{
			key: "arrow",
			header: "",
			render: () => (
				<span className="dim policy-arrow-cell">
					may connect to <ArrowRight size={14} />
				</span>
			),
		},
		{
			key: "object",
			header: "Object",
			value: (p) => p.object_name,
			sortable: true,
			render: (p) => (
				<span>
					<TypeBadge type={p.object_type} /> {p.object_name}
				</span>
			),
		},
		{
			key: "actions",
			header: "",
			render: (p) => (
				<ConfirmButton
					title="Delete policy?"
					target={<span className="confirm-inline">{p.subject_name} <ArrowRight size={13} /> {p.object_name}</span>}
					onConfirm={() =>
						run(async () => {
							await del(`/policies/${p.id}`);
							toast.ok(<span className="toast-inline">Removed {p.subject_name} <ArrowRight size={13} /> {p.object_name}</span>);
							policies.reload();
						})
					}
				/>
			),
		},
	];

	return (
		<Page
			title="Policies"
			error={policies.error}
			actions={
				<Button variant="primary" icon={Plus} onClick={() => setCreating(true)}>
					Add policy
				</Button>
			}
		>
			<Table
				columns={cols}
				rows={policies.data ?? []}
				rowKey={(p) => p.id}
				filterPlaceholder="Filter policies..."
				empty="No policies yet. Add one to grant access."
			/>

			{creating && (
				<PolicyModal
					onClose={() => setCreating(false)}
					onSaved={() => {
						setCreating(false);
						policies.reload();
					}}
				/>
			)}
		</Page>
	);
}

function PolicyModal({
	onClose,
	onSaved,
}: {
	onClose: () => void;
	onSaved: () => void;
}) {
	const toast = useToast();
	const principals = useData<Principal[]>(() => get("/principals"));
	const objects = useData<ApiObject[]>(() => get("/objects"));

	const [subject, setSubject] = useState("");
	const [object, setObject] = useState("");

	const onSubmit = async () => {
		const sp = (principals.data ?? []).find((p) => `${p.type}:${p.id}` === subject);
		const ob = (objects.data ?? []).find((o) => `${o.type}:${o.id}` === object);
		if (!sp || !ob) throw new Error("Select both a subject and an object.");
		await post("/policies", {
			subject_type: sp.type,
			subject_id: sp.id,
			object_type: ob.type,
			object_id: ob.id,
			action: "allow",
		});
		toast.ok("Policy added");
		onSaved();
	};

	const loadErr = principals.error || objects.error;

	return (
		<FormModal
			title="Add policy"
			onClose={onClose}
			onSubmit={onSubmit}
			submitLabel="Add"
			submitIcon={Plus}
			canSubmit={!!(subject && object)}
			dirty={!!(subject || object)}
		>
			{loadErr && (
				<div className="error-banner">Couldn't load options: {loadErr}</div>
			)}
			<Field label="Subject" hint="who is granted access">
				<Select
					value={subject}
					onChange={setSubject}
					placeholder="Select subject..."
					options={(principals.data ?? []).map((p) => ({
						value: `${p.type}:${p.id}`,
						label: `${TYPE_LABEL[p.type]}: ${p.name}`,
					}))}
				/>
			</Field>
			<div className="policy-arrow">
				<ArrowRight size={14} /> may connect to <ArrowRight size={14} />
			</div>
			<Field label="Object" hint="what they may reach">
				<Select
					value={object}
					onChange={setObject}
					placeholder="Select object..."
					options={(objects.data ?? []).map((o) => ({
						value: `${o.type}:${o.id}`,
						label: `${TYPE_LABEL[o.type]}: ${o.name}`,
					}))}
				/>
			</Field>
		</FormModal>
	);
}
