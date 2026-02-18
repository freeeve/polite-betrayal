import { test, expect } from "@playwright/test";
import { ApiClient, GameState, OrderInput } from "./helpers/api-client";
import { GameFixture } from "./helpers/game-fixture";

const API_URL = "http://localhost:8009";

/** Find a unit in a GameState by province. */
function unitAt(state: GameState, province: string) {
  return state.Units.find((u) => u.Province === province);
}

/** Check that no unit exists at a province. */
function noUnitAt(state: GameState, province: string) {
  return !state.Units.some((u) => u.Province === province);
}

// ─── Test 1: Basic movement ─────────────────────────────────────────────────

test("basic movement — units move to adjacent provinces", async () => {
  const api = new ApiClient(API_URL);
  const game = await GameFixture.create(api, "basic_move");

  // Spring 1901: Austria moves vie→tyr, bud→ser
  await game.submitOrders("austria", [
    { unit_type: "army", location: "vie", order_type: "move", target: "tyr" },
    { unit_type: "army", location: "bud", order_type: "move", target: "ser" },
    { unit_type: "fleet", location: "tri", order_type: "hold" },
  ]);

  for (const power of ["england", "france", "germany", "italy", "russia", "turkey"]) {
    await holdAll(game, power);
  }

  const resolved = await game.resolvePhase();
  const state = resolved.state_before;

  const tyrUnit = unitAt(state, "tyr");
  expect(tyrUnit).toBeDefined();
  expect(tyrUnit!.Power).toBe("austria");
  expect(tyrUnit!.Type).toBe(0); // Army

  const serUnit = unitAt(state, "ser");
  expect(serUnit).toBeDefined();
  expect(serUnit!.Power).toBe("austria");

  expect(noUnitAt(state, "vie")).toBe(true);
  expect(noUnitAt(state, "bud")).toBe(true);
});

// ─── Test 2: Bounce ─────────────────────────────────────────────────────────

test("bounce — two units contest same province, both stay", async () => {
  const api = new ApiClient(API_URL);
  const game = await GameFixture.create(api, "bounce");

  // Spring 1901: Austria vie→gal, Russia war→gal — both strength 1, bounce
  await game.submitOrders("austria", [
    { unit_type: "army", location: "vie", order_type: "move", target: "gal" },
    { unit_type: "army", location: "bud", order_type: "hold" },
    { unit_type: "fleet", location: "tri", order_type: "hold" },
  ]);

  await game.submitOrders("russia", [
    { unit_type: "army", location: "war", order_type: "move", target: "gal" },
    { unit_type: "army", location: "mos", order_type: "hold" },
    { unit_type: "fleet", location: "sev", order_type: "hold" },
    { unit_type: "fleet", location: "stp", order_type: "hold" },
  ]);

  for (const power of ["england", "france", "germany", "italy", "turkey"]) {
    await holdAll(game, power);
  }

  const resolved = await game.resolvePhase();
  const state = resolved.state_before;

  expect(noUnitAt(state, "gal")).toBe(true);

  const vieUnit = unitAt(state, "vie");
  expect(vieUnit).toBeDefined();
  expect(vieUnit!.Power).toBe("austria");

  const warUnit = unitAt(state, "war");
  expect(warUnit).toBeDefined();
  expect(warUnit!.Power).toBe("russia");
});

// ─── Test 3: Supported attack dislodges defender ────────────────────────────

test("supported attack — France takes Burgundy with support (2 vs 1)", async () => {
  const api = new ApiClient(API_URL);
  const game = await GameFixture.create(api, "support_attack");

  // Spring 1901: Germany mun→bur (str 1), France par→bur supported by mar (str 2)
  await game.submitOrders("germany", [
    { unit_type: "army", location: "mun", order_type: "move", target: "bur" },
    { unit_type: "army", location: "ber", order_type: "hold" },
    { unit_type: "fleet", location: "kie", order_type: "hold" },
  ]);

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

  for (const power of ["austria", "england", "italy", "russia", "turkey"]) {
    await holdAll(game, power);
  }

  const resolved = await game.resolvePhase();
  const state = resolved.state_before;

  const burUnit = unitAt(state, "bur");
  expect(burUnit).toBeDefined();
  expect(burUnit!.Power).toBe("france");

  const munUnit = unitAt(state, "mun");
  expect(munUnit).toBeDefined();
  expect(munUnit!.Power).toBe("germany");
});

// ─── Test 4: Full turn cycle — Spring → Fall → Winter build ─────────────────

