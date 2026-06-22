import { ReactNode } from "react";
import { CheckSquare, Square } from "lucide-react";

interface Props {
	checked: boolean;
	onChange: (checked: boolean) => void;
	disabled?: boolean;
	children?: ReactNode;
}

export function Checkbox({ checked, onChange, disabled, children }: Props) {
	return (
		<label className={"toggle" + (disabled ? " toggle-disabled" : "")}>
			<input
				type="checkbox"
				checked={checked}
				disabled={disabled}
				onChange={(e) => onChange(e.target.checked)}
			/>
			{checked ? <CheckSquare size={16} /> : <Square size={16} />}
			{children}
		</label>
	);
}
