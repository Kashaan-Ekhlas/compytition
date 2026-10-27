/** @type {import('next').NextConfig} */
const apiBase = process.env.API_INTERNAL_URL ?? "http://localhost:8080";

const nextConfig = {
  output: "standalone",
  reactStrictMode: true,
  poweredByHeader: false,
  async rewrites() {
    return [
      {
        source: "/api/v1/:path*",
        destination: `${apiBase}/api/v1/:path*`,
      },
    ];
  },
};

export default nextConfig;
