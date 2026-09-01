import { useRef, useState } from "react";
import { useDismiss } from "./useDismiss";
import { Popover } from "./Popover";

export type ComboOption =
	| { value: string; label: string; }
	| { separator: true; };

interface Props {
	value: string;
	onChange: (value: string) => void;
	options: ComboOption[];
	placeholder?: string;
	autoFocus?: boolean;
}

function isSep(o: ComboOption): o is { separator: true; } {
	return "separator" in o;
}

export function Combobox({ value, onChange, options, placeholder, autoFocus }: Props) {
	const [open, setOpen] = useState(false);
	const [active, setActive] = useState(0);
	const root = useRef<HTMLDivElement>(null);

	const q = value.toLowerCase();
	const filtered = q
		? options.filter((o) => !isSep(o) && (o.value.toLowerCase().includes(q) || o.label.toLowerCase().includes(q)))
		: options;
	const matches = filtered.filter((o, i) => {
		if (!isSep(o)) return true;
		return filtered.slice(0, i).some((p) => !isSep(p)) && filtered.slice(i + 1).some((p) => !isSep(p));
	});

	const step = (from: number, dir: 1 | -1): number => {
		for (let i = from + dir; i >= 0 && i < matches.length; i += dir) {
			if (!isSep(matches[i])) return i;
		}
		return from;
	};

	useDismiss(root, open, setOpen);

	const pick = (v: string) => {
		onChange(v);
		setOpen(false);
	};

	const onKey = (e: React.KeyboardEvent) => {
		if (e.key === "Escape") {
			setOpen(false);
		} else if (e.key === "ArrowDown") {
			e.preventDefault();
			setOpen(true);
			setActive((i) => step(i, 1));
		} else if (e.key === "ArrowUp") {
			e.preventDefault();
			setActive((i) => step(i, -1));
		} else if (e.key === "Enter" && open) {
			const o = matches[active];
			if (o && !isSep(o)) {
				e.preventDefault();
				pick(o.value);
			}
		}
	};

	return (
		<div className="select" ref={root}>
			<input
				autoFocus={autoFocus}
				placeholder={placeholder}
				value={value}
				onChange={(e) => {
					onChange(e.target.value);
					setOpen(true);
					setActive(0);
				}}
				onFocus={() => setOpen(true)}
				onKeyDown={onKey}
			/>
			<Popover anchor={root} open={open && matches.length > 0}>
				<ul className="select-list" role="listbox">
					{matches.map((o, i) =>
						isSep(o) ? (
							<li key={`sep-${i}`} className="select-sep" role="separator" />
						) : (
							<li
								key={o.value}
								role="option"
								aria-selected={o.value === value}
								className={"select-option" + (i === active ? " select-active" : "")}
								onMouseEnter={() => setActive(i)}
								onMouseDown={(e) => {
									e.preventDefault();
									pick(o.value);
								}}
							>
								{o.label}
							</li>
						),
					)}
				</ul>
			</Popover>
		</div>
	);
}
