import { test, expect } from "../fixtures/test.js";
import { loginViaUI, waitForStableRender } from "../fixtures/auth.js";

test.describe("Map Canvas & Context Interactions", () => {
  test("renders seeded topology on p5 canvas, zooms, saves, and adds/deletes node via context menu", async ({
    page,
    seededData,
    errorMonitor,
  }) => {
    void seededData;
    await loginViaUI(page);
    await waitForStableRender(page, 400);

    const canvas = page.locator("#p5-map-canvas canvas");
    await expect(canvas).toBeVisible();

    // Poll briefly so p5.js setup + initial redraw complete deterministically
    await expect
      .poll(
        async () =>
          await page.evaluate(() => {
            const c = document.querySelector("#p5-map-canvas canvas") as HTMLCanvasElement | null;
            if (!c) return 0;
            const ctx = c.getContext("2d");
            if (!ctx) return 0;
            const sample = ctx.getImageData(0, 0, Math.min(c.width, 900), Math.min(c.height, 500)).data;
            const seen = new Set<number>();
            for (let i = 0; i < sample.length; i += 16) {
              const rgb = (sample[i] << 16) | (sample[i + 1] << 8) | sample[i + 2];
              seen.add(rgb);
              if (seen.size >= 10) break;
            }
            return seen.size;
          }),
        { timeout: 10_000 }
      )
      .toBeGreaterThanOrEqual(4);

    // Zoom in & out
    await page.getByTestId("map-zoom-in").click();
    await page.getByTestId("map-zoom-out").click();

    // Save map positions (handles window.alert)
    page.once("dialog", async (dialog) => {
      await dialog.accept();
    });
    await page.getByTestId("map-save").click();

    // Right-click visible blank area inside #p5-map-canvas container (away from seeded nodes)
    const mapContainer = page.getByTestId("map-canvas");
    const containerBox = await mapContainer.boundingBox();
    expect(containerBox).not.toBeNull();
    const clickX = Math.min(Math.floor(containerBox!.width - 120), 960);
    const clickY = Math.min(Math.floor(containerBox!.height - 60), 420);
    await mapContainer.click({
      button: "right",
      position: { x: clickX, y: clickY },
    });

    await expect(page.getByTestId("map-context-menu")).toBeVisible();
    await page.getByTestId("map-ctx-add-node").click();

    await expect(page.getByTestId("node-dialog")).toBeVisible();
    await page.getByTestId("node-input-name").fill("E2E Map Node");
    await page.getByTestId("node-input-ip").fill("192.168.1.222");
    await page.getByTestId("node-btn-save").click();
    await expect(page.getByTestId("node-dialog")).toBeHidden();

    // Confirm the newly added node is persisted and listed, then delete via ConfirmModal
    await page.getByTestId("nav-list").click();
    await expect(page.getByTestId("node-search")).toBeVisible();
    await page.getByTestId("node-search").fill("E2E Map Node");
    await expect(page.getByText("E2E Map Node").first()).toBeVisible();

    await page.getByTestId("btn-node-delete").first().click();
    await expect(page.getByTestId("confirm-ok")).toBeVisible();
    await page.getByTestId("confirm-ok").click();
    await expect(page.getByText("E2E Map Node")).toHaveCount(0);
    await page.getByTestId("node-search").fill("");

    errorMonitor.assertNoErrors();
  });
});
