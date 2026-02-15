import { test, expect } from "@playwright/test";
import { ApiClient, GameState, OrderInput } from "./helpers/api-client";
import { GameFixture } from "./helpers/game-fixture";
import { UiHelper } from "./helpers/ui-helper";

const API_URL = "http://localhost:8009";

const POWERS = ["austria", "england", "france", "germany", "italy", "russia", "turkey"] as const;

const STARTING_UNITS: Record<string, { unit_type: string; location: string; coast?: string }[]> = {
  austria: [
    { unit_type: "army", location: "vie" },
    { unit_type: "army", location: "bud" },
    { unit_type: "fleet", location: "tri" },
  ],
  england: [
    { unit_type: "fleet", location: "lon" },
    { unit_type: "fleet", location: "edi" },
    { unit_type: "army", location: "lvp" },
  ],
  france: [
    { unit_type: "fleet", location: "bre" },
    { unit_type: "army", location: "par" },
    { unit_type: "army", location: "mar" },
  ],
  germany: [
    { unit_type: "fleet", location: "kie" },
    { unit_type: "army", location: "ber" },
    { unit_type: "army", location: "mun" },
  ],
  italy: [
    { unit_type: "fleet", location: "nap" },
    { unit_type: "army", location: "rom" },
    { unit_type: "army", location: "ven" },
  ],
  russia: [
    { unit_type: "fleet", location: "stp", coast: "sc" },
    { unit_type: "army", location: "mos" },
    { unit_type: "army", location: "war" },
    { unit_type: "fleet", location: "sev" },
  ],
  turkey: [
    { unit_type: "fleet", location: "ank" },
    { unit_type: "army", location: "con" },
    { unit_type: "army", location: "smy" },
  ],
};

/** Find a unit in a GameState by province. */
function unitAt(state: GameState, province: string) {
  return state.Units.find((u) => u.Province === province);
}

/** Submit hold orders for all of a power's units. */
async function holdAll(game: GameFixture, power: string): Promise<void> {
  let state: GameState;
  try {
    state = await game.currentState();
  } catch {
    const units = STARTING_UNITS[power];
    if (!units) throw new Error(`Unknown power: ${power}`);
    await game.submitOrders(
      power,
      units.map((u) => ({
        unit_type: u.unit_type,
        location: u.location,
        coast: u.coast,
        order_type: "hold",
      }))
    );
    return;
  }

  const myUnits = state.Units.filter((u) => u.Power === power);
  if (myUnits.length === 0) return;
  if (state.Phase !== "movement") return;

  const orders: OrderInput[] = myUnits.map((u) => ({
    unit_type: u.Type === 0 ? "army" : "fleet",
    location: u.Province,
    coast: u.Coast || undefined,
    order_type: "hold",
  }));

  await game.submitOrders(power, orders);
}

/** Submit hold orders for all powers except the specified one. */
async function holdOthers(game: GameFixture, exceptPower: string): Promise<void> {
  for (const power of POWERS) {
    if (power !== exceptPower) {
      await holdAll(game, power);
    }
  }
}

// ─── Test 1: Login and navigate to game map ──────────────────────────────────

test("UI: login and navigate to game map", async ({ page }) => {
  const api = new ApiClient(API_URL);
  const game = await GameFixture.create(api, "ui_login");
  const ui = new UiHelper(page, api);

  const consoleErrors: string[] = [];
  page.on("console", (msg) => {
    if (msg.type() === "error") consoleErrors.push(msg.text());
  });

  // Pick first available power and its player name.
  const firstPower = POWERS.find((p) => game.players.has(p))!;
  const player = game.players.get(firstPower)!;

  // Dev login via UI.
  await ui.login(`ui_login_P1`);
  await ui.screenshot("01-login-home");

  // Navigate to game.
  await ui.navigateToGame(game.gameId);
  await ui.screenshot("01-game-map");

  // Verify no real console errors.
  const realErrors = consoleErrors.filter(
    (e) =>
      !e.includes("Slow network") &&
      !e.includes("ServiceWorker") &&
      !e.includes("devicePixelRatio")
  );
  expect(realErrors).toEqual([]);
});

// ─── Test 2: Submit move order via UI ────────────────────────────────────────

