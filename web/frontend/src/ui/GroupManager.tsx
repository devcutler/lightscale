import { ReactNode, useState } from "react";
import { ChevronDown, ChevronRight, Plus, UserPlus, X } from "lucide-react";
import { Button } from "./Button";
import { useData } from "./useData";
import { del, get, post } from "../api";
import { FormModal } from "./FormModal";
import { Page } from "./Page";
import { Select } from "./Select";
import { Field } from "./Field";
import { ConfirmButton } from "./ConfirmButton";
import { useAction, useToast } from "./toast";

interface Named {
	id: number;
	name: string;
}

export interface GroupManagerConfig<G extends Named> {
	title: string;
	basePath: string;
	membersSourcePath: string;
	memberIdField: string;
	emptyText: string;
	badge?: (group: G) => ReactNode;
	extraControls?: (group: G, reload: () => void) => ReactNode;
	createExtras?: (
		extra: Record<string, unknown>,
		setExtra: (patch: Record<string, unknown>) => void,
	) => ReactNode;
	createBody?: (name: string, extra: Record<string, unknown>) => Record<string, unknown>;
}

export function GroupManager<G extends Named>(config: GroupManagerConfig<G>) {
	const groups = useData<G[]>(() => get(config.basePath));
	const allMembers = useData<Named[]>(() => get(config.membersSourcePath));
	const [creating, setCreating] = useState(false);
	const [expanded, setExpanded] = useState<Set<number>>(new Set());
	const toggle = (id: number) =>
		setExpanded((prev) => {
			const next = new Set(prev);
			if (next.has(id)) next.delete(id);
			else next.add(id);
			return next;
		});

	const list = groups.data ?? [];

	return (
		<Page
			title={config.title}
			error={groups.error}
			actions={
				<Button variant="primary" icon={Plus} onClick={() => setCreating(true)}>
					Add group
				</Button>
			}
		>
			<div className="group-list">
				{list.map((g) => (
					<div className="group-card" key={g.id}>
						<div className="group-head" onClick={() => toggle(g.id)}>
							<div className="group-title">
								<span className="caret">
									{expanded.has(g.id) ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
								</span>
								{g.name}
								{config.badge?.(g)}
							</div>
						</div>
						{expanded.has(g.id) && (
							<GroupBody
								group={g}
								config={config}
								all={allMembers.data ?? []}
								onChanged={() => groups.reload()}
							/>
						)}
					</div>
				))}
				{list.length === 0 && (
					<div className="empty-cell">{config.emptyText}</div>
				)}
			</div>

			{creating && (
				<CreateGroupModal
					config={config}
					onClose={() => setCreating(false)}
					onSaved={() => {
						setCreating(false);
						groups.reload();
					}}
				/>
			)}
		</Page>
	);
}

function GroupBody<G extends Named>({
	group,
	config,
	all,
	onChanged,
}: {
	group: G;
	config: GroupManagerConfig<G>;
	all: Named[];
	onChanged: () => void;
}) {
	const toast = useToast();
	const run = useAction();
	const members = useData<Named[]>(() => get(`${config.basePath}/${group.id}/members`));
	const [addId, setAddId] = useState("");

	const ms = members.data ?? [];
	const memberIds = new Set(ms.map((m) => m.id));
	const candidates = all.filter((m) => !memberIds.has(m.id));

	const addMember = () => {
		if (!addId) return;
		const m = candidates.find((c) => c.id === Number(addId));
		run(async () => {
			await post(`${config.basePath}/${group.id}/members`, {
				[config.memberIdField]: Number(addId),
			});
			setAddId("");
			if (m) toast.ok(`Added ${m.name} to ${group.name}`);
			members.reload();
		});
	};

	const removeMember = (id: number, name: string) =>
		run(async () => {
			await del(`${config.basePath}/${group.id}/members/${id}`);
			toast.ok(`Removed ${name} from ${group.name}`);
			members.reload();
		});

	return (
		<div className="group-body" onClick={(e) => e.stopPropagation()}>
			{members.error && (
				<div className="error-banner">Couldn't load members: {members.error}</div>
			)}
			<div className="chips">
				{members.loading && !members.data ? (
					<span className="field-hint">Loading members...</span>
				) : (
					<>
						{ms.map((m) => (
							<span className="chip" key={m.id}>
								{m.name}
								<button
									className="chip-x"
									title={`Remove ${m.name}`}
									aria-label={`Remove ${m.name}`}
									onClick={() => removeMember(m.id, m.name)}
								>
									<X size={14} />
								</button>
							</span>
						))}
						{ms.length === 0 && !members.error && (
							<span className="field-hint">No members.</span>
						)}
					</>
				)}
			</div>
			<div className="group-controls">
				<Select
					value={addId}
					onChange={setAddId}
					placeholder="Add member..."
					options={candidates.map((m) => ({ value: String(m.id), label: m.name }))}
				/>
				<Button icon={UserPlus} disabled={!addId} onClick={addMember}>
					Add
				</Button>
				{config.extraControls?.(group, onChanged)}
				<ConfirmButton
					label="Delete group"
					title="Delete group?"
					target={group.name}
					onConfirm={() =>
						run(async () => {
							await del(`${config.basePath}/${group.id}`);
							toast.ok(`Deleted ${group.name}`);
							onChanged();
						})
					}
				/>
			</div>
		</div>
	);
}

function CreateGroupModal<G extends Named>({
	config,
	onClose,
	onSaved,
}: {
	config: GroupManagerConfig<G>;
	onClose: () => void;
	onSaved: () => void;
}) {
	const toast = useToast();
	const [name, setName] = useState("");
	const [extra, setExtraState] = useState<Record<string, unknown>>({});
	const setExtra = (patch: Record<string, unknown>) =>
		setExtraState((prev) => ({ ...prev, ...patch }));

	const onSubmit = async () => {
		const body = config.createBody ? config.createBody(name, extra) : { name };
		await post(config.basePath, body);
		toast.ok(`Created ${name}`);
		onSaved();
	};

	return (
		<FormModal
			title={`Add ${config.title.replace(/s$/, "").toLowerCase()}`}
			onClose={onClose}
			onSubmit={onSubmit}
			submitLabel="Create"
			submitIcon={Plus}
			canSubmit={!!name}
			dirty={!!name}
		>
			<Field label="Name">
				<input autoFocus value={name} onChange={(e) => setName(e.target.value)} />
			</Field>
			{config.createExtras?.(extra, setExtra)}
		</FormModal>
	);
}
