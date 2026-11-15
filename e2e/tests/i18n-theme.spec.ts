import { test, expect } from "../fixtures/test.js";
import { loginViaUI } from "../fixtures/auth.js";

test.describe("i18n & Theme Persistence", () => {
  test("switches locale between JA/EN and theme between dark/light across reloads", async ({
    page,
    seededData,
    errorMonitor,
  }) => {
    void seededData;
    await loginViaUI(page);

    // Theme toggle persistence
    const initialDark = await page.evaluate(() =>
      document.documentElement.classList.contains("dark")
    );
    await page.getByTestId("btn-theme").click();
    const toggledDark = await page.evaluate(() =>
      document.documentElement.classList.contains("dark")
    );
    expect(toggledDark).toBe(!initialDark);

    // Locale toggle persistence
    const initialLocale = await page.evaluate(() => localStorage.getItem("twsnmp_locale") || "ja");
    await page.getByTestId("btn-locale").click();
    const toggledLocale = await page.evaluate(() => localStorage.getItem("twsnmp_locale"));
    expect(toggledLocale).toBe(initialLocale === "ja" ? "en" : "ja");

    await page.reload();
    await expect(page.getByTestId("nav-map")).toBeVisible({ timeout: 15_000 });

    const persistedDark = await page.evaluate(() =>
      document.documentElement.classList.contains("dark")
    );
    const persistedLocale = await page.evaluate(() => localStorage.getItem("twsnmp_locale"));
    expect(persistedDark).toBe(toggledDark);
    expect(persistedLocale).toBe(toggledLocale);

    // Restore original settings
    await page.getByTestId("btn-theme").click();
    await page.getByTestId("btn-locale").click();

    errorMonitor.assertNoErrors();
  });
});
