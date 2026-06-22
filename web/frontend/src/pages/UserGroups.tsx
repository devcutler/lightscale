import { useState } from "react";
import { errMsg, patch } from "../api";
import { UserGroup } from "../types";
import { GroupManager } from "../ui/GroupManager";
import { Checkbox } from "../ui/Checkbox";
import { useToast } from "../ui/toast";

export function UserGroups() {
	return (
		<GroupManager<UserGroup>
			title="User Groups"
			basePath="/user-groups"
			membersSourcePath="/users"
			memberIdField="user_id"
			emptyText="No user groups yet."
			badge={(g) => (g.lan_mode ? <span className="badge">LAN</span> : null)}
			extraControls={(g, reload) => <LanToggle group={g} onChanged={reload} />}
			createExtras={(extra, setExtra) => (
				<Checkbox
					checked={!!extra.lan_mode}
					onChange={(c) => setExtra({ lan_mode: c })}
				>
					LAN mode
				</Checkbox>
			)}
			createBody={(name, extra) => ({ name, lan_mode: !!extra.lan_mode })}
		/>
	);
}

function LanToggle({
	group,
	onChanged,
}: {
	group: UserGroup;
	onChanged: () => void;
}) {
	const toast = useToast();
	const [busy, setBusy] = useState(false);

	const toggle = async () => {
		const next = !group.lan_mode;
		setBusy(true);
		try {
			await patch(`/user-groups/${group.id}`, { lan_mode: next });
			toast.ok(`LAN mode ${next ? "enabled" : "disabled"} for ${group.name}`);
			onChanged();
		} catch (e) {
			toast.err(errMsg(e));
		} finally {
			setBusy(false);
		}
	};

	return (
		<Checkbox checked={group.lan_mode} disabled={busy} onChange={toggle}>
			LAN mode
		</Checkbox>
	);
}
