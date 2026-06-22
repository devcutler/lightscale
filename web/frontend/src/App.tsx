import { useState, type MouseEvent } from "react";
import { Link, Route, Switch, useLocation } from "wouter";
import {
	Activity,
	LayoutDashboard,
	Menu,
	Moon,
	Network,
	Server,
	Settings,
	Share2,
	Shield,
	Sun,
	Users as UsersIcon,
	UsersRound,
	type LucideIcon,
} from "lucide-react";
import { useTheme } from "./ui/theme";
import { Brand } from "./ui/Brand";
import { Button } from "./ui/Button";
import { SettingsModal } from "./ui/SettingsModal";
import { LiveIndicator } from "./ui/LiveIndicator";
import { Page } from "./ui/Page";
import { Dashboard } from "./pages/Dashboard";
import { Users } from "./pages/Users";
import { Services } from "./pages/Services";
import { UserGroups } from "./pages/UserGroups";
import { ServiceGroups } from "./pages/ServiceGroups";
import { Policies } from "./pages/Policies";
import { Peers } from "./pages/Peers";
import { Connections } from "./pages/Connections";

const NAV: { href: string; label: string; icon: LucideIcon; }[] = [
	{ href: "/", label: "Dashboard", icon: LayoutDashboard },
	{ href: "/users", label: "Users", icon: UsersIcon },
	{ href: "/services", label: "Services", icon: Server },
	{ href: "/user-groups", label: "User Groups", icon: UsersRound },
	{ href: "/service-groups", label: "Service Groups", icon: Share2 },
	{ href: "/policies", label: "Policies", icon: Shield },
	{ href: "/peers", label: "Peers", icon: Network },
	{ href: "/connections", label: "Connections", icon: Activity },
];

function SidebarActions() {
	const [theme, toggle] = useTheme();
	const [settingsOpen, setSettingsOpen] = useState(false);
	const dark = theme === "dark";
	const onToggle = (e: MouseEvent<HTMLButtonElement>) => {
		const r = e.currentTarget.getBoundingClientRect();
		toggle({ x: r.left + r.width / 2, y: r.top + r.height / 2 });
	};
	return (
		<div className="sidebar-actions">
			{/* Icon reflects the theme you'd switch TO: sun in dark, moon in light. */}
			<Button
				variant="ghost"
				icon={dark ? Sun : Moon}
				onClick={onToggle}
				aria-label={dark ? "Switch to light theme" : "Switch to dark theme"}
			/>
			<Button
				variant="ghost"
				icon={Settings}
				onClick={() => setSettingsOpen(true)}
				aria-label="Settings"
			/>
			{settingsOpen && <SettingsModal onClose={() => setSettingsOpen(false)} />}
		</div>
	);
}

export function App() {
	const [loc] = useLocation();
	const [navOpen, setNavOpen] = useState(false);
	return (
		<div className="layout">
			<header className="topbar">
				<Button
					variant="ghost"
					icon={Menu}
					onClick={() => setNavOpen(true)}
					aria-label="Open navigation"
				/>
				<Brand />
			</header>
			{navOpen && (
				<div className="nav-backdrop" onClick={() => setNavOpen(false)} />
			)}
			<aside className={"sidebar" + (navOpen ? " open" : "")}>
				<Brand />
				<nav>
					{NAV.map((n) => {
						const active =
							n.href === "/"
								? loc === "/"
								: loc === n.href || loc.startsWith(n.href + "/");
						return (
							<Link
								key={n.href}
								href={n.href}
								className={"nav-link" + (active ? " active" : "")}
								onClick={() => setNavOpen(false)}
							>
								<n.icon size={17} className="nav-icon" />
								<span>{n.label}</span>
							</Link>
						);
					})}
				</nav>
				<div className="sidebar-foot">
					<LiveIndicator />
					<SidebarActions />
				</div>
			</aside>
			<main className="content">
				<Switch>
					<Route path="/" component={Dashboard} />
					<Route path="/users" component={Users} />
					<Route path="/services" component={Services} />
					<Route path="/user-groups" component={UserGroups} />
					<Route path="/service-groups" component={ServiceGroups} />
					<Route path="/policies" component={Policies} />
					<Route path="/peers" component={Peers} />
					<Route path="/connections" component={Connections} />
					<Route>
						<Page title="Not found" />
					</Route>
				</Switch>
			</main>
		</div>
	);
}
