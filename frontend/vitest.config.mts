import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    // The API client only depends on fetch, which Node provides globally.
    environment: "node",
    include: ["**/*.test.ts"],
    exclude: ["node_modules/**", ".next/**"],
    // Undo vi.fn/vi.spyOn, vi.stubGlobal and vi.stubEnv after each test.
    restoreMocks: true,
    unstubGlobals: true,
    unstubEnvs: true,
  },
});
