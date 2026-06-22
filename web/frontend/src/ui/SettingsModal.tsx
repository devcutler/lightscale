import { Modal } from "./Modal";
import { Select } from "./Select";
import { Field } from "./Field";
import { DataFormat, setSetting, useSettings } from "./settings";

export function SettingsModal({ onClose }: { onClose: () => void; }) {
	const settings = useSettings();

	return (
		<Modal title="Settings" onClose={onClose}>
			<Field
				label="Data display format"
				hint="Binary uses 1024-based units (GiB, MiB); decimal uses 1000-based units (GB, MB)."
			>
				<Select
					value={settings.dataFormat}
					onChange={(v) => setSetting("dataFormat", v as DataFormat)}
					options={[
						{ value: "binary", label: "Binary - GiB, MiB (1024)" },
						{ value: "decimal", label: "Decimal - GB, MB (1000)" },
					]}
				/>
			</Field>
		</Modal>
	);
}