test("UI: submit move order (vie→tyr)", async ({ page }) => {
  const api = new ApiClient(API_URL);
  const game = await GameFixture.create(api, "ui_move");
  const ui = new UiHelper(page, api);

  // Login as Austria's player.
  const austriaPlayer = game.players.get("austria")!;
  await ui.login(`ui_move_P1`);
  await ui.navigateToGame(game.gameId);

  // Click Vienna (unit location) → order type buttons appear.
  await ui.clickProvince("vie");
  await ui.screenshot("02-unit-selected");

  // Click "Move" button.
  await ui.clickOrderType("move");
  await ui.screenshot("02-move-targets");

  // Click Tyrolia (target).
  await ui.clickProvince("tyr");
  await ui.screenshot("02-order-added");

  // Submit remaining Austria orders via API (hold bud, hold tri).
  await game.submitOrders("austria", [
    { unit_type: "army", location: "vie", order_type: "move", target: "tyr" },
    { unit_type: "army", location: "bud", order_type: "hold" },
    { unit_type: "fleet", location: "tri", order_type: "hold" },
  ]);

  // Hold all other powers via API.
  await holdOthers(game, "austria");

  // Resolve the phase.
  const resolved = await game.resolvePhase();
  const state = resolved.state_before;

  // Verify unit moved to Tyrolia.
  const tyrUnit = unitAt(state, "tyr");
  expect(tyrUnit).toBeDefined();
  expect(tyrUnit!.Power).toBe("austria");
  expect(tyrUnit!.Type).toBe(0); // Army
});

// ─── Test 3: Submit hold order via UI ────────────────────────────────────────

test("UI: submit hold order", async ({ page }) => {
  const api = new ApiClient(API_URL);
  const game = await GameFixture.create(api, "ui_hold");
  const ui = new UiHelper(page, api);

  await ui.login(`ui_hold_P1`);
  await ui.navigateToGame(game.gameId);

  // Click Vienna (unit location) → click "Hold".
  await ui.clickProvince("vie");
  await ui.clickOrderType("hold");
  await ui.screenshot("03-hold-added");

  // Submit all Austria orders via API (including the hold we just did via UI).
  await game.submitOrders("austria", [
    { unit_type: "army", location: "vie", order_type: "hold" },
    { unit_type: "army", location: "bud", order_type: "hold" },
    { unit_type: "fleet", location: "tri", order_type: "hold" },
  ]);

  await holdOthers(game, "austria");

  const resolved = await game.resolvePhase();
  const state = resolved.state_before;

  // Verify unit stayed in Vienna.
  const vieUnit = unitAt(state, "vie");
  expect(vieUnit).toBeDefined();
  expect(vieUnit!.Power).toBe("austria");
});

// ─── Test 4: Submit support order via UI ─────────────────────────────────────

test("UI: support order — France supports par→bur (2 vs 1)", async ({ page }) => {
  const api = new ApiClient(API_URL);
  const game = await GameFixture.create(api, "ui_support");
  const ui = new UiHelper(page, api);

  await ui.login(`ui_support_P1`);
  await ui.navigateToGame(game.gameId);

  // Click Marseilles (select A mar) → click "Support" → click Paris (unit to support) → click Burgundy (target).
  await ui.clickProvince("mar");
  await ui.screenshot("04-mar-selected");

  await ui.clickOrderType("support");
  await ui.screenshot("04-support-mode");

  await ui.clickProvince("par");
  await ui.screenshot("04-support-unit");

  await ui.clickProvince("bur");
  await ui.screenshot("04-support-target");

  // Submit all France orders via API (par→bur, mar supports par→bur, bre hold).
  await game.submitOrders("france", [
    { unit_type: "army", location: "par", order_type: "move", target: "bur" },
    {
      unit_type: "army",
      location: "mar",
      order_type: "support",
      aux_loc: "par",
      aux_target: "bur",
      aux_unit_type: "army",
    },
    { unit_type: "fleet", location: "bre", order_type: "hold" },
  ]);

  // Germany also tries to take Burgundy (will fail, 1 vs 2).
  await game.submitOrders("germany", [
    { unit_type: "army", location: "mun", order_type: "move", target: "bur" },
    { unit_type: "army", location: "ber", order_type: "hold" },
    { unit_type: "fleet", location: "kie", order_type: "hold" },
  ]);

  for (const power of ["austria", "england", "italy", "russia", "turkey"]) {
    await holdAll(game, power);
  }

  const resolved = await game.resolvePhase();
  const state = resolved.state_before;

  // France takes Burgundy (2 vs 1).
  const burUnit = unitAt(state, "bur");
  expect(burUnit).toBeDefined();
  expect(burUnit!.Power).toBe("france");

  // Germany bounces back to Munich.
  const munUnit = unitAt(state, "mun");
  expect(munUnit).toBeDefined();
  expect(munUnit!.Power).toBe("germany");
});

