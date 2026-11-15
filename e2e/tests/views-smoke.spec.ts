import { test, expect } from "../fixtures/test.js";
import { loginViaUI, waitForStableRender } from "../fixtures/auth.js";

test.describe("Core Views & Modals Smoke", () => {
  test("navigates all 9 views, NodeDetailModal 3D vpanel, and ConfigModal without errors", async ({
    page,
    seededData,
    errorMonitor,
  }) => {
    void seededData;
    await loginViaUI(page);

    // 1. Map view + p5 canvas
    const mapCanvas = page.locator("#p5-map-canvas canvas");
    await expect(mapCanvas).toBeVisible();
    const mapBox = await mapCanvas.boundingBox();
    expect(mapBox && mapBox.width > 100 && mapBox.height > 100).toBeTruthy();

    // 2. Location view (MapLibre with offline mocked tiles)
    await page.getByTestId("nav-location").click();
    await waitForStableRender(page);
    await expect(page.locator("canvas").first()).toBeVisible({ timeout: 10_000 });

    // 3. List view + node search + NodeDetailModal (including 3D WebGL vpanel)
    await page.getByTestId("nav-list").click();
    await expect(page.getByTestId("list-cat-nodes")).toBeVisible();
    await expect(page.getByText("Core Gateway").first()).toBeVisible();

    const searchInput = page.getByTestId("node-search");
    await searchInput.fill("Core Gateway");
    await expect(page.getByText("Core Gateway").first()).toBeVisible();

    await page.getByTestId("btn-node-detail").first().click();
    await expect(page.getByTestId("nodedetail-close")).toBeVisible();

    const vpanelTab = page.getByTestId("nodedetail-tab-vpanel");
    if (await vpanelTab.isVisible()) {
      await vpanelTab.click();
      const vpanelCanvas = page.locator("#vpanel-canvas-container canvas");
      await expect(vpanelCanvas).toBeVisible({ timeout: 10_000 });
      const vpBox = await vpanelCanvas.boundingBox();
      expect(vpBox && vpBox.width > 50 && vpBox.height > 50).toBeTruthy();
    }
    await page.getByTestId("nodedetail-close").click();
    await expect(page.getByTestId("nodedetail-close")).toBeHidden();
    await searchInput.fill("");

    for (const cat of ["pollings", "networks", "lines", "drawitems", "nodes"] as const) {
      await page.getByTestId(`list-cat-${cat}`).click();
    }

    // 4. Reports view
    await page.getByTestId("nav-reports").click();
    for (const rep of ["device", "ipam", "polling", "event", "syslog", "otel"] as const) {
      await page.getByTestId(`report-cat-${rep}`).click();
      await waitForStableRender(page, 150);
    }

    // 5. PKI view
    await page.getByTestId("nav-pki").click();
    await expect(page.getByTestId("pki-tab-csr")).toBeVisible();
    await page.getByTestId("pki-tab-csr").click();

    // 6. Logs view
    await page.getByTestId("nav-logs").click();
    await expect(page.getByTestId("log-cat-event")).toBeVisible();
    await page.getByTestId("log-cat-syslog").click();
    await waitForStableRender(page, 200);

    // 7. OTel view
    await page.getByTestId("nav-otel").click();
    await expect(page.getByTestId("otel-tab-metric")).toBeVisible();
    await page.getByTestId("otel-tab-trace").click();
    await page.getByTestId("otel-tab-log").click();
    await page.getByTestId("otel-tab-metric").click();

    // 8. MQTT view
    await page.getByTestId("nav-mqtt").click();
    await expect(page.getByTestId("mqtt-tab-stats")).toBeVisible();
    await page.getByTestId("mqtt-tab-logs").click();

    // 9. System view
    await page.getByTestId("nav-system").click();
    await expect(page.getByTestId("system-tab-status")).toBeVisible();
    await page.getByTestId("system-tab-monitor").click();

    // 10. ConfigModal tabs
    await page.getByTestId("btn-config").click();
    await expect(page.getByTestId("config-modal")).toBeVisible();
    for (const tab of [
      "map",
      "loc",
      "dashboard",
      "polling",
      "receivers",
      "notify",
      "ai",
      "database",
      "mib",
      "icons",
      "users",
    ] as const) {
      await page.getByTestId(`config-tab-${tab}`).click();
      await waitForStableRender(page, 100);
    }
    await page.getByTestId("config-close").click();
    await expect(page.getByTestId("config-modal")).toBeHidden();

    errorMonitor.assertNoErrors();
  });
});
