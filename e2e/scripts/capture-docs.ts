import fs from "node:fs";
import path from "node:path";
import { chromium, type Page } from "@playwright/test";
import { E2E_DIR, REPO_ROOT, startTestServer } from "../fixtures/test-server.js";
import { loginViaUI, mockExternalMapTiles, waitForStableRender } from "../fixtures/auth.js";
import { seedTestData } from "../fixtures/seed-data.js";

function parseArg(name: string, fallback?: string): string | undefined {
  const prefix = `--${name}=`;
  const hit = process.argv.find((a) => a.startsWith(prefix));
  return hit ? hit.slice(prefix.length) : fallback;
}

interface CaptureJob {
  name: string;
  run: (page: Page, publicKey: string) => Promise<void>;
}

const JOBS: CaptureJob[] = [
  {
    name: "map_overview",
    run: async (page) => {
      await page.getByTestId("nav-map").click();
      await waitForStableRender(page, 600);
    },
  },
  {
    name: "location_map",
    run: async (page) => {
      await page.getByTestId("nav-location").click();
      await waitForStableRender(page, 800);
    },
  },
  {
    name: "node_list",
    run: async (page) => {
      await page.getByTestId("nav-list").click();
      await page.getByTestId("list-cat-nodes").click();
      await waitForStableRender(page, 500);
    },
  },
  {
    name: "node_detail",
    run: async (page) => {
      await page.getByTestId("nav-list").click();
      await page.getByTestId("list-cat-nodes").click();
      await waitForStableRender(page, 400);
      await page.getByTestId("btn-node-detail").first().click();
      await page.getByTestId("nodedetail-close").waitFor({ state: "visible" });
      await waitForStableRender(page, 500);
    },
  },
  {
    name: "map_vpanel_3d",
    run: async (page) => {
      await page.getByTestId("nav-list").click();
      await page.getByTestId("list-cat-nodes").click();
      await waitForStableRender(page, 300);
      await page.getByTestId("btn-node-detail").first().click();
      await page.getByTestId("nodedetail-close").waitFor({ state: "visible" });
      const vpanelTab = page.getByTestId("nodedetail-tab-vpanel");
      if (await vpanelTab.isVisible()) {
        await vpanelTab.click();
        await waitForStableRender(page, 900);
      }
    },
  },
  {
    name: "report_device",
    run: async (page) => {
      await page.getByTestId("nav-reports").click();
      await page.getByTestId("report-cat-device").click();
      await waitForStableRender(page, 700);
    },
  },
  {
    name: "report_opentelemetry",
    run: async (page) => {
      await page.getByTestId("nav-reports").click();
      await page.getByTestId("report-cat-otel").click();
      await waitForStableRender(page, 700);
    },
  },
  {
    name: "log_event",
    run: async (page) => {
      await page.getByTestId("nav-logs").click();
      await page.getByTestId("log-cat-event").click();
      await waitForStableRender(page, 600);
    },
  },
  {
    name: "log_syslog",
    run: async (page) => {
      await page.getByTestId("nav-logs").click();
      await page.getByTestId("log-cat-syslog").click();
      await waitForStableRender(page, 600);
    },
  },
  {
    name: "system_monitor",
    run: async (page) => {
      await page.getByTestId("nav-system").click();
      await page.getByTestId("system-tab-monitor").click();
      await waitForStableRender(page, 700);
    },
  },
  {
    name: "settings_map",
    run: async (page) => {
      await page.getByTestId("btn-config").click();
      await page.getByTestId("config-tab-map").click();
      await waitForStableRender(page, 500);
    },
  },
  {
    name: "settings_dashboard",
    run: async (page) => {
      await page.getByTestId("btn-config").click();
      await page.getByTestId("config-tab-dashboard").click();
      await waitForStableRender(page, 500);
    },
  },
  {
    name: "public_dashboard",
    run: async (page, publicKey) => {
      await page.goto(`/dashboard/${publicKey}`);
      await waitForStableRender(page, 800);
    },
  },
  {
    name: "public_dashboard_drama",
    run: async (page, publicKey) => {
      await page.goto(`/dashboard/${publicKey}`);
      await waitForStableRender(page, 600);
      await page.getByTestId("public-test-alert").click();
      await page.getByTestId("public-drama-dismiss").waitFor({ state: "visible" });
      await waitForStableRender(page, 500);
    },
  },
];

