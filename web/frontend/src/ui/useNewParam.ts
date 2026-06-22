import { useEffect, useRef } from "react";
import { useLocation, useSearch } from "wouter";

export function useNewParam(open: () => void) {
	const search = useSearch();
	const [loc, navigate] = useLocation();

	const openRef = useRef(open);
	openRef.current = open;

	useEffect(() => {
		const params = new URLSearchParams(search);
		if (params.get("new") === "1") {
			openRef.current();
			navigate(loc, { replace: true });
		}

	}, [search]);
}
