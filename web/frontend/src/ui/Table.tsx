import { ReactNode, useMemo, useRef, useState } from "react";
import { ChevronDown, ChevronUp, Columns3, Search } from "lucide-react";
import { Checkbox } from "./Checkbox";
import { useDismiss } from "./useDismiss";

export interface Column<T> {
	key: string;
	header: string;
	value?: (row: T) => string | number;
	render?: (row: T) => ReactNode;
	sortable?: boolean;
	compare?: (a: T, b: T) => number;
}

interface SortSpec {
	key: string;
	dir?: 1 | -1;
}

interface Props<T> {
	columns: Column<T>[];
	rows: T[];
	rowKey: (row: T) => string | number;
	filterPlaceholder?: string;
	empty?: ReactNode;
	toolbar?: ReactNode;
	defaultSort?: SortSpec[];
}

export function Table<T>({
	columns,
	rows,
	rowKey,
	filterPlaceholder = "Filter...",
	empty = "Nothing here yet.",
	toolbar,
	defaultSort,
}: Props<T>) {
	const [filter, setFilter] = useState("");
	const [sortKey, setSortKey] = useState<string | null>(null);
	const [sortDir, setSortDir] = useState<1 | -1>(1);
	const [hidden, setHidden] = useState<Set<string>>(new Set());
	const [colsOpen, setColsOpen] = useState(false);
	const colsRoot = useRef<HTMLDivElement>(null);

	const columnsRef = useRef(columns);
	columnsRef.current = columns;
	const defaultSortRef = useRef(defaultSort);
	defaultSortRef.current = defaultSort;

	const toggleable = columns.filter((c) => c.header.trim() !== "");
	const visibleColumns = columns.filter((c) => !hidden.has(c.key));

	useDismiss(colsRoot, colsOpen, setColsOpen);

	const toggleColumn = (key: string, show: boolean) => {
		setHidden((prev) => {
			const next = new Set(prev);
			if (show) next.delete(key);
			else next.add(key);
			return next;
		});
	};

	const valueOf = (row: T, col: Column<T>): string | number =>
		col.value ? col.value(row) : "";

	const compareBy = (key: string, dir: 1 | -1) => {
		const col = columnsRef.current.find((c) => c.key === key);
		if (!col) return () => 0;
		if (col.compare) return (a: T, b: T) => col.compare!(a, b) * dir;
		return (a: T, b: T) => {
			const va = valueOf(a, col);
			const vb = valueOf(b, col);
			if (va < vb) return -1 * dir;
			if (va > vb) return 1 * dir;
			return 0;
		};
	};

	const filtered = useMemo(() => {
		const q = filter.trim().toLowerCase();
		let out = rows;
		if (q) {
			out = rows.filter((row) =>
				columnsRef.current.some((c) => String(valueOf(row, c)).toLowerCase().includes(q)),
			);
		}
		const chain: Array<(a: T, b: T) => number> = [];
		if (sortKey) chain.push(compareBy(sortKey, sortDir));
		for (const s of defaultSortRef.current ?? []) {
			if (s.key === sortKey) continue;
			chain.push(compareBy(s.key, s.dir ?? 1));
		}
		if (chain.length) {
			out = [...out].sort((a, b) => {
				for (const cmp of chain) {
					const r = cmp(a, b);
					if (r !== 0) return r;
				}
				return 0;
			});
		}
		return out;
	}, [rows, filter, sortKey, sortDir]);

	const toggleSort = (col: Column<T>) => {
		if (!col.sortable) return;
		if (sortKey !== col.key) {
			setSortKey(col.key);
			setSortDir(1);
		} else if (sortDir === 1) {
			setSortDir(-1);
		} else {
			setSortKey(null);
			setSortDir(1);
		}
	};

	return (
		<div className="table-wrap">
			<div className="table-toolbar">
				<div className="filter-field">
					<Search size={14} className="filter-icon" />
					<input
						className="filter-input"
						placeholder={filterPlaceholder}
						value={filter}
						onChange={(e) => setFilter(e.target.value)}
					/>
				</div>
				<div className="table-meta">
					{toolbar}
					{toggleable.length > 1 && (
						<div className="col-toggle" ref={colsRoot}>
							<button
								type="button"
								className="btn btn-ghost"
								aria-haspopup="true"
								aria-expanded={colsOpen}
								onClick={() => setColsOpen((o) => !o)}
							>
								<Columns3 size={16} />
								Columns
							</button>
							{colsOpen && (
								<ul className="col-toggle-list">
									{toggleable.map((c) => (
										<li key={c.key} className="col-toggle-option">
											<Checkbox
												checked={!hidden.has(c.key)}
												onChange={(show) => toggleColumn(c.key, show)}
											>
												{c.header}
											</Checkbox>
										</li>
									))}
								</ul>
							)}
						</div>
					)}
					<span>
						{filtered.length === rows.length
							? `${rows.length} ${rows.length === 1 ? "row" : "rows"}`
							: `${filtered.length} of ${rows.length}`}
					</span>
				</div>
			</div>
			<div className="table-scroll">
				<table className="data-table">
					<thead>
						<tr>
							{visibleColumns.map((c) => (
								<th
									key={c.key}
									onClick={() => toggleSort(c)}
									className={c.sortable ? "sortable" : ""}
								>
									<span className="th-label">
										{c.header}
										{sortKey === c.key &&
											(sortDir === 1 ? (
												<ChevronUp size={14} />
											) : (
												<ChevronDown size={14} />
											))}
									</span>
								</th>
							))}
						</tr>
					</thead>
					<tbody>
						{filtered.length === 0 ? (
							<tr>
								<td className="empty-cell" colSpan={visibleColumns.length}>
									{empty}
								</td>
							</tr>
						) : (
							filtered.map((row) => (
								<tr key={rowKey(row)}>
									{visibleColumns.map((c) => (
										<td key={c.key}>{c.render ? c.render(row) : valueOf(row, c)}</td>
									))}
								</tr>
							))
						)}
					</tbody>
				</table>
			</div>
		</div>
	);
}
