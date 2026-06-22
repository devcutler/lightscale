import { useMemo } from "react";
import { useData } from "../ui/useData";
import { get } from "../api";
import { Connection, Peer } from "../types";
import { Table, Column } from "../ui/Table";
import { Page } from "../ui/Page";
import { ago, ipKey, NEVER } from "../format";

export function Connections() {
	const conns = useData<Connection[]>(() => get("/connections"), 5000);
	const peers = useData<Peer[]>(() => get("/peers"), 5000);

	const handshakeByUser = useMemo(() => {
		const m = new Map<number, number>();
		for (const p of peers.data ?? []) {
			if (p.user_id != null && p.last_handshake_ago_sec != null) {
				m.set(p.user_id, p.last_handshake_ago_sec);
			}
		}
		return m;
	}, [peers.data]);

	const cols: Column<Connection>[] = [
		{
			key: "src", header: "Source", value: (c) => c.src_name ?? "", sortable: true,
			compare: (a, b) => ipKey(a.src_ip) - ipKey(b.src_ip),
			render: (c) => (
				<span>{c.src_name || "-"} <span className="dim">{c.src_ip}</span></span>
			)
		},
		{
			key: "obj", header: "Destination", value: (c) => c.object_name ?? "", sortable: true,
			compare: (a, b) => ipKey(a.object_ip) - ipKey(b.object_ip),
			render: (c) => (
				<span>{c.object_name || "-"} <span className="dim">{c.object_ip}</span></span>
			)
		},
		{ key: "type", header: "Type", value: (c) => c.object_type },
		{ key: "port", header: "Port", value: (c) => c.port, sortable: true, render: (c) => `${c.port}/${c.protocol}` },
		{
			key: "hs",
			header: "Last handshake",
			value: (c) => handshakeByUser.get(c.src_user_id) ?? NEVER,
			sortable: true,
			render: (c) => ago(handshakeByUser.get(c.src_user_id)),
		},
	];

	return (
		<Page title="Connections" error={conns.error}>
			<Table
				columns={cols}
				rows={conns.data ?? []}
				rowKey={(c) => c.id}
				defaultSort={[{ key: "src" }, { key: "obj" }, { key: "port" }]}
				filterPlaceholder="Filter connections..."
				empty="No active connections."
			/>
		</Page>
	);
}
