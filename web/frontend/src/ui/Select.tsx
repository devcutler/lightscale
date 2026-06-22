import { useRef, useState } from "react";
import { Check, ChevronDown } from "lucide-react";
import { useDismiss } from "./useDismiss";

interface Option {
	value: string;
	label: string;
}

interface Props {
	value: string;
	onChange: (value: string) => void;
	options: Option[];
	placeholder?: string;
	disabled?: boolean;
}

export function Select({ value, onChange, options, placeholder = "Select...", disabled }: Props) {
	const [open, setOpen] = useState(false);
	const [active, setActive] = useState(0);
	const root = useRef<HTMLDivElement>(null);

	const selected = options.find((o) => o.value === value);

	useDismiss(root, open, setOpen);

	const pick = (v: string) => {
		onChange(v);
		setOpen(false);
	};

	const onKey = (e: React.KeyboardEvent) => {
		if (disabled) return;
		if (e.key === "Escape") {
			setOpen(false);
			return;
		}
		if (!open && (e.key === "ArrowDown" || e.key === "Enter" || e.key === " ")) {
			e.preventDefault();
			setOpen(true);
			setActive(Math.max(0, options.findIndex((o) => o.value === value)));
			return;
		}
		if (!open) return;
		if (e.key === "ArrowDown") {
			e.preventDefault();
			setActive((i) => Math.min(options.length - 1, i + 1));
		} else if (e.key === "ArrowUp") {
			e.preventDefault();
			setActive((i) => Math.max(0, i - 1));
		} else if (e.key === "Enter") {
			e.preventDefault();
			if (options[active]) pick(options[active].value);
		}
	};

	return (
		<div className="select" ref={root}>
			<button
				type="button"
				className={"select-btn" + (selected ? "" : " select-placeholder")}
				disabled={disabled}
				aria-haspopup="listbox"
				aria-expanded={open}
				onClick={() => setOpen((o) => !o)}
				onKeyDown={onKey}
			>
				<span>{selected ? selected.label : placeholder}</span>
				<ChevronDown size={15} className="dim" />
			</button>
			{open && (
				<ul className="select-list" role="listbox">
					{options.map((o, i) => (
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
							<span>{o.label}</span>
							{o.value === value && <Check size={14} />}
						</li>
					))}
				</ul>
			)}
		</div>
	);
}