test("full turn cycle — Spring move, Fall capture, Winter build", async () => {
  const api = new ApiClient(API_URL);
  const game = await GameFixture.create(api, "full_cycle");

  // ── Spring 1901: Move toward neutral SCs
  await game.submitOrders("france", [
    { unit_type: "army", location: "par", order_type: "move", target: "bur" },
    { unit_type: "army", location: "mar", order_type: "move", target: "spa" },
    { unit_type: "fleet", location: "bre", order_type: "move", target: "mao" },
  ]);

  for (const power of ["austria", "england", "germany", "italy", "russia", "turkey"]) {
    await holdAll(game, power);
  }

  const fallPhase = await game.resolvePhase();
  expect(fallPhase.season).toBe("fall");
  expect(fallPhase.phase_type).toBe("movement");
  expect(fallPhase.year).toBe(1901);

  const springState = fallPhase.state_before;
  expect(unitAt(springState, "bur")?.Power).toBe("france");
  expect(unitAt(springState, "spa")?.Power).toBe("france");
  expect(unitAt(springState, "mao")?.Power).toBe("france");

  // ── Fall 1901: Capture neutral SCs (spa is SC, por is SC, bur is NOT an SC)
  await game.submitOrders("france", [
    { unit_type: "army", location: "bur", order_type: "hold" },
    { unit_type: "army", location: "spa", order_type: "hold" },
    { unit_type: "fleet", location: "mao", order_type: "move", target: "por" },
  ]);

  for (const power of ["austria", "england", "germany", "italy", "russia", "turkey"]) {
    await holdAll(game, power);
  }

  const winterPhase = await game.resolvePhase();

  // France gained spa + por = 2 new SCs → 5 SCs, 3 units → can build 2
  if (winterPhase.phase_type === "build") {
    expect(winterPhase.year).toBe(1901);
    expect(winterPhase.season).toBe("fall");

    const buildState = winterPhase.state_before;
    const frenchSCs = Object.entries(buildState.SupplyCenters).filter(
      ([, owner]) => owner === "france"
    );
    expect(frenchSCs.length).toBeGreaterThan(3);

    // Build on vacant home SCs
    await game.submitOrders("france", [
      { unit_type: "army", location: "par", order_type: "build" },
      { unit_type: "army", location: "mar", order_type: "build" },
    ]);

    const nextPhase = await game.resolvePhase();
    expect(nextPhase.year).toBe(1902);
    expect(nextPhase.season).toBe("spring");
    expect(nextPhase.phase_type).toBe("movement");

    const newState = nextPhase.state_before;
    const frenchUnits = newState.Units.filter((u) => u.Power === "france");
    expect(frenchUnits.length).toBeGreaterThanOrEqual(4);
  } else {
    expect(winterPhase.year).toBe(1902);
    expect(winterPhase.season).toBe("spring");
  }
});

// ─── Test 5: Retreat phase triggered by dislodgement ────────────────────────

test("retreat phase — dislodged unit must retreat or disband", async () => {
  const api = new ApiClient(API_URL);
  const game = await GameFixture.create(api, "retreat");

  // ── Spring 1901: France par→bur (will bounce vs Germany), mar→gas
  // Germany mun→bur (will bounce vs France)
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

  // Both bounce on bur (1 vs 1). France: army stays at par, army at gas. Germany: army at mun.
  const fallPhase = await game.resolvePhase();
  expect(fallPhase.season).toBe("fall");

  // ── Fall 1901: France takes bur uncontested, Austria/Germany hold
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

  // ── Spring 1902: Austria vie→tyr (positioning for support next turn)
  expect(nextPhase.year).toBe(1902);
  expect(nextPhase.season).toBe("spring");

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
  expect(fall1902.year).toBe(1902);
  expect(unitAt(fall1902.state_before, "tyr")?.Power).toBe("austria");

  // ── Fall 1902: France bur→mun supported by Austria tyr (str 2 vs 1) → dislodge!
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

  const retreatState = retreatPhase.state_before;
  expect(retreatState.Dislodged).not.toBeNull();
  expect(retreatState.Dislodged!.length).toBeGreaterThan(0);

  const dislodged = retreatState.Dislodged!.find(
    (d) => d.Unit.Power === "germany" && d.DislodgedFrom === "mun"
  );
  expect(dislodged).toBeDefined();

  // German army retreats mun→boh
  await game.submitOrders("germany", [
    { unit_type: "army", location: "mun", order_type: "retreat_move", target: "boh" },
  ]);

  const afterRetreat = await game.resolvePhase();
  const afterState = afterRetreat.state_before;
  expect(unitAt(afterState, "boh")?.Power).toBe("germany");
  expect(unitAt(afterState, "mun")?.Power).toBe("france");
});

