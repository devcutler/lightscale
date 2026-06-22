import { ArrowDown, ArrowUp } from "lucide-react";
import { useData } from "../ui/useData";
import { get } from "../api";
import { Peer } from "../types";
import { Table, Column } from "../ui/Table";
import { Page } from "../ui/Page";
import { useSettings } from "../ui/settings";
import { ago, bytes, ipKey, NEVER } from "../format";

export function Peers() {
	const peers = useData<Peer[]>(() => get("/peers"), 5000);
	useSettings();

	const cols: Column<Peer>[] = [
		{ key: "name", header: "Name", value: (p) => p.name ?? "", sortable: true, render: (p) => p.name || "-" },
		{ key: "ip", header: "IP", value: (p) => p.ip_address ?? "", sortable: true, compare: (a, b) => ipKey(a.ip_address) - ipKey(b.ip_address), render: (p) => p.ip_address || "-" },
		{ key: "endpoint", header: "Endpoint", value: (p) => p.endpoint ?? "", render: (p) => p.endpoint || "-" },
		{
			key: "hs",
			header: "Last handshake",
			value: (p) => p.last_handshake_ago_sec ?? NEVER,
			sortable: true,
			render: (p) => ago(p.last_handshake_ago_sec),
		},
		{ key: "rx", header: "Rx", value: (p) => p.rx_bytes, sortable: true, render: (p) => <span className="byte-cell"><ArrowDown size={14} className="dim" />{bytes(p.rx_bytes)}</span> },
		{ key: "tx", header: "Tx", value: (p) => p.tx_bytes, sortable: true, render: (p) => <span className="byte-cell"><ArrowUp size={14} className="dim" />{bytes(p.tx_bytes)}</span> },
		{ key: "ka", header: "Keepalive", value: (p) => p.keepalive_interval, render: (p) => (p.keepalive_interval ? `${p.keepalive_interval}s` : "-") },
	];

	return (
		<Page title="Peers" error={peers.error}>
			<Table
				columns={cols}
				rows={peers.data ?? []}
				rowKey={(p) => p.public_key}
				defaultSort={[{ key: "ip" }]}
				filterPlaceholder="Filter peers..."
				empty="No peers."
			/>
		</Page>
	);
}