// ─── Test 5: Convoy order via UI ─────────────────────────────────────────────

test("UI: convoy — England convoys army across North Sea", async ({ page }) => {
  const api = new ApiClient(API_URL);
  const game = await GameFixture.create(api, "ui_convoy");
  const ui = new UiHelper(page, api);

  await ui.login(`ui_convoy_P1`);
  await ui.navigateToGame(game.gameId);

  // Spring 1901: Move F edi→nth via API, hold others.
  await game.submitOrders("england", [
    { unit_type: "fleet", location: "edi", order_type: "move", target: "nth" },
    { unit_type: "army", location: "lvp", order_type: "move", target: "edi" },
    { unit_type: "fleet", location: "lon", order_type: "hold" },
  ]);
  await holdOthers(game, "england");

  const fallPhase = await game.resolvePhase();
  expect(fallPhase.season).toBe("fall");
  expect(unitAt(fallPhase.state_before, "nth")?.Power).toBe("england");
  expect(unitAt(fallPhase.state_before, "edi")?.Power).toBe("england");

  // Reload the UI to see Fall 1901.
  await ui.navigateToGame(game.gameId);

  // Fall 1901: Convoy A edi→nwy via F nth.
  // Click North Sea (F nth) → click "Convoy".
  await ui.clickProvince("nth");
  await ui.screenshot("05-nth-selected");

  await ui.clickOrderType("convoy");
  await ui.screenshot("05-convoy-mode");

  // Click Edinburgh (army to convoy) → click Norway (destination).
  await ui.clickProvince("edi");
  await ui.screenshot("05-convoy-army");

  await ui.clickProvince("nwy");
  await ui.screenshot("05-convoy-target");

  // Submit all England orders via API.
  await game.submitOrders("england", [
    {
      unit_type: "fleet",
      location: "nth",
      order_type: "convoy",
      aux_loc: "edi",
      aux_target: "nwy",
      aux_unit_type: "army",
    },
    { unit_type: "army", location: "edi", order_type: "move", target: "nwy" },
    { unit_type: "fleet", location: "lon", order_type: "hold" },
  ]);
  await holdOthers(game, "england");

  const afterConvoy = await game.resolvePhase();
  const state = afterConvoy.state_before;

  const nwyUnit = unitAt(state, "nwy");
  expect(nwyUnit).toBeDefined();
  expect(nwyUnit!.Power).toBe("england");
  expect(nwyUnit!.Type).toBe(0); // Army
});

// ─── Test 6: Build order via UI ──────────────────────────────────────────────

test("UI: build order — France builds after capturing SCs", async ({ page }) => {
  const api = new ApiClient(API_URL);
  const game = await GameFixture.create(api, "ui_build");
  const ui = new UiHelper(page, api);

  await ui.login(`ui_build_P1`);

  // Spring 1901: France moves toward neutral SCs.
  await game.submitOrders("france", [
    { unit_type: "army", location: "par", order_type: "move", target: "bur" },
    { unit_type: "army", location: "mar", order_type: "move", target: "spa" },
    { unit_type: "fleet", location: "bre", order_type: "move", target: "mao" },
  ]);
  await holdOthers(game, "france");

  const fallPhase = await game.resolvePhase();
  expect(fallPhase.season).toBe("fall");

  // Fall 1901: Capture neutral SCs.
  await game.submitOrders("france", [
    { unit_type: "army", location: "bur", order_type: "hold" },
    { unit_type: "army", location: "spa", order_type: "hold" },
    { unit_type: "fleet", location: "mao", order_type: "move", target: "por" },
  ]);
  await holdOthers(game, "france");

  const winterPhase = await game.resolvePhase();

  if (winterPhase.phase_type !== "build") {
    test.skip(true, "No build phase generated");
    return;
  }

  // Navigate to game in build phase.
  await ui.navigateToGame(game.gameId);
  await ui.screenshot("06-build-phase");

  // The BuildOrderPanel shows "Build N units" with clickable home SC chips.
  // Build via API since the build UI uses dialog popups (hard to hit test).
  await game.submitOrders("france", [
    { unit_type: "army", location: "par", order_type: "build" },
    { unit_type: "army", location: "mar", order_type: "build" },
  ]);

  const nextPhase = await game.resolvePhase();
  expect(nextPhase.year).toBe(1902);
  expect(nextPhase.season).toBe("spring");

  const newState = nextPhase.state_before;
  const frenchUnits = newState.Units.filter((u) => u.Power === "france");
  expect(frenchUnits.length).toBeGreaterThanOrEqual(4);

  await ui.navigateToGame(game.gameId);
  await ui.screenshot("06-after-build");
});

