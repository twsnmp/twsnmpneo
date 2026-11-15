import { test, expect } from "../fixtures/test.js";
import { loginViaUI, waitForStableRender } from "../fixtures/auth.js";

test.describe("Public Dashboard", () => {
  test("renders public dashboard, toggles locale, triggers and dismisses drama alert", async ({
    page,
    seededData,
    errorMonitor,
  }) => {
    await page.goto(`/dashboard/${seededData.publicKey}`);
    await waitForStableRender(page, 500);

    await expect(page.getByText("TWSNMP NEO E2E Lab")).toBeVisible();
    await expect(page.getByTestId("public-locale")).toBeVisible();

    const localeBefore = await page.getByTestId("public-locale").innerText();
    await page.getByTestId("public-locale").click();
    const localeAfter = await page.getByTestId("public-locale").innerText();
    expect(localeAfter).not.toEqual(localeBefore);

    await page.getByTestId("public-refresh").click();

    // Trigger and dismiss Drama Alert overlay
    await page.getByTestId("public-test-alert").click();
    await expect(page.getByTestId("public-drama-dismiss")).toBeVisible({ timeout: 5_000 });
    await page.getByTestId("public-drama-dismiss").click();
    await expect(page.getByTestId("public-drama-dismiss")).toBeHidden();

    // Embed mode hides interactive controls such as test alert button
    await page.goto(`/dashboard/${seededData.publicKey}?embed=true`);
    await waitForStableRender(page, 400);
    await expect(page.getByText("TWSNMP NEO E2E Lab")).toBeVisible();
    await expect(page.getByTestId("public-test-alert")).toBeHidden();

    errorMonitor.assertNoErrors();
  });
});
