/** @type {import('next').NextConfig} */
const nextConfig = {
	async rewrites() {
		return [
			{
				source: "/api/health",
				destination: "http://localhost:8081/api/health",
			},
		];
	},
};

export default nextConfig;