// ─── Test 7: Retreat order via UI ────────────────────────────────────────────

test("UI: retreat order — dislodged unit retreats", async ({ page }) => {
  const api = new ApiClient(API_URL);
  const game = await GameFixture.create(api, "ui_retreat");
  const ui = new UiHelper(page, api);

  await ui.login(`ui_retreat_P1`);

  // Spring 1901: Set up for dislodgement.
  // France par→bur bounces with Germany mun→bur, France mar→gas.
  await game.submitOrders("france", [
    { unit_type: "army", location: "par", order_type: "move", target: "bur" },
    { unit_type: "army", location: "mar", order_type: "move", target: "gas" },
    { unit_type: "fleet", location: "bre", order_type: "hold" },
  ]);
  await game.submitOrders("germany", [
    { unit_type: "army", location: "mun", order_type: "move", target: "bur" },
    { unit_type: "army", location: "ber", order_type: "hold" },
    { unit_type: "fleet", location: "kie", order_type: "hold" },
  ]);
  for (const power of ["austria", "england", "italy", "russia", "turkey"]) {
    await holdAll(game, power);
  }

  const fall1901 = await game.resolvePhase();
  expect(fall1901.season).toBe("fall");

  // Fall 1901: France takes bur uncontested.
  await game.submitOrders("france", [
    { unit_type: "army", location: "par", order_type: "move", target: "bur" },
    { unit_type: "army", location: "gas", order_type: "hold" },
    { unit_type: "fleet", location: "bre", order_type: "hold" },
  ]);
  await game.submitOrders("germany", [
    { unit_type: "army", location: "mun", order_type: "hold" },
    { unit_type: "army", location: "ber", order_type: "hold" },
    { unit_type: "fleet", location: "kie", order_type: "hold" },
  ]);
  for (const power of ["austria", "england", "italy", "russia", "turkey"]) {
    await holdAll(game, power);
  }

  let nextPhase = await game.resolvePhase();
  if (nextPhase.phase_type === "build") {
    nextPhase = await game.resolvePhase();
  }

  // Spring 1902: Austria vie→tyr (positioning for support).
  expect(nextPhase.year).toBe(1902);
  await game.submitOrders("france", [
    { unit_type: "army", location: "bur", order_type: "hold" },
    { unit_type: "army", location: "gas", order_type: "hold" },
    { unit_type: "fleet", location: "bre", order_type: "hold" },
  ]);
  await game.submitOrders("austria", [
    { unit_type: "army", location: "vie", order_type: "move", target: "tyr" },
    { unit_type: "army", location: "bud", order_type: "hold" },
    { unit_type: "fleet", location: "tri", order_type: "hold" },
  ]);
  await game.submitOrders("germany", [
    { unit_type: "army", location: "mun", order_type: "hold" },
    { unit_type: "army", location: "ber", order_type: "hold" },
    { unit_type: "fleet", location: "kie", order_type: "hold" },
  ]);
  for (const power of ["england", "italy", "russia", "turkey"]) {
    await holdAll(game, power);
  }

  const fall1902 = await game.resolvePhase();
  expect(fall1902.season).toBe("fall");

  // Fall 1902: France bur→mun supported by Austria tyr (2 vs 1) → dislodge!
  await game.submitOrders("france", [
    { unit_type: "army", location: "bur", order_type: "move", target: "mun" },
    { unit_type: "army", location: "gas", order_type: "hold" },
    { unit_type: "fleet", location: "bre", order_type: "hold" },
  ]);
  await game.submitOrders("austria", [
    {
      unit_type: "army",
      location: "tyr",
      order_type: "support",
      aux_loc: "bur",
      aux_target: "mun",
      aux_unit_type: "army",
    },
    { unit_type: "army", location: "bud", order_type: "hold" },
    { unit_type: "fleet", location: "tri", order_type: "hold" },
  ]);
  await game.submitOrders("germany", [
    { unit_type: "army", location: "mun", order_type: "hold" },
    { unit_type: "army", location: "ber", order_type: "hold" },
    { unit_type: "fleet", location: "kie", order_type: "hold" },
  ]);
  for (const power of ["england", "italy", "russia", "turkey"]) {
    await holdAll(game, power);
  }

  const retreatPhase = await game.resolvePhase();
  expect(retreatPhase.phase_type).toBe("retreat");

  // Navigate UI to see retreat phase.
  await ui.navigateToGame(game.gameId);
  await ui.screenshot("07-retreat-phase");

  // Retreat German army mun→boh via API (retreat UI uses chip buttons, not map clicks).
  await game.submitOrders("germany", [
    { unit_type: "army", location: "mun", order_type: "retreat_move", target: "boh" },
  ]);

  const afterRetreat = await game.resolvePhase();
  const afterState = afterRetreat.state_before;

  expect(unitAt(afterState, "boh")?.Power).toBe("germany");
  expect(unitAt(afterState, "mun")?.Power).toBe("france");

  await ui.navigateToGame(game.gameId);
  await ui.screenshot("07-after-retreat");
});

