"use client";

import { useAuth } from "@clerk/nextjs";
import { useEffect, useState } from "react";

type UserState = "loading" | "signed-out" | "ready" | "error";

export default function AccountStatus() {
	const { getToken, isLoaded, isSignedIn } = useAuth();
	const [user, setUser] = useState<UserState>("loading");

	useEffect(() => {
		if (!isLoaded) return;
		if (!isSignedIn) {
			setUser("signed-out");
			return;
		}

		getToken()
			.then((token) => fetch("/api/me", { headers: { Authorization: `Bearer ${token}` } }))
			.then((response) => {
				if (!response.ok) throw new Error("Could not load current user");
				setUser("ready");
			})
			.catch(() => setUser("error"));
	}, [getToken, isLoaded, isSignedIn]);

	return (
		<section aria-labelledby="account-heading">
			<h2 id="account-heading">Account</h2>
			{user === "loading" && <p role="status">Checking account...</p>}
			{user === "signed-out" && <p>Sign in to create your Mesh account.</p>}
			{user === "ready" && <p role="status">Mesh account is ready.</p>}
			{user === "error" && <p role="alert">Could not load your Mesh account.</p>}
		</section>
	);
}
