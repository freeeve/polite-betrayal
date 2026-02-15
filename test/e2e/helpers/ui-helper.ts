import { Page } from "@playwright/test";
import { ApiClient } from "./api-client";
import { PROVINCE_CENTERS, SVG_VIEWBOX } from "./province-coords";

/** Layout constants derived from Flutter widget tree. */
const APPBAR_HEIGHT = 56;
const PHASEBAR_HEIGHT = 48;
const SIDE_PANEL_WIDTH = 250;
const WIDE_BREAKPOINT = 900;

/**
 * Estimates bottom chrome height (SupplyCenterTable + OrderActionBar idle state).
 * SupplyCenterTable: ~52px (20px circle + 2px gap + 14px text + 8px padding)
 * OrderActionBar idle: ~56px (prompt text + gap + "Select a unit" text + 16px padding)
 */
const BOTTOM_CHROME_HEIGHT = 108;

/** Encapsulates Flutter canvas UI interactions for E2E tests. */
export class UiHelper {
  constructor(
    private page: Page,
    private api: ApiClient
  ) {}

  /** Compute map widget bounds from known layout constants. */
  private getMapBounds(): { left: number; top: number; size: number } {
    const vw = 1280;
    const vh = 900;
    const isWide = vw >= WIDE_BREAKPOINT;
    const panelWidth = isWide ? SIDE_PANEL_WIDTH : 0;

    const contentWidth = vw - panelWidth;
    const contentTop = APPBAR_HEIGHT + PHASEBAR_HEIGHT;
    const availableHeight = vh - contentTop - BOTTOM_CHROME_HEIGHT;

    // Map is AspectRatio(1:1), constrained by the smaller dimension.
    const mapSize = Math.min(contentWidth, availableHeight);
    const mapLeft = panelWidth + (contentWidth - mapSize) / 2;
    const mapTop = contentTop;

    return { left: mapLeft, top: mapTop, size: mapSize };
  }

  /** Convert SVG province center to screen coordinates. */
  provinceScreenCoords(provinceId: string): { x: number; y: number } {
    const center = PROVINCE_CENTERS[provinceId];
    if (!center) throw new Error(`Unknown province: ${provinceId}`);

    const { left, top, size } = this.getMapBounds();
    return {
      x: Math.round(left + (center.x / SVG_VIEWBOX) * size),
      y: Math.round(top + (center.y / SVG_VIEWBOX) * size),
    };
  }

  /** Login via dev auth in the Flutter UI (click text field, type name, press Enter). */
  async login(playerName: string): Promise<void> {
    await this.page.goto("/");
    await this.waitForFlutter();

    // Dev login text field is centered in the 1280x900 viewport.
    await this.page.mouse.click(640, 505);
    await this.page.waitForTimeout(300);
    await this.page.keyboard.press("Meta+a");
    await this.page.keyboard.type(playerName);
    await this.page.keyboard.press("Enter");

    // Wait for login + navigation to home screen.
    await this.page.waitForTimeout(3_000);
  }

  /** Navigate to game and wait for map render. */
  async navigateToGame(gameId: string): Promise<void> {
    await this.page.goto(`/#/game/${gameId}`);
    await this.page.waitForTimeout(5_000);
  }

  /** Wait for Flutter to finish rendering. */
  async waitForFlutter(ms?: number): Promise<void> {
    await this.page.waitForSelector("flt-glass-pane", {
      state: "attached",
      timeout: 30_000,
    });
    await this.page.waitForTimeout(ms ?? 5_000);
  }

  /** Click on a province on the map. */
  async clickProvince(provinceId: string): Promise<void> {
    const { x, y } = this.provinceScreenCoords(provinceId);
    await this.page.mouse.click(x, y);
    await this.page.waitForTimeout(500);
  }

  /**
   * Click order type button in OrderActionBar.
   * Buttons appear in a Wrap after clicking a unit: Hold, Move, Support, Convoy, Cancel.
   * The bar is at the bottom of the viewport. Buttons are ~80px wide with 8px spacing.
   */
  async clickOrderType(type: "hold" | "move" | "support" | "convoy"): Promise<void> {
    const vh = 900;
    // OrderActionBar: prompt text (~20px from top of bar) + 8px gap + buttons row
    // The bar's top is roughly vh - 56px. Button centers ~30px from bottom.
    const buttonY = vh - 26;

    // Buttons are in a Wrap centered in the content area.
    // Content area starts at 250px (side panel), width 1030px.
    // 5 buttons × ~90px each ≈ 450px total. Centered: start at 250 + (1030-450)/2 ≈ 540px.
    const buttonOrder = ["hold", "move", "support", "convoy"];
    const idx = buttonOrder.indexOf(type);
    if (idx < 0) throw new Error(`Unknown order type: ${type}`);

    const buttonWidth = 90;
    const totalWidth = buttonOrder.length * buttonWidth + 3 * 8; // 4 buttons + Cancel
    const startX = SIDE_PANEL_WIDTH + (1030 - totalWidth) / 2;
    const buttonX = startX + idx * (buttonWidth + 8) + buttonWidth / 2;

    await this.page.mouse.click(Math.round(buttonX), buttonY);
    await this.page.waitForTimeout(500);
  }

  /** Click the Submit button in OrderActionBar or retreat/build panel. */
  async clickSubmit(): Promise<void> {
    const vh = 900;
    // Submit button is centered in the bottom bar.
    const contentCenterX = SIDE_PANEL_WIDTH + 1030 / 2;
    // Submit appears in idle phase at bottom of OrderActionBar.
    await this.page.mouse.click(Math.round(contentCenterX), vh - 26);
    await this.page.waitForTimeout(1_000);
  }

  /** Click the Ready (skip) button for retreat/build phases. */
  async clickReady(): Promise<void> {
    const vh = 900;
    // Ready button is to the right of Submit in the bottom row.
    const contentCenterX = SIDE_PANEL_WIDTH + 1030 / 2;
    await this.page.mouse.click(Math.round(contentCenterX) + 100, vh - 20);
    await this.page.waitForTimeout(1_000);
  }

  /** Take a screenshot for visual verification. */
  async screenshot(name: string): Promise<void> {
    await this.page.screenshot({ path: `screenshots/ui-${name}.png` });
  }
}
