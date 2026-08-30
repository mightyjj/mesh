"use client";

import { useEffect, useState } from "react";

type HealthState = "loading" | "healthy" | "error";

export default function Home() {
	const [health, setHealth] = useState<HealthState>("loading");

	useEffect(() => {
		let cancelled = false;

		fetch("/api/health", { cache: "no-store" })
			.then(async (response) => {
				if (!response.ok) {
					throw new Error("API health check failed");
				}

				const result = (await response.json()) as { status?: string };
				if (result.status !== "ok") {
					throw new Error("API is not healthy");
				}
			})
			.then(() => {
				if (!cancelled) {
					setHealth("healthy");
				}
			})
			.catch(() => {
				if (!cancelled) {
					setHealth("error");
				}
			});

		return () => {
			cancelled = true;
		};
	}, []);

	return (
		<main>
			<h1>Mesh</h1>
			<section aria-labelledby="api-health-heading">
				<h2 id="api-health-heading">API health</h2>
				{health === "loading" && <p role="status">Checking API health...</p>}
				{health === "healthy" && <p role="status">API is healthy.</p>}
				{health === "error" && <p role="alert">API health check failed.</p>}
			</section>
		</main>
	);
}