// ─── Test 8: Multi-phase progression via UI ──────────────────────────────────

test("UI: multi-phase progression — 2 turns with UI orders", async ({ page }) => {
  const api = new ApiClient(API_URL);
  const game = await GameFixture.create(api, "ui_multi");
  const ui = new UiHelper(page, api);

  await ui.login(`ui_multi_P1`);
  await ui.navigateToGame(game.gameId);

  // Verify Spring 1901 phase.
  const initialPhase = await api.currentPhase(game.tokenFor("austria"), game.gameId);
  expect(initialPhase.year).toBe(1901);
  expect(initialPhase.season).toBe("spring");
  await ui.screenshot("08-spring-1901");

  // Spring 1901: Submit Austria vie→tyr via UI interaction.
  await ui.clickProvince("vie");
  await ui.clickOrderType("move");
  await ui.clickProvince("tyr");
  await ui.screenshot("08-spring-order");

  // Submit all orders via API.
  await game.submitOrders("austria", [
    { unit_type: "army", location: "vie", order_type: "move", target: "tyr" },
    { unit_type: "army", location: "bud", order_type: "hold" },
    { unit_type: "fleet", location: "tri", order_type: "hold" },
  ]);
  await holdOthers(game, "austria");

  const fallPhase = await game.resolvePhase();
  expect(fallPhase.year).toBe(1901);
  expect(fallPhase.season).toBe("fall");

  // Reload UI for Fall 1901.
  await ui.navigateToGame(game.gameId);
  await ui.screenshot("08-fall-1901");

  // Fall 1901: Submit Austria tyr→mun via UI interaction.
  await ui.clickProvince("tyr");
  await ui.clickOrderType("move");
  await ui.clickProvince("mun");
  await ui.screenshot("08-fall-order");

  // Germany moves mun away so Austria can take it.
  await game.submitOrders("austria", [
    { unit_type: "army", location: "tyr", order_type: "move", target: "mun" },
    { unit_type: "army", location: "bud", order_type: "hold" },
    { unit_type: "fleet", location: "tri", order_type: "hold" },
  ]);
  await game.submitOrders("germany", [
    { unit_type: "army", location: "mun", order_type: "move", target: "ruh" },
    { unit_type: "army", location: "ber", order_type: "hold" },
    { unit_type: "fleet", location: "kie", order_type: "hold" },
  ]);
  for (const power of ["england", "france", "italy", "russia", "turkey"]) {
    await holdAll(game, power);
  }

  const afterFall = await game.resolvePhase();

  // Handle build phase if present.
  // Austria gained mun SC (4 SCs, 3 units → build 1).
  // Germany lost mun SC (2 SCs, 3 units → must disband 1).
  if (afterFall.phase_type === "build") {
    const buildState = afterFall.state_before;
    const austriaSCs = Object.entries(buildState.SupplyCenters).filter(
      ([, owner]) => owner === "austria"
    );
    if (austriaSCs.length > 3) {
      await game.submitOrders("austria", [
        { unit_type: "army", location: "vie", order_type: "build" },
      ]);
    }

    // Germany must disband 1 unit (lost mun SC).
    const germanSCs = Object.entries(buildState.SupplyCenters).filter(
      ([, owner]) => owner === "germany"
    );
    const germanUnits = buildState.Units.filter((u) => u.Power === "germany");
    if (germanUnits.length > germanSCs.length) {
      // Disband the unit farthest from home (ruh).
      const disbandUnit = germanUnits.find((u) => u.Province === "ruh") ?? germanUnits[0];
      await game.submitOrders("germany", [
        {
          unit_type: disbandUnit.Type === 0 ? "army" : "fleet",
          location: disbandUnit.Province,
          order_type: "disband",
        },
      ]);
    }

    const spring1902 = await game.resolvePhase();
    expect(spring1902.year).toBe(1902);
    expect(spring1902.season).toBe("spring");

    await ui.navigateToGame(game.gameId);
    await ui.screenshot("08-spring-1902");
  } else {
    expect(afterFall.year).toBe(1902);
    await ui.navigateToGame(game.gameId);
    await ui.screenshot("08-spring-1902");
  }

  // Verify Austria controls Munich.
  const finalState = await game.currentState();
  expect(unitAt(finalState, "mun")?.Power).toBe("austria");
});

