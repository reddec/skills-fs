import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// Build output lands where the Go binary embeds it; dev proxies API + /fs to the Go server.
export default defineConfig({
  plugins: [react()],
  build: {
    outDir: "../internal/web/dist",
    emptyOutDir: true,
  },
  server: {
    proxy: {
      "/api": "http://localhost:8080",
      "/fs": "http://localhost:8080",
    },
  },
});
