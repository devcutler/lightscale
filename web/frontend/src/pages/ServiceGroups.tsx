import { ServiceGroup } from "../types";
import { GroupManager } from "../ui/GroupManager";

export function ServiceGroups() {
	return (
		<GroupManager<ServiceGroup>
			title="Service Groups"
			basePath="/service-groups"
			membersSourcePath="/services"
			memberIdField="service_id"
			emptyText="No service groups yet."
		/>
	);
}