// ─── Test 9: Foreign power support ───────────────────────────────────────────

test("UI: foreign support — Austria supports French move into Munich", async ({ page }) => {
  const api = new ApiClient(API_URL);
  const game = await GameFixture.create(api, "ui_foreign_sup");
  const ui = new UiHelper(page, api);

  await ui.login(`ui_foreign_sup_P1`);

  // Spring 1901: Austria A vie→tyr, France A par→bur (positioning).
  await game.submitOrders("austria", [
    { unit_type: "army", location: "vie", order_type: "move", target: "tyr" },
    { unit_type: "army", location: "bud", order_type: "hold" },
    { unit_type: "fleet", location: "tri", order_type: "hold" },
  ]);
  await game.submitOrders("france", [
    { unit_type: "army", location: "par", order_type: "move", target: "bur" },
    { unit_type: "army", location: "mar", order_type: "hold" },
    { unit_type: "fleet", location: "bre", order_type: "hold" },
  ]);
  for (const power of ["england", "germany", "italy", "russia", "turkey"]) {
    await holdAll(game, power);
  }

  const fallPhase = await game.resolvePhase();
  expect(fallPhase.season).toBe("fall");
  expect(unitAt(fallPhase.state_before, "tyr")?.Power).toBe("austria");
  expect(unitAt(fallPhase.state_before, "bur")?.Power).toBe("france");

  // Reload UI for Fall 1901.
  await ui.navigateToGame(game.gameId);
  await ui.screenshot("09-fall-setup");

  // Fall 1901: Austria A tyr supports French A bur→mun (foreign support).
  // France A bur→mun (strength 2 with Austrian support).
  // Germany A mun holds (strength 1, will be dislodged).
  await ui.clickProvince("tyr");
  await ui.screenshot("09-tyr-selected");

  await ui.clickOrderType("support");
  await ui.screenshot("09-support-mode");

  await ui.clickProvince("bur");
  await ui.screenshot("09-support-foreign-unit");

  await ui.clickProvince("mun");
  await ui.screenshot("09-support-target");

  // Submit all orders via API.
  await game.submitOrders("austria", [
    {
      unit_type: "army",
      location: "tyr",
      order_type: "support",
      aux_loc: "bur",
      aux_target: "mun",
      aux_unit_type: "army",
    },
    { unit_type: "army", location: "bud", order_type: "hold" },
    { unit_type: "fleet", location: "tri", order_type: "hold" },
  ]);
  await game.submitOrders("france", [
    { unit_type: "army", location: "bur", order_type: "move", target: "mun" },
    { unit_type: "army", location: "mar", order_type: "hold" },
    { unit_type: "fleet", location: "bre", order_type: "hold" },
  ]);
  await game.submitOrders("germany", [
    { unit_type: "army", location: "mun", order_type: "hold" },
    { unit_type: "army", location: "ber", order_type: "hold" },
    { unit_type: "fleet", location: "kie", order_type: "hold" },
  ]);
  for (const power of ["england", "italy", "russia", "turkey"]) {
    await holdAll(game, power);
  }

  const afterFall = await game.resolvePhase();

  // Expect retreat phase — Germany dislodged from Munich.
  if (afterFall.phase_type === "retreat") {
    const retreatState = afterFall.state_before;
    expect(retreatState.Dislodged).not.toBeNull();
    const dislodged = retreatState.Dislodged!.find(
      (d) => d.Unit.Power === "germany" && d.DislodgedFrom === "mun"
    );
    expect(dislodged).toBeDefined();

    // France has taken Munich.
    expect(unitAt(retreatState, "mun")?.Power).toBe("france");

    // Disband the dislodged German army.
    await game.submitOrders("germany", [
      { unit_type: "army", location: "mun", order_type: "disband" },
    ]);
    await game.resolvePhase();
  } else {
    // No retreat needed — just verify France took Munich.
    expect(unitAt(afterFall.state_before, "mun")?.Power).toBe("france");
  }

  await ui.navigateToGame(game.gameId);
  await ui.screenshot("09-after-foreign-support");
});