async function closeAnyModal(page: Page) {
  const closeDetail = page.getByTestId("nodedetail-close");
  if (await closeDetail.isVisible().catch(() => false)) {
    await closeDetail.click();
    await waitForStableRender(page, 150);
  }
  const closeConfig = page.getByTestId("config-close");
  if (await closeConfig.isVisible().catch(() => false)) {
    await closeConfig.click();
    await waitForStableRender(page, 150);
  }
}

async function main() {
  const localesArg = parseArg("locales", process.env.DOCS_LOCALES || "ja,en")!;
  const locales = localesArg
    .split(",")
    .map((l) => l.trim())
    .filter((l): l is "ja" | "en" => l === "ja" || l === "en");
  const targetsArg = parseArg("targets", process.env.DOCS_TARGETS || "");
  const filterSet = targetsArg
    ? new Set(
        targetsArg
          .split(",")
          .map((t) => t.trim())
          .filter(Boolean)
      )
    : null;

  const writeDocs = process.argv.includes("--write-docs") || process.env.DOCS_WRITE === "1";
  const outRoot = writeDocs
    ? path.join(REPO_ROOT, "docs", "images")
    : path.resolve(E2E_DIR, parseArg("out", process.env.DOCS_OUT_DIR || "output/docs-preview")!);

  const server = await startTestServer({ lang: "ja" });
  try {
    const seeded = await seedTestData(server.baseURL, server.ports);
    const browser = await chromium.launch({
      headless: true,
      args: [
        "--use-gl=angle",
        "--use-angle=swiftshader",
        "--enable-webgl",
        "--ignore-gpu-blocklist",
      ],
    });

    try {
      for (const locale of locales) {
        const localeDir = path.join(outRoot, locale);
        fs.mkdirSync(localeDir, { recursive: true });

        const context = await browser.newContext({
          baseURL: server.baseURL,
          locale: locale === "en" ? "en-US" : "ja-JP",
          viewport: { width: 1440, height: 900 },
          deviceScaleFactor: 2,
        });
        const page = await context.newPage();
        await mockExternalMapTiles(page);

        // Ensure localStorage matches target locale before loading
        await page.addInitScript((loc) => {
          localStorage.setItem("twsnmp_locale", loc);
        }, locale);

        // Login screenshot if selected
        if (!filterSet || filterSet.has("login")) {
          await page.goto("/");
          await waitForStableRender(page, 500);
          const outFile = path.join(localeDir, "login.png");
          await page.screenshot({ path: outFile });
          console.log(`[capture-docs] Captured ${outFile}`);
        }

        await loginViaUI(page);
        // Re-confirm locale matches target
        const currentLocale = await page.evaluate(
          () => localStorage.getItem("twsnmp_locale") || "ja"
        );
        if (currentLocale !== locale) {
          await page.getByTestId("btn-locale").click();
          await waitForStableRender(page, 300);
        }

        for (const job of JOBS) {
          if (filterSet && !filterSet.has(job.name)) continue;

          // If previous job navigated to public dashboard, return cleanly to main SPA
          if (!page.url().endsWith("/") && !(await page.getByTestId("nav-map").isVisible().catch(() => false))) {
            await page.goto("/");
            await page.getByTestId("nav-map").waitFor({ state: "visible", timeout: 15_000 });
          }
          await closeAnyModal(page);
          await job.run(page, seeded.publicKey);
          const outFile = path.join(localeDir, `${job.name}.png`);
          await page.screenshot({ path: outFile });
          console.log(`[capture-docs] Captured ${outFile}`);
        }

        await context.close();
      }
    } finally {
      await browser.close();
    }
  } finally {
    await server.stop();
  }
}

main().catch((err) => {
  console.error("[capture-docs] Failed:", err);
  process.exit(1);
});
