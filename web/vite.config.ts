import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      // chanarr's Go API, running separately in dev (`go run ./cmd/chanarr`).
      "/api": "http://localhost:5004",
    },
  },
  build: {
    // Built straight into internal/webui's embed target — see
    // internal/webui/webui.go — so `npm run build` output is exactly what
    // go:embed picks up for the single-binary production build.
    outDir: "../internal/webui/dist",
    emptyOutDir: true,
  },
});
