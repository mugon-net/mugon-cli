import { resolve } from "path";
import { defineConfig } from "vite";

export default defineConfig({
  define: {
    "process.env.NODE_ENV": JSON.stringify("production"),
  },
  publicDir: resolve(__dirname, "public"),
  build: {
    outDir: resolve(__dirname, "out"),
    emptyOutDir: true,
    lib: {
      entry: resolve(__dirname, "src/main.tsx"),
      formats: ["iife"],
      name: "MugonParentframe",
      fileName: () => "index.js",
    },
  },
});
