import { test, expect } from "../fixtures/test.js";
import { loginViaUI } from "../fixtures/auth.js";

test.describe("Authentication & RBAC", () => {
  test("rejects invalid login credentials and shows error alert", async ({ page, errorMonitor }) => {
    errorMonitor.allowNext401Login();
    await page.goto("/");
    await expect(page.getByTestId("login-username")).toBeVisible();
    await page.getByTestId("login-username").fill("twsnmp");
    await page.getByTestId("login-password").fill("wrong-password-123");
    await page.getByTestId("login-submit").click();

    await expect(page.getByTestId("login-error")).toBeVisible();
    await expect(page.getByTestId("nav-map")).toBeHidden();
    expect(errorMonitor.pageErrors).toEqual([]);
    expect(errorMonitor.http5xx).toEqual([]);
  });

  test("logs in as admin and logs out cleanly", async ({ page, errorMonitor }) => {
    await loginViaUI(page);
    await expect(page.getByTestId("user-badge")).toContainText("admin");

    await page.getByTestId("btn-logout").click();
    await expect(page.getByTestId("login-username")).toBeVisible();
    errorMonitor.assertNoErrors();
  });

  test("supports URL parameter auto-login with view deep-link and strips credentials", async ({
    page,
    errorMonitor,
  }) => {
    await page.goto("/?user=twsnmp&password=twsnmp&view=list");
    await expect(page.getByTestId("list-cat-nodes")).toBeVisible({ timeout: 15_000 });

    const searchParams = await page.evaluate(() => window.location.search);
    expect(searchParams).not.toContain("password=");
    expect(searchParams).not.toContain("user=");
    errorMonitor.assertNoErrors();
  });

  test("enforces readonly role restrictions on mutating API calls", async ({
    page,
    seededData,
    request,
    baseURL,
    errorMonitor,
  }) => {
    await loginViaUI(page, {
      user: seededData.viewerCredentials.user,
      password: seededData.viewerCredentials.password,
    });
    await expect(page.getByTestId("user-badge")).toContainText("readonly");

    const loginRes = await request.post(`${baseURL}/api/login`, {
      data: {
        user: seededData.viewerCredentials.user,
        password: seededData.viewerCredentials.password,
      },
    });
    expect(loginRes.ok()).toBeTruthy();
    const { token: viewerToken } = await loginRes.json();

    const readRes = await request.get(`${baseURL}/api/nodes`, {
      headers: { Authorization: `Bearer ${viewerToken}` },
    });
    expect(readRes.status()).toBe(200);

    const mutateRes = await request.post(`${baseURL}/api/nodes`, {
      headers: { Authorization: `Bearer ${viewerToken}` },
      data: {
        Name: "Forbidden Node",
        IP: "192.0.2.99",
        X: 100,
        Y: 100,
      },
    });
    expect(mutateRes.status()).toBe(403);

    errorMonitor.assertNoErrors();
  });
});
