import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Standalone output is for the Docker image (frontend/Dockerfile copies
  // .next/standalone and runs server.js). Vercel does not want it: it has
  // its own Next.js adapter, and on Next 16.3.0 the combination breaks the
  // build outright -- Vercel's onBuildComplete step looks for
  // .next/next-server.js.nft.json, which standalone mode no longer emits
  // (vercel/next.js#96646). VERCEL is set on every Vercel build.
  output: process.env.VERCEL ? undefined : "standalone",
};

export default nextConfig;
