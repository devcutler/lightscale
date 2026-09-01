import { useState } from "react";
import { Check, Pencil, Plus } from "lucide-react";
import { Button } from "../ui/Button";
import { useData } from "../ui/useData";
import { useNewParam } from "../ui/useNewParam";
import { del, get, patch, post } from "../api";
import { ContainerSummary, OriginKind, Service } from "../types";
import { Table, Column } from "../ui/Table";
import { Page } from "../ui/Page";
import { FormModal } from "../ui/FormModal";
import { Combobox } from "../ui/Combobox";
import { Field } from "../ui/Field";
import { ConfirmButton } from "../ui/ConfirmButton";
import { useAction, useToast } from "../ui/toast";
import { ipKey, portsLabel } from "../format";

const originValueLabel = (s: Service): string =>
	s.origin_kind === "host" ? "(this machine)" : s.origin_value;

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
		{ key: "ip", header: "Internal IP", value: (s) => s.ip_address ?? "", sortable: true, compare: (a, b) => ipKey(a.ip_address) - ipKey(b.ip_address), render: (s) => s.ip_address || "-" },
		{ key: "kind", header: "Kind", value: (s) => s.origin_kind, sortable: true },
		{
			key: "backend",
			header: "Backend",
			value: (s) => originValueLabel(s),
			sortable: true,
			render: (s) => originValueLabel(s),
		},
		{ key: "hostname", header: "Domain", value: (s) => s.hostname },
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
	const [originKind, setOriginKind] = useState<OriginKind>(service?.origin_kind ?? "container");
	const [originValue, setOriginValue] = useState(service?.origin_value ?? "");
	const [originNetwork, setOriginNetwork] = useState(service?.origin_network ?? "");
	const [ports, setPorts] = useState(service ? portsLabel(service.ports) : "");
	const [hostname, setHostname] = useState(service?.hostname ?? "");
	const [ip, setIp] = useState(service?.ip_address ?? "");
	const [description, setDescription] = useState(service?.description ?? "");

	const changeKind = (k: OriginKind) => {
		setOriginKind(k);
		if (k === "host") setOriginValue("");
		if (k !== "container") setOriginNetwork("");
	};

	const onSubmit = async () => {
		const payload = {
			name,
			origin_kind: originKind,
			origin_value: originKind === "host" ? "" : originValue,
			origin_network: originKind === "container" ? originNetwork : "",
			ports,
			hostname,
			ip,
			description,
		};
		if (editing) {
			await patch(`/services/${service!.id}`, payload);
			toast.ok(`Updated ${name}`);
		} else {
			await post("/services", payload);
			toast.ok(`Created ${name}`);
		}
		onSaved();
	};

	const containerOptions = (containers.data ?? []).map((c) => ({
		value: c.name,
		label: c.shared ? c.name : `${c.name} (no shared network)`,
	}));
	const containerHint = containers.error
		? "couldn't list containers, so type the name yourself"
		: containerOptions.length > 0
			? "containers lightscale can see"
			: "no containers visible. Check that a runtime socket is configured";

	const valueField = () => {
		switch (originKind) {
			case "host":
				return null;
			case "container":
				return (
					<>
						<Field label="Container" hint={containerHint}>
							<Combobox
								value={originValue}
								onChange={setOriginValue}
								placeholder="container name"
								options={containerOptions}
							/>
						</Field>
						<Field
							label="Network"
							hint="pin selection to one network. Rarely needed"
						>
							<input
								value={originNetwork}
								onChange={(e) => setOriginNetwork(e.target.value)}
							/>
						</Field>
					</>
				);
			case "ip":
				return (
					<Field label="IP address" hint="a literal address, e.g. 192.168.1.50">
						<input
							value={originValue}
							onChange={(e) => setOriginValue(e.target.value)}
							placeholder="192.168.1.50"
						/>
					</Field>
				);
			case "hostname":
				return (
					<Field label="Hostname" hint="a DNS name the gateway can resolve">
						<input
							value={originValue}
							onChange={(e) => setOriginValue(e.target.value)}
							placeholder="nas.internal"
						/>
					</Field>
				);
		}
	};

	return (
		<FormModal
			title={editing ? `Edit ${service!.name}` : "Add service"}
			onClose={onClose}
			onSubmit={onSubmit}
			submitLabel={editing ? "Save" : "Create"}
			submitIcon={editing ? Check : Plus}
			canSubmit={!!name && (originKind === "host" || !!originValue)}
			dirty={!editing && !!(name || originValue || ports || hostname || ip || description)}
		>
			<Field label="Name">
				<input autoFocus value={name} onChange={(e) => setName(e.target.value)} />
			</Field>
			<Field
				label="Backed by"
				hint={
					originKind === "host"
						? "127.0.0.1 from lightscale's perspective; requires explicit ports below"
						: "what this service points at"
				}
			>
				<select value={originKind} onChange={(e) => changeKind(e.target.value as OriginKind)}>
					<option value="container">Container</option>
					<option value="ip">IP address</option>
					<option value="hostname">Hostname</option>
					<option value="host">This machine (host)</option>
				</select>
			</Field>
			{valueField()}
			<Field label="Ports" hint="e.g. 80,443/tcp,5353/udp. A bare port means tcp and udp">
				<input value={ports} onChange={(e) => setPorts(e.target.value)} />
			</Field>
			<Field label="Domain" hint="what users open in a browser. Derived from the name if blank">
				<input value={hostname} onChange={(e) => setHostname(e.target.value)} />
			</Field>
			<Field label="Internal IP" hint="its address on the mesh. Assigned from the service subnet if blank">
				<input value={ip} onChange={(e) => setIp(e.target.value)} />
			</Field>
			<Field label="Description" hint="optional">
				<input value={description} onChange={(e) => setDescription(e.target.value)} />
			</Field>
		</FormModal>
	);
}
