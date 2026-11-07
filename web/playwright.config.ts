import { defineConfig, devices } from "@playwright/test";

const baseURL = "http://localhost:6030";
const serverStartTimeout = 180_000;

export default defineConfig({
  testDir: "./e2e",
  outputDir: "../test-results",
  fullyParallel: false,
  workers: 1,
  reporter: process.env.CI
    ? [
        ["list"],
        ["html", { open: "never", outputFolder: "../playwright-report" }],
      ]
    : [["list"]],
  use: { baseURL, trace: "retain-on-failure" },
  projects: [
    {
      name: "desktop",
      use: {
        ...devices["Desktop Chrome"],
        viewport: { width: 1280, height: 944 },
      },
    },
  ],
  webServer: {
    command: "../scripts/e2e-server.sh",
    url: `${baseURL}/healthz`,
    timeout: serverStartTimeout,
    // Never test against something already on port 6030 (it could be a real
    // instance): Playwright stops if the port is in use.
    reuseExistingServer: false,
  },
});
