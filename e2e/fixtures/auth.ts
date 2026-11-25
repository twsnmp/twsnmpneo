import { expect, type Page } from "@playwright/test";

// 1x1 transparent PNG bytes to mock external map raster tile servers cleanly offline.
const TRANSPARENT_PNG_BASE64 =
  "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII=";
const TRANSPARENT_PNG = Buffer.from(TRANSPARENT_PNG_BASE64, "base64");

const matchesDomain = (host: string, domain: string): boolean =>
  host === domain || host.endsWith(`.${domain}`);

export async function mockExternalMapTiles(page: Page): Promise<void> {
  await page.route(
    (url) => {
      const host = url.hostname;
      return (
        matchesDomain(host, "openstreetmap.org") ||
        matchesDomain(host, "gsi.go.jp") ||
        matchesDomain(host, "cartocdn.com")
      );
    },
    async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "image/png",
        body: TRANSPARENT_PNG,
      });
    }
  );
}

export interface ErrorMonitor {
  pageErrors: string[];
  consoleErrors: string[];
  http5xx: string[];
  unexpectedHttpErrors: string[];
  allowNext401Login: () => void;
  assertNoErrors: () => void;
}

export function attachErrorMonitor(page: Page): ErrorMonitor {
  const pageErrors: string[] = [];
  const consoleErrors: string[] = [];
  const http5xx: string[] = [];
  const unexpectedHttpErrors: string[] = [];
  let allowLogin401 = false;

  page.on("pageerror", (err) => {
    pageErrors.push(err.message || String(err));
  });

  page.on("console", (msg) => {
    if (msg.type() === "error") {
      const text = msg.text();
      if (text.includes("net::ERR_ABORTED")) return;
      // Chromium logs HTTP 401/403 status responses to console; we validate exact
      // HTTP endpoints via the response listener below so expected initial /api/me
      // unauthenticated probe does not trigger false console errors.
      if (text.includes("the server responded with a status of 401")) return;
      consoleErrors.push(text);
    }
  });

  page.on("response", (res) => {
    const status = res.status();
    const url = res.url();
    if (status >= 500) {
      http5xx.push(`${status} ${url}`);
    } else if (status >= 400) {
      // Initial SPA mount checks session via GET /api/me before user logs in.
      if (status === 401 && new URL(url).pathname === "/api/me") return;
      if (allowLogin401 && status === 401 && new URL(url).pathname === "/api/login") {
        allowLogin401 = false;
        return;
      }
      unexpectedHttpErrors.push(`${status} ${url}`);
    }
  });

  return {
    pageErrors,
    consoleErrors,
    http5xx,
    unexpectedHttpErrors,
    allowNext401Login: () => {
      allowLogin401 = true;
    },
    assertNoErrors: () => {
      expect(pageErrors, `Uncaught page errors:\n${pageErrors.join("\n")}`).toEqual([]);
      expect(http5xx, `HTTP 5xx responses:\n${http5xx.join("\n")}`).toEqual([]);
      expect(
        unexpectedHttpErrors,
        `Unexpected HTTP >=400 responses:\n${unexpectedHttpErrors.join("\n")}`
      ).toEqual([]);
      expect(consoleErrors, `Console errors:\n${consoleErrors.join("\n")}`).toEqual([]);
    },
  };
}

export async function loginViaUI(
  page: Page,
  options?: { user?: string; password?: string; baseURL?: string }
): Promise<void> {
  const user = options?.user ?? "twsnmp";
  const password = options?.password ?? "twsnmp";
  await page.goto(options?.baseURL ?? "/");
  await expect(page.getByTestId("login-username")).toBeVisible();
  await page.getByTestId("login-username").fill(user);
  await page.getByTestId("login-password").fill(password);
  await page.getByTestId("login-submit").click();
  await expect(page.getByTestId("nav-map")).toBeVisible({ timeout: 15_000 });
}

export async function waitForStableRender(page: Page, delayMs = 350): Promise<void> {
  await page.evaluate(async () => {
    if ("fonts" in document) {
      await document.fonts.ready;
    }
  });
  await page.waitForTimeout(delayMs);
}
