import { test, expect } from "@playwright/test";
import { execSync } from "child_process";
import path from "path";

// Seed data populated by the seed script before the test suite runs.
let gameId: string;

const seedScript = path.resolve(__dirname, "..", "seed_game.sh");

test.beforeAll(async () => {
  const out = execSync(`bash "${seedScript}"`, {
    encoding: "utf-8",
    timeout: 15_000,
  });

  const idMatch = out.match(/GAME_ID=(.+)/);
  if (!idMatch) {
    throw new Error(`Failed to parse seed output:\n${out}`);
  }
  gameId = idMatch[1].trim();
});

// Flutter CanvasKit renders to <canvas>, so DOM text selectors don't work.
// We use coordinate-based clicks and keyboard input instead.

test("map renders without errors", async ({ page }) => {
  const consoleErrors: string[] = [];
  page.on("console", (msg) => {
    if (msg.type() === "error") {
      consoleErrors.push(msg.text());
    }
  });

  // Navigate to app root — Flutter hash-based routing.
  await page.goto("/");

  // Wait for Flutter engine to finish loading.
  // flt-glass-pane is a zero-size wrapper so use 'attached' not 'visible'.
  await page.waitForSelector("flt-glass-pane", {
    state: "attached",
    timeout: 30_000,
  });
  // Extra time for Flutter framework init + first frame.
  await page.waitForTimeout(5_000);

  // Screenshot the login page to verify it loaded.
  await page.screenshot({ path: "screenshots/01-login-page.png" });

  // Dev login: click the text field (centered, ~56% down), clear it, type name.
  // The TextField is at roughly center of the 1280x900 viewport.
  await page.mouse.click(640, 505);
  await page.waitForTimeout(300);
  await page.keyboard.press("Meta+a");
  await page.keyboard.type("TestPlayer1");
  // Press Enter to submit (TextField has onSubmitted handler).
  await page.keyboard.press("Enter");

  // Wait for login + navigation to home screen.
  await page.waitForTimeout(3_000);
  await page.screenshot({ path: "screenshots/02-home-screen.png" });

  // Navigate directly to the game via hash URL (more reliable than clicking canvas text).
  await page.goto(`/#/game/${gameId}`);
  await page.waitForTimeout(5_000);

  // Full-page screenshot of the game map.
  await page.screenshot({
    path: "screenshots/03-game-map-full.png",
    fullPage: true,
  });

  // Viewport-only screenshot.
  await page.screenshot({
    path: "screenshots/04-game-map-viewport.png",
  });

  // Verify no JS console errors (filter out benign Flutter engine messages).
  const realErrors = consoleErrors.filter(
    (e) =>
      !e.includes("Slow network") &&
      !e.includes("ServiceWorker") &&
      !e.includes("devicePixelRatio")
  );
  expect(realErrors).toEqual([]);
});
