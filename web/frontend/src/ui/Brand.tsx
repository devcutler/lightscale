import logoUrl from "../logo.svg";

export function Brand() {
	return (
		<div className="brand">
			<img src={logoUrl} alt="" />
			<span>lightscale</span>
		</div>
	);
}
