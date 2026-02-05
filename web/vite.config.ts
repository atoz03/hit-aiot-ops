import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";

// 说明：
// - 构建产物输出到 web/dist
// - 静态资源目录固定为 dist/static，方便 Go 控制器用 r.Static("/static", ...) 直接托管
export default defineConfig({
  plugins: [vue()],
  build: {
    outDir: "dist",
    assetsDir: "static",
    sourcemap: true,
  },
  server: {
    host: "0.0.0.0",
    port: 5173,
  },
});

