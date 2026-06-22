import { useLocation } from "wouter";
import { ArrowDown, ArrowUp, Plus } from "lucide-react";
import { Button } from "../ui/Button";
import { useData } from "../ui/useData";
import { useRate } from "../ui/useRate";
import { Page } from "../ui/Page";
import { useSettings } from "../ui/settings";
import { get } from "../api";
import { Connection, Peer } from "../types";
import { ago, NEVER, rate } from "../format";

const ONLINE_SEC = 180;

export function Dashboard() {
	const [, navigate] = useLocation();
	useSettings();
	const peers = useData<Peer[]>(() => get("/peers"), 5000);
	const conns = useData<Connection[]>(() => get("/connections"), 5000);
	const rates = useRate(peers.data ?? [], peers.updatedAt);

	const online = (peers.data ?? [])
		.filter((p) => (p.last_handshake_ago_sec ?? NEVER) < ONLINE_SEC)
		.sort((a, b) => (a.last_handshake_ago_sec ?? NEVER) - (b.last_handshake_ago_sec ?? NEVER));

	const flows = conns.data?.length ?? 0;

	return (
		<Page
			title="Dashboard"
			error={peers.error}
			actions={
				<>
					<Button icon={Plus} onClick={() => navigate("/users?new=1")}>
						Add user
					</Button>
					<Button icon={Plus} onClick={() => navigate("/services?new=1")}>
						Add service
					</Button>
				</>
			}
		>
			<div className="cards">
				<div className="card">
					<small>Download</small>
					<strong className="rate">{rate(rates.totalRx)}</strong>
					<p>current bandwidth in</p>
				</div>
				<div className="card">
					<small>Upload</small>
					<strong className="rate">{rate(rates.totalTx)}</strong>
					<p>current bandwidth out</p>
				</div>
				<div className="card">
					<small>Connections</small>
					<strong>{flows}</strong>
					<p>live flows</p>
				</div>
				<div className="card">
					<small>Connected peers</small>
					<strong>{online.length}</strong>
					<p>handshake in last 3 min</p>
				</div>
			</div>

			<h2>Connected sessions</h2>
			<div className="table-wrap">
				<table className="data-table">
					<thead>
						<tr>
							<th>Name</th>
							<th>Mesh IP</th>
							<th>Down</th>
							<th>Up</th>
							<th>Last handshake</th>
						</tr>
					</thead>
					<tbody>
						{online.length === 0 ? (
							<tr>
								<td className="empty-cell" colSpan={5}>
									No connected sessions.
								</td>
							</tr>
						) : (
							online.map((p) => {
								const r = rates.byPeer.get(p.public_key);
								return (
									<tr key={p.public_key}>
										<td>{p.name || "-"}</td>
										<td>{p.ip_address || "-"}</td>
										<td>
											<span className="byte-cell">
												<ArrowDown size={12} className="dim" />
												{rate(r?.rx ?? 0)}
											</span>
										</td>
										<td>
											<span className="byte-cell">
												<ArrowUp size={12} className="dim" />
												{rate(r?.tx ?? 0)}
											</span>
										</td>
										<td>{ago(p.last_handshake_ago_sec)}</td>
									</tr>
								);
							})
						)}
					</tbody>
				</table>
			</div>
		</Page>
	);
}
