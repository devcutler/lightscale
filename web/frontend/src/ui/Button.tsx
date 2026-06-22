import { ButtonHTMLAttributes, ReactNode } from "react";
import { LucideIcon } from "lucide-react";

type ButtonVariant =
	| "default"
	| "primary"
	| "ghost"
	| "danger"
	| "ghost-danger";

const VARIANT_CLASS: Record<ButtonVariant, string> = {
	default: "btn",
	primary: "btn btn-primary",
	ghost: "btn btn-ghost",
	danger: "btn btn-danger",
	"ghost-danger": "btn btn-ghost-danger",
};

interface Props extends ButtonHTMLAttributes<HTMLButtonElement> {
	variant?: ButtonVariant;
	icon?: LucideIcon;
	iconSize?: number;
	children?: ReactNode;
}

export function Button({
	variant = "default",
	icon: Icon,
	iconSize = 16,
	className,
	type = "button",
	children,
	...rest
}: Props) {
	const iconOnly = Icon != null && (children == null || children === false);
	const classes = [
		VARIANT_CLASS[variant],
		iconOnly ? "btn-icon-only" : "",
		className ?? "",
	]
		.filter(Boolean)
		.join(" ");

	return (
		<button type={type} className={classes} {...rest}>
			{Icon && <Icon size={iconSize} />}
			{children}
		</button>
	);
}
