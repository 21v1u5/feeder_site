import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  reactStrictMode: true,
  // Don't auto-generate AGENTS.md/CLAUDE.md on every `next dev` run.
  agentRules: false,
};

export default nextConfig;
