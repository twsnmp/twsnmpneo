import { test as base, expect } from "@playwright/test";
import { startTestServer, type TestServerHandle } from "./test-server.js";
import { seedTestData, type SeededData } from "./seed-data.js";
import { attachErrorMonitor, mockExternalMapTiles, type ErrorMonitor } from "./auth.js";

type WorkerFixtures = {
  testServer: TestServerHandle;
  seededData: SeededData;
};

type TestFixtures = {
  errorMonitor: ErrorMonitor;
};

export const test = base.extend<TestFixtures, WorkerFixtures>({
  testServer: [
    async ({}, use) => {
      const server = await startTestServer({ lang: "ja" });
      try {
        await use(server);
      } finally {
        await server.stop();
      }
    },
    { scope: "worker" },
  ],

  seededData: [
    async ({ testServer }, use) => {
      const data = await seedTestData(testServer.baseURL, testServer.ports);
      await use(data);
    },
    { scope: "worker" },
  ],

  baseURL: async ({ testServer }, use) => {
    await use(testServer.baseURL);
  },

  errorMonitor: async ({ page }, use) => {
    await mockExternalMapTiles(page);
    const monitor = attachErrorMonitor(page);
    await use(monitor);
  },
});

export { expect };
