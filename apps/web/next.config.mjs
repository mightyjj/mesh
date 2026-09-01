/** @type {import('next').NextConfig} */
const gatewayURL = process.env.GATEWAY_URL ?? "http://localhost:8081";

const nextConfig = {
	async rewrites() {
		return [
			{
				source: "/api/health",
				destination: `${gatewayURL}/api/health`,
			},
			{
				source: "/api/me",
				destination: `${gatewayURL}/api/me`,
			},
			{
				source: "/api/content",
				destination: `${gatewayURL}/api/content`,
			},
		];
	},
};

export default nextConfig;