// ─── Test 6: Convoy across water ────────────────────────────────────────────

test("convoy — England convoys army across North Sea", async () => {
  const api = new ApiClient(API_URL);
  const game = await GameFixture.create(api, "convoy");

  // Spring 1901: fleet edi→nth, army lvp→edi (both valid adjacencies)
  await game.submitOrders("england", [
    { unit_type: "fleet", location: "edi", order_type: "move", target: "nth" },
    { unit_type: "army", location: "lvp", order_type: "move", target: "edi" },
    { unit_type: "fleet", location: "lon", order_type: "hold" },
  ]);
  for (const power of ["austria", "france", "germany", "italy", "russia", "turkey"]) {
    await holdAll(game, power);
  }

  const fallPhase = await game.resolvePhase();
  expect(fallPhase.season).toBe("fall");
  expect(unitAt(fallPhase.state_before, "nth")?.Power).toBe("england");
  expect(unitAt(fallPhase.state_before, "edi")?.Power).toBe("england");

  // Fall 1901: convoy army edi→nwy via F nth (edi adj to nth, nth adj to nwy)
  await game.submitOrders("england", [
    {
      unit_type: "fleet",
      location: "nth",
      order_type: "convoy",
      aux_loc: "edi",
      aux_target: "nwy",
      aux_unit_type: "army",
    },
    {
      unit_type: "army",
      location: "edi",
      order_type: "move",
      target: "nwy",
    },
    { unit_type: "fleet", location: "lon", order_type: "hold" },
  ]);
  for (const power of ["austria", "france", "germany", "italy", "russia", "turkey"]) {
    await holdAll(game, power);
  }

  const afterConvoy = await game.resolvePhase();
  const state = afterConvoy.state_before;

  const nwyUnit = unitAt(state, "nwy");
  expect(nwyUnit).toBeDefined();
  expect(nwyUnit!.Power).toBe("england");
  expect(nwyUnit!.Type).toBe(0); // Army

  expect(unitAt(state, "nth")?.Power).toBe("england");
});

// ─── Test 7: Multi-phase game progression ───────────────────────────────────

test("multi-phase progression — Spring 1901 through Spring 1902", async () => {
  const api = new ApiClient(API_URL);
  const game = await GameFixture.create(api, "multi_phase");

  // ── Verify initial state
  const initialPhase = await api.currentPhase(game.tokenFor("england"), game.gameId);
  expect(initialPhase.year).toBe(1901);
  expect(initialPhase.season).toBe("spring");
  expect(initialPhase.phase_type).toBe("movement");

  const initialState = initialPhase.state_before;
  expect(initialState.Year).toBe(1901);
  expect(initialState.Season).toBe("spring");
  expect(initialState.Units.length).toBe(22); // 7 powers × 3 + Russia's 4th

  // ── Spring 1901: Everyone holds
  for (const power of ["austria", "england", "france", "germany", "italy", "russia", "turkey"]) {
    await holdAll(game, power);
  }

  const fallPhase = await game.resolvePhase();
  expect(fallPhase.year).toBe(1901);
  expect(fallPhase.season).toBe("fall");
  expect(fallPhase.phase_type).toBe("movement");

  // ── Fall 1901: Everyone holds
  for (const power of ["austria", "england", "france", "germany", "italy", "russia", "turkey"]) {
    await holdAll(game, power);
  }

  const afterFall = await game.resolvePhase();

  // Nobody moved → no new SCs → build phase skipped or empty
  if (afterFall.phase_type === "build") {
    expect(afterFall.year).toBe(1901);
    expect(afterFall.season).toBe("fall");

    const spring1902 = await game.resolvePhase();
    expect(spring1902.year).toBe(1902);
    expect(spring1902.season).toBe("spring");
    expect(spring1902.phase_type).toBe("movement");
  } else {
    expect(afterFall.year).toBe(1902);
    expect(afterFall.season).toBe("spring");
    expect(afterFall.phase_type).toBe("movement");
  }

  // Verify SC ownership unchanged — 34 total SCs on standard map
  const finalState = (await api.currentPhase(game.tokenFor("england"), game.gameId)).state_before;
  const scEntries = Object.entries(finalState.SupplyCenters);
  expect(scEntries.length).toBe(34);

  // Verify all phases are recorded
  const allPhases = await api.listPhases(game.tokenFor("england"), game.gameId);
  expect(allPhases.length).toBeGreaterThanOrEqual(3);
});

// ─── Helpers ────────────────────────────────────────────────────────────────

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

/** Submit hold orders for all of a power's units (reads current state for position awareness). */
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
