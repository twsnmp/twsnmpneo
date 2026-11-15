import { execFileSync } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import { chromium } from "@playwright/test";
import { E2E_DIR, startTestServer } from "../fixtures/test-server.js";
import { seedTestData } from "../fixtures/seed-data.js";
import { mockExternalMapTiles, waitForStableRender } from "../fixtures/auth.js";

function parseArg(name: string, fallback?: string): string | undefined {
  const prefix = `--${name}=`;
  const hit = process.argv.find((a) => a.startsWith(prefix));
  return hit ? hit.slice(prefix.length) : fallback;
}

function findFfmpeg(): string | null {
  const candidates = ["/opt/homebrew/bin/ffmpeg", "/usr/local/bin/ffmpeg", "ffmpeg"];
  for (const bin of candidates) {
    try {
      execFileSync(bin, ["-version"], { stdio: "ignore" });
      return bin;
    } catch {
      // try next
    }
  }
  return null;
}

async function main() {
  const scenario = parseArg("scenario", process.env.DEMO_SCENARIO || "overview")!;
  const lang = (parseArg("lang", process.env.DEMO_LANG || "ja") === "en" ? "en" : "ja") as
    | "ja"
    | "en";
  const outBase = path.resolve(
    E2E_DIR,
    parseArg("out", process.env.DEMO_OUT_DIR || "output")!
  );
  const headed = process.argv.includes("--headed") || process.env.DEMO_HEADED === "1";

  const rawDir = path.join(outBase, "raw-videos");
  fs.mkdirSync(rawDir, { recursive: true });

  console.log(`[record-demo] Starting server (scenario=${scenario}, lang=${lang})...`);
  const server = await startTestServer({ lang });
  let seeded: Awaited<ReturnType<typeof seedTestData>>;
  try {
    seeded = await seedTestData(server.baseURL, server.ports);

    const browser = await chromium.launch({
      headless: !headed,
      args: [
        "--use-gl=angle",
        "--use-angle=swiftshader",
        "--enable-webgl",
        "--ignore-gpu-blocklist",
      ],
    });

    const context = await browser.newContext({
      baseURL: server.baseURL,
      locale: lang === "en" ? "en-US" : "ja-JP",
      viewport: { width: 1440, height: 900 },
      recordVideo: {
        dir: rawDir,
        size: { width: 1440, height: 900 },
      },
    });

    const page = await context.newPage();
    await mockExternalMapTiles(page);

    const video = page.video();

    if (scenario === "public-dashboard") {
      await page.goto(`/dashboard/${seeded.publicKey}`);
      await waitForStableRender(page, 1200);
      await page.getByTestId("public-locale").click();
      await waitForStableRender(page, 1000);
      await page.getByTestId("public-test-alert").click();
      await waitForStableRender(page, 1500);
      await page.getByTestId("public-drama-dismiss").click();
      await waitForStableRender(page, 800);
    } else {
      // Overview walkthrough
      await page.goto("/");
      await waitForStableRender(page, 700);
      await page.getByTestId("login-username").fill("twsnmp");
      await page.getByTestId("login-password").fill("twsnmp");
      await page.getByTestId("login-submit").click();
      await page.getByTestId("nav-map").waitFor({ state: "visible", timeout: 15_000 });
      await waitForStableRender(page, 1000);

      // Zoom interactions on MapView
      await page.getByTestId("map-zoom-in").click();
      await waitForStableRender(page, 600);
      await page.getByTestId("map-zoom-out").click();
      await waitForStableRender(page, 700);

      // Location View
      await page.getByTestId("nav-location").click();
      await waitForStableRender(page, 1000);

      // List View + NodeDetailModal + 3D VPanel
      await page.getByTestId("nav-list").click();
      await waitForStableRender(page, 900);
      await page.getByTestId("btn-node-detail").first().click();
      await page.getByTestId("nodedetail-close").waitFor({ state: "visible" });
      await waitForStableRender(page, 900);

      const vpanelTab = page.getByTestId("nodedetail-tab-vpanel");
      if (await vpanelTab.isVisible()) {
        await vpanelTab.click();
        await waitForStableRender(page, 1400);
      }
      await page.getByTestId("nodedetail-close").click();
      await waitForStableRender(page, 600);

      // Reports View (OTel)
      await page.getByTestId("nav-reports").click();
      await waitForStableRender(page, 700);
      await page.getByTestId("report-cat-otel").click();
      await waitForStableRender(page, 900);

      // OTel View
      await page.getByTestId("nav-otel").click();
      await waitForStableRender(page, 900);
      await page.getByTestId("otel-tab-trace").click();
      await waitForStableRender(page, 900);

      // Public Dashboard & Drama Alert
      await page.goto(`/dashboard/${seeded.publicKey}`);
      await waitForStableRender(page, 1100);
      await page.getByTestId("public-test-alert").click();
      await waitForStableRender(page, 1400);
      await page.getByTestId("public-drama-dismiss").click();
      await waitForStableRender(page, 800);
    }

    // Close page and context first so Playwright finalizes and flushes the video file.
    await page.close();
    await context.close();
    await browser.close();

    if (!video) {
      throw new Error("Playwright did not produce a video handle");
    }

    const recordedPath = await video.path();
    const baseName = `${scenario}-${lang}`;
    const webmOut = path.join(outBase, `${baseName}.webm`);
    fs.copyFileSync(recordedPath, webmOut);
    console.log(`[record-demo] WebM saved: ${webmOut}`);

    const ffmpeg = findFfmpeg();
    if (ffmpeg) {
      const mp4Out = path.join(outBase, `${baseName}.mp4`);
      const gifOut = path.join(outBase, `${baseName}.gif`);
      const palettePath = path.join(rawDir, `${baseName}-palette.png`);

      execFileSync(
        ffmpeg,
        [
          "-y",
          "-i",
          webmOut,
          "-c:v",
          "libx264",
          "-pix_fmt",
          "yuv420p",
          "-movflags",
          "+faststart",
          "-preset",
          "medium",
          "-crf",
          "20",
          mp4Out,
        ],
        { stdio: "inherit" }
      );
      console.log(`[record-demo] MP4 saved: ${mp4Out}`);

      execFileSync(
        ffmpeg,
        [
          "-y",
          "-i",
          webmOut,
          "-vf",
          "fps=10,scale=960:-1:flags=lanczos,palettegen",
          palettePath,
        ],
        { stdio: "inherit" }
      );
      execFileSync(
        ffmpeg,
        [
          "-y",
          "-i",
          webmOut,
          "-i",
          palettePath,
          "-lavfi",
          "fps=10,scale=960:-1:flags=lanczos[x];[x][1:v]paletteuse",
          gifOut,
        ],
        { stdio: "inherit" }
      );
      console.log(`[record-demo] GIF saved: ${gifOut}`);
    } else {
      console.log("[record-demo] ffmpeg not found; skipped MP4/GIF transcoding.");
    }
  } finally {
    await server.stop();
  }
}

main().catch((err) => {
  console.error("[record-demo] Failed:", err);
  process.exit(1);
});
