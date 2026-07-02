import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "standalone",
  async redirects() {
    return [
      {
        source: "/settings/sso",
        destination: "/settings",
        permanent: false,
      },
    ];
  },
};

export default nextConfig;
