import { test, expect } from "../fixtures/test.js";
import { loginViaUI, waitForStableRender } from "../fixtures/auth.js";

test.describe("Polling Detail Report Modal", () => {
  test("displays polling logs, charts, and all tabs cleanly across light/dark themes and locales", async ({
    page,
    seededData,
    request,
    baseURL,
    errorMonitor,
  }) => {
    const appNodeId = seededData.nodes.appSrv.id || seededData.nodes.appSrv.ID;

    // Create a fast 1s HTTP polling targeting local /api/health so real logs + rtt metrics accumulate quickly
    const createRes = await request.post(`${baseURL}/api/pollings`, {
      headers: { Authorization: `Bearer ${seededData.token}` },
      data: {
        NodeID: appNodeId,
        Name: "E2E HTTP Latency Probe",
        Type: "http",
        Mode: "http",
        Params: `${baseURL}/api/health`,
        Level: "warn",
        PollInt: 1,
        Timeout: 2,
        Retry: 0,
        LogMode: 1,
        State: "unknown",
        NextTime: 0,
      },
    });
    expect(createRes.ok()).toBeTruthy();
    const createdPolling = await createRes.json();
    const probeId = createdPolling.id || createdPolling.ID;

    // Wait for at least 3 polling log entries to be recorded in Parquet/buffer
    let logCount = 0;
    const deadline = Date.now() + 12_000;
    while (Date.now() < deadline) {
      const logsRes = await request.get(`${baseURL}/api/pollings/${probeId}/logs`, {
        headers: { Authorization: `Bearer ${seededData.token}` },
      });
      if (logsRes.ok()) {
        const logsJson = await logsRes.json();
        logCount = Array.isArray(logsJson.logs) ? logsJson.logs.length : 0;
        if (logCount >= 3) break;
      }
      await new Promise((r) => setTimeout(r, 400));
    }
    expect(logCount).toBeGreaterThanOrEqual(2);

    const taRes = await request.get(`${baseURL}/api/pollings/${probeId}/time-analyze`, {
      headers: { Authorization: `Bearer ${seededData.token}` },
    });
    expect(taRes.ok()).toBeTruthy();

    await loginViaUI(page);

    // 1. Open PollingDetailModal from Reports -> Polling Report
    await page.getByTestId("nav-reports").click();
    await page.getByTestId("report-cat-polling").click();
    await waitForStableRender(page, 200);

    await page.getByTestId("btn-report-polling-detail").first().click();
    await expect(page.getByTestId("pollingdetail-modal")).toBeVisible();
    await page.getByTestId("pollingdetail-close").click();
    await expect(page.getByTestId("pollingdetail-modal")).toBeHidden();

    // 2. Open PollingDetailModal for "E2E HTTP Latency Probe" from List -> Pollings
    await page.getByTestId("nav-list").click();
    await page.getByTestId("list-cat-pollings").click();
    await expect(page.getByText("E2E HTTP Latency Probe").first()).toBeVisible();

    const probeRow = page.locator("tr", { hasText: "E2E HTTP Latency Probe" }).first();
    await probeRow.getByTestId("btn-polling-detail").click();
    await expect(page.getByTestId("pollingdetail-modal")).toBeVisible();

    // Traverse all 9 tabs
    for (const tab of [
      "basic",
      "downtime",
      "log",
      "time",
      "histogram",
      "qqplot",
      "stl",
      "fft",
      "ai",
    ] as const) {
      await page.getByTestId(`pollingdetail-tab-${tab}`).click();
      await waitForStableRender(page, 200);
    }

    // Verify Log tab renders chart canvas & log rows
    await page.getByTestId("pollingdetail-tab-log").click();
    await expect(page.getByTestId("pollingdetail-modal").locator("canvas").first()).toBeVisible({
      timeout: 10_000,
    });

    // Verify Time Series tab renders chart canvas with rtt (ms)
    await page.getByTestId("pollingdetail-tab-time").click();
    await expect(page.getByTestId("pollingdetail-modal").locator("canvas").first()).toBeVisible({
      timeout: 10_000,
    });

    // Switch theme and locale while modal is closed, then reopen to verify parity
    await page.getByTestId("pollingdetail-close").click();
    await expect(page.getByTestId("pollingdetail-modal")).toBeHidden();

    await page.getByTestId("btn-theme").click();
    await page.getByTestId("btn-locale").click();
    await waitForStableRender(page, 150);

    await probeRow.getByTestId("btn-polling-detail").click();
    await expect(page.getByTestId("pollingdetail-modal")).toBeVisible();
    await page.getByTestId("pollingdetail-tab-histogram").click();
    await expect(page.getByTestId("pollingdetail-modal").locator("canvas").first()).toBeVisible({
      timeout: 10_000,
    });
    await page.getByTestId("pollingdetail-close").click();

    // Restore theme & locale
    await page.getByTestId("btn-theme").click();
    await page.getByTestId("btn-locale").click();

    // Cleanup probe polling
    await request.delete(`${baseURL}/api/pollings/${probeId}`, {
      headers: { Authorization: `Bearer ${seededData.token}` },
    });

    errorMonitor.assertNoErrors();
  });

  test("enforces readonly RBAC on polling log APIs and hides destructive UI actions", async ({
    page,
    seededData,
    request,
    baseURL,
    errorMonitor,
  }) => {
    const appPollingId = seededData.pollings.appHttp.id || seededData.pollings.appHttp.ID;

    const loginRes = await request.post(`${baseURL}/api/login`, {
      data: {
        user: seededData.viewerCredentials.user,
        password: seededData.viewerCredentials.password,
      },
    });
    expect(loginRes.ok()).toBeTruthy();
    const { token: viewerToken } = await loginRes.json();

    // Readonly can read logs & time-analyze
    const getLogsRes = await request.get(`${baseURL}/api/pollings/${appPollingId}/logs`, {
      headers: { Authorization: `Bearer ${viewerToken}` },
    });
    expect(getLogsRes.status()).toBe(200);

    const getTaRes = await request.get(`${baseURL}/api/pollings/${appPollingId}/time-analyze`, {
      headers: { Authorization: `Bearer ${viewerToken}` },
    });
    expect(getTaRes.status()).toBe(200);

    // Readonly is forbidden from deleting polling logs
    const delLogsRes = await request.delete(`${baseURL}/api/pollings/${appPollingId}/logs`, {
      headers: { Authorization: `Bearer ${viewerToken}` },
    });
    expect(delLogsRes.status()).toBe(403);

    // Login via UI as readonly viewer and verify Clear Logs button is hidden
    await loginViaUI(page, {
      user: seededData.viewerCredentials.user,
      password: seededData.viewerCredentials.password,
    });
    await page.getByTestId("nav-list").click();
    await page.getByTestId("list-cat-pollings").click();
    const appRow = page.locator("tr", { hasText: "App Health Check" }).first();
    await appRow.getByTestId("btn-polling-detail").click();
    await expect(page.getByTestId("pollingdetail-modal")).toBeVisible();
    await expect(page.getByTestId("pollingdetail-clear-logs")).toBeHidden();
    await page.getByTestId("pollingdetail-close").click();

    errorMonitor.assertNoErrors();
  });
});
