import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// The Go binary embeds this output directory (see internal/webui/embed.go).
const goEmbedDirectory = "../internal/webui/dist";

// Development: the Go server runs with APP_ORIGIN=http://localhost:5173 and
// KEEPS_DEV_LOOPBACK=1; Vite forwards API calls to it.
const developmentServer = "http://127.0.0.1:6030";

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: { "/api": developmentServer },
  },
  build: {
    outDir: goEmbedDirectory,
    emptyOutDir: true,
  },
});
