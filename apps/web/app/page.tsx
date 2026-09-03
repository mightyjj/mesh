"use client";

import { useAuth } from "@clerk/nextjs";
import { FormEvent, useEffect, useState } from "react";

type Content = {
	content_id: number;
	title: string;
	created_at: string;
};

type AccountState = "loading" | "ready" | "error";
type ContentState = "loading" | "ready" | "error";

export default function Home() {
	return (
		<main>
			<h1>Mesh</h1>
			{process.env.NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY ? <ContentPage /> : <SignedOut />}
		</main>
	);
}

function SignedOut() {
	return <p>Upload your video once, publish it to your accounts, and keep the results in one place.</p>;
}

function ContentPage() {
	const { getToken, isLoaded, isSignedIn } = useAuth();
	const [account, setAccount] = useState<AccountState>("loading");
	const [content, setContent] = useState<Content[]>([]);
	const [contentState, setContentState] = useState<ContentState>("loading");
	const [title, setTitle] = useState("");
	const [titleError, setTitleError] = useState(false);
	const [submitError, setSubmitError] = useState(false);
	const [submitting, setSubmitting] = useState(false);

	useEffect(() => {
		let cancelled = false;

		if (!isLoaded || !isSignedIn) return;

		setAccount("loading");
		getToken()
			.then(async (response) => {
				if (!response) throw new Error("Could not load session token");
				const result = await fetch("/api/me", {
					headers: { Authorization: `Bearer ${response}` },
				});
				if (!result.ok) throw new Error("Could not load current user");
			})
			.then(() => {
				if (!cancelled) {
					setAccount("ready");
				}
			})
			.catch(() => {
				if (!cancelled) {
					setAccount("error");
				}
			});

		return () => {
			cancelled = true;
		};
	}, [getToken, isLoaded, isSignedIn]);

	useEffect(() => {
		let cancelled = false;

		if (account !== "ready") return;

		setContentState("loading");
		getToken()
			.then(async (token) => {
				if (!token) throw new Error("Could not load session token");
				const response = await fetch("/api/content", {
					headers: { Authorization: `Bearer ${token}` },
				});
				if (!response.ok) throw new Error("Could not load content");
				return (await response.json()) as Content[];
			})
			.then((items) => {
				if (!cancelled) {
					setContent(items);
					setContentState("ready");
				}
			})
			.catch(() => {
				if (!cancelled) {
					setContentState("error");
				}
			});

		return () => {
			cancelled = true;
		};
	}, [account, getToken]);

	async function addContent(event: FormEvent<HTMLFormElement>) {
		event.preventDefault();
		if (!title.trim()) {
			setTitleError(true);
			return;
		}

		setSubmitError(false);
		setSubmitting(true);
		try {
			const token = await getToken();
			if (!token) throw new Error("Could not load session token");
			const response = await fetch("/api/content", {
				method: "POST",
				headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
				body: JSON.stringify({ title }),
			});
			if (!response.ok) throw new Error("Could not create content");
			const item = (await response.json()) as Content;
			setContent((items) => [...items, item]);
			setContentState("ready");
			setTitle("");
		} catch {
			setSubmitError(true);
		} finally {
			setSubmitting(false);
		}
	}

	if (!isLoaded || !isSignedIn) return <SignedOut />;
	if (account === "loading") return <p role="status">Setting up your Mesh account...</p>;
	if (account === "error") return <p role="alert">Could not set up your Mesh account. Reload the page to try again.</p>;

	return (
		<>
			<form onSubmit={addContent} aria-busy={submitting}>
				<label htmlFor="content-title">Title</label>
				<input
					id="content-title"
					value={title}
					onChange={(event) => {
						setTitle(event.target.value);
						setTitleError(false);
					}}
					aria-describedby={titleError ? "title-helper title-error" : "title-helper"}
					aria-invalid={titleError || undefined}
				/>
				<p id="title-helper">Just for you. This isn't the caption you publish.</p>
				{titleError && <p id="title-error" role="alert">Enter a title</p>}
				<button type="submit" disabled={submitting}>{submitting ? "Adding..." : "Add content"}</button>
				{submitError && <p role="alert">Could not add that. Try again.</p>}
			</form>
			{contentState === "loading" && <p role="status">Loading your content...</p>}
			{contentState === "error" && <p role="alert">Could not load your content. Reload the page to try again.</p>}
			{contentState === "ready" && content.length === 0 && <p>You haven't added anything yet.</p>}
			{contentState === "ready" && content.length > 0 && (
				<ul>
					{content.map((item) => (
						<li key={item.content_id}>
							{item.title} <small><time dateTime={item.created_at}>{new Date(item.created_at).toLocaleDateString()}</time></small>
						</li>
					))}
				</ul>
			)}
		</>
	);
}
