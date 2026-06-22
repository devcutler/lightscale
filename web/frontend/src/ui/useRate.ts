import { useEffect, useMemo, useRef } from "react";
import { Peer } from "../types";

interface PeerRate {
	rx: number;
	tx: number;
}

export interface Rates {
	byPeer: Map<string, PeerRate>;
	totalRx: number;
	totalTx: number;
}

export function useRate(peers: Peer[], updatedAt: number | null): Rates {
	const prev = useRef<{ at: number; rx: Map<string, number>; tx: Map<string, number>; } | null>(null);

	const rates = useMemo(() => {
		const byPeer = new Map<string, PeerRate>();
		let totalRx = 0;
		let totalTx = 0;

		const p = prev.current;
		const dt = p && updatedAt ? (updatedAt - p.at) / 1000 : 0;

		for (const peer of peers) {
			if (dt > 0) {
				const rx = Math.max(0, peer.rx_bytes - (p!.rx.get(peer.public_key) ?? peer.rx_bytes)) / dt;
				const tx = Math.max(0, peer.tx_bytes - (p!.tx.get(peer.public_key) ?? peer.tx_bytes)) / dt;
				byPeer.set(peer.public_key, { rx, tx });
				totalRx += rx;
				totalTx += tx;
			} else {
				byPeer.set(peer.public_key, { rx: 0, tx: 0 });
			}
		}

		return { byPeer, totalRx, totalTx };
	}, [peers, updatedAt]);

	useEffect(() => {
		if (!updatedAt) return;
		prev.current = {
			at: updatedAt,
			rx: new Map(peers.map((peer) => [peer.public_key, peer.rx_bytes])),
			tx: new Map(peers.map((peer) => [peer.public_key, peer.tx_bytes])),
		};
	}, [peers, updatedAt]);

	return rates;
}
