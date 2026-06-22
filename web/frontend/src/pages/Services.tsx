import { useState } from "react";
import { Check, Pencil, Plus } from "lucide-react";
import { Button } from "../ui/Button";
import { useData } from "../ui/useData";
import { useNewParam } from "../ui/useNewParam";
import { del, get, patch, post } from "../api";
import { ContainerSummary, Service } from "../types";
import { Table, Column } from "../ui/Table";
import { Page } from "../ui/Page";
import { FormModal } from "../ui/FormModal";
import { Combobox } from "../ui/Combobox";
import { Field } from "../ui/Field";
import { ConfirmButton } from "../ui/ConfirmButton";
import { useAction, useToast } from "../ui/toast";
import { ipKey, portsLabel } from "../format";

const firstPort = (s: Service): number =>
	s.ports.length ? Math.min(...s.ports.map((p) => p.port)) : Number.MAX_SAFE_INTEGER;

export function Services() {
	const toast = useToast();
	const run = useAction();
	const services = useData<Service[]>(() => get("/services"));
	const [creating, setCreating] = useState(false);
	const [editing, setEditing] = useState<Service | null>(null);

	useNewParam(() => setCreating(true));

	const cols: Column<Service>[] = [
		{ key: "name", header: "Name", value: (s) => s.name, sortable: true },
		{ key: "ip", header: "IP", value: (s) => s.ip_address ?? "", sortable: true, compare: (a, b) => ipKey(a.ip_address) - ipKey(b.ip_address), render: (s) => s.ip_address || "-" },
		{ key: "origin", header: "Origin", value: (s) => s.origin, sortable: true },
		{ key: "hostname", header: "Hostname", value: (s) => s.hostname },
		{ key: "ports", header: "Ports", value: (s) => portsLabel(s.ports), sortable: true, compare: (a, b) => firstPort(a) - firstPort(b), render: (s) => portsLabel(s.ports) || "-" },
		{
			key: "actions",
			header: "",
			render: (s) => (
				<div className="row-actions">
					<Button variant="ghost" icon={Pencil} onClick={() => setEditing(s)}>
						Edit
					</Button>
					<ConfirmButton
						title="Delete service?"
						target={s.name}
						onConfirm={() =>
							run(async () => {
								await del(`/services/${s.id}`);
								toast.ok(`Deleted ${s.name}`);
								services.reload();
							})
						}
					/>
				</div>
			),
		},
	];

	return (
		<Page
			title="Services"
			error={services.error}
			actions={
				<Button variant="primary" icon={Plus} onClick={() => setCreating(true)}>
					Add service
				</Button>
			}
		>
			<Table
				columns={cols}
				rows={services.data ?? []}
				rowKey={(s) => s.id}
				defaultSort={[{ key: "ip" }, { key: "ports" }]}
				filterPlaceholder="Filter services..."
				empty="No services yet."
			/>

			{creating && (
				<ServiceModal
					onClose={() => setCreating(false)}
					onSaved={() => {
						setCreating(false);
						services.reload();
					}}
				/>
			)}
			{editing && (
				<ServiceModal
					service={editing}
					onClose={() => setEditing(null)}
					onSaved={() => {
						setEditing(null);
						services.reload();
					}}
				/>
			)}
		</Page>
	);
}

function ServiceModal({
	service,
	onClose,
	onSaved,
}: {
	service?: Service;
	onClose: () => void;
	onSaved: () => void;
}) {
	const toast = useToast();
	const containers = useData<ContainerSummary[]>(() => get("/discover/containers"));
	const editing = !!service;

	const [name, setName] = useState(service?.name ?? "");
	const [origin, setOrigin] = useState(service?.origin ?? "");
	const [ports, setPorts] = useState(service ? portsLabel(service.ports) : "");
	const [hostname, setHostname] = useState(service?.hostname ?? "");
	const [ip, setIp] = useState(service?.ip_address ?? "");
	const [description, setDescription] = useState(service?.description ?? "");

	const onSubmit = async () => {
		const payload = { name, origin, ports, hostname, ip, description };
		if (editing) {
			await patch(`/services/${service!.id}`, payload);
			toast.ok(`Updated ${name}`);
		} else {
			await post("/services", payload);
			toast.ok(`Created ${name}`);
		}
		onSaved();
	};

	const containerCount = containers.data?.length ?? 0;
	const originHint = containers.error
		? "couldn't list containers - type a container name or host address"
		: containerCount > 0
			? `start typing to pick from ${containerCount} running containers, or enter a host address`
			: "a Docker container or host address that backs this service";

	return (
		<FormModal
			title={editing ? `Edit ${service!.name}` : "Add service"}
			onClose={onClose}
			onSubmit={onSubmit}
			submitLabel={editing ? "Save" : "Create"}
			submitIcon={editing ? Check : Plus}
			canSubmit={!!name}
			dirty={!editing && !!(name || origin || ports || hostname || ip || description)}
		>
			<Field label="Name">
				<input autoFocus value={name} onChange={(e) => setName(e.target.value)} />
			</Field>
			<Field label="Origin" hint={originHint}>
				<Combobox
					value={origin}
					onChange={setOrigin}
					placeholder="container name or host address"
					options={[
						{ value: "host", label: "host (this machine)" },
						...((containers.data ?? []).length ? [{ separator: true as const }] : []),
						...(containers.data ?? []).map((c) => ({
							value: c.name,
							label: c.ip ? `${c.name} (${c.ip})` : c.name,
						})),
					]}
				/>
			</Field>
			<Field label="Ports" hint="e.g. 80,443/tcp,5353/udp - bare port means tcp+udp">
				<input value={ports} onChange={(e) => setPorts(e.target.value)} />
			</Field>
			<Field label="Hostname" hint="optional; auto-derived if blank">
				<input value={hostname} onChange={(e) => setHostname(e.target.value)} />
			</Field>
			<Field label="IP address" hint="optional; auto-assigned from the service subnet if blank">
				<input value={ip} onChange={(e) => setIp(e.target.value)} />
			</Field>
			<Field label="Description" hint="optional">
				<input value={description} onChange={(e) => setDescription(e.target.value)} />
			</Field>
		</FormModal>
	);
}
