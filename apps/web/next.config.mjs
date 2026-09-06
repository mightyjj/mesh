/** @type {import('next').NextConfig} */
const gatewayURL = process.env.GATEWAY_URL ?? "http://localhost:8081";

const nextConfig = {
	experimental: {
		proxyClientMaxBodySize: "110mb",
	},
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
			{
				source: "/api/content/:path*",
				destination: `${gatewayURL}/api/content/:path*`,
			},
		];
	},
};

export default nextConfig;