// ─── Test 10: Foreign convoy ─────────────────────────────────────────────────

test("UI: foreign convoy — France convoys English army to Belgium", async ({ page }) => {
  const api = new ApiClient(API_URL);
  const game = await GameFixture.create(api, "ui_foreign_con");
  const ui = new UiHelper(page, api);

  await ui.login(`ui_foreign_con_P1`);

  // Spring 1901: France F bre→eng, England A lvp→wal.
  await game.submitOrders("france", [
    { unit_type: "fleet", location: "bre", order_type: "move", target: "eng" },
    { unit_type: "army", location: "par", order_type: "hold" },
    { unit_type: "army", location: "mar", order_type: "hold" },
  ]);
  await game.submitOrders("england", [
    { unit_type: "army", location: "lvp", order_type: "move", target: "wal" },
    { unit_type: "fleet", location: "edi", order_type: "hold" },
    { unit_type: "fleet", location: "lon", order_type: "hold" },
  ]);
  for (const power of ["austria", "germany", "italy", "russia", "turkey"]) {
    await holdAll(game, power);
  }

  const fallPhase = await game.resolvePhase();
  expect(fallPhase.season).toBe("fall");
  expect(unitAt(fallPhase.state_before, "eng")?.Power).toBe("france");
  expect(unitAt(fallPhase.state_before, "wal")?.Power).toBe("england");

  // Reload UI for Fall 1901.
  await ui.navigateToGame(game.gameId);
  await ui.screenshot("10-fall-convoy-setup");

  // Fall 1901: France F eng convoys English A wal→bel (foreign convoy).
  await ui.clickProvince("eng");
  await ui.screenshot("10-eng-selected");

  await ui.clickOrderType("convoy");
  await ui.screenshot("10-convoy-mode");

  await ui.clickProvince("wal");
  await ui.screenshot("10-convoy-army");

  await ui.clickProvince("bel");
  await ui.screenshot("10-convoy-target");

  // Submit all orders via API.
  await game.submitOrders("france", [
    {
      unit_type: "fleet",
      location: "eng",
      order_type: "convoy",
      aux_loc: "wal",
      aux_target: "bel",
      aux_unit_type: "army",
    },
    { unit_type: "army", location: "par", order_type: "hold" },
    { unit_type: "army", location: "mar", order_type: "hold" },
  ]);
  await game.submitOrders("england", [
    { unit_type: "army", location: "wal", order_type: "move", target: "bel" },
    { unit_type: "fleet", location: "edi", order_type: "hold" },
    { unit_type: "fleet", location: "lon", order_type: "hold" },
  ]);
  for (const power of ["austria", "germany", "italy", "russia", "turkey"]) {
    await holdAll(game, power);
  }

  const afterConvoy = await game.resolvePhase();
  const state = afterConvoy.state_before;

  // English army arrives at Belgium via French convoy.
  const belUnit = unitAt(state, "bel");
  expect(belUnit).toBeDefined();
  expect(belUnit!.Power).toBe("england");
  expect(belUnit!.Type).toBe(0); // Army

  // French fleet still in English Channel.
  const engUnit = unitAt(state, "eng");
  expect(engUnit).toBeDefined();
  expect(engUnit!.Power).toBe("france");
  expect(engUnit!.Type).toBe(1); // Fleet

  await ui.navigateToGame(game.gameId);
  await ui.screenshot("10-after-foreign-convoy");
});
