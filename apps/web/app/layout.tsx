import type { Metadata } from "next";
import {
	ClerkProvider,
	Show,
	SignInButton,
	SignUpButton,
	UserButton,
} from "@clerk/nextjs";
import type { ReactNode } from "react";

export const metadata: Metadata = {
	title: "Mesh",
	description: "Creator intelligence platform",
};

export default function RootLayout({ children }: { children: ReactNode }) {
	const publishableKey = process.env.NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY;
	const content = (
		<>
			{publishableKey && (
				<header>
					<Show when="signed-out">
						<SignInButton mode="modal">
							<button type="button">Sign in</button>
						</SignInButton>
						<SignUpButton mode="modal">
							<button type="button">Sign up</button>
						</SignUpButton>
					</Show>
					<Show when="signed-in">
						<UserButton />
					</Show>
				</header>
			)}
			{children}
		</>
	);

	return (
		<html lang="en">
			<body>
				{publishableKey ? (
					<ClerkProvider publishableKey={publishableKey}>{content}</ClerkProvider>
				) : (
					content
				)}
			</body>
		</html>
	);
}
