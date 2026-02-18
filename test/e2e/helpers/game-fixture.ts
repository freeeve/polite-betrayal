import { ApiClient, Game, GameState, OrderInput, Phase } from "./api-client";

const POWERS = ["austria", "england", "france", "germany", "italy", "russia", "turkey"] as const;
type Power = (typeof POWERS)[number];

interface PlayerInfo {
  token: string;
  userId: string;
  power: string;
}

/**
 * Sets up a 7-player all-human Diplomacy game and provides convenience
 * methods for submitting orders and resolving phases.
 */
export class GameFixture {
  players: Map<string, PlayerInfo> = new Map();
  gameId: string = "";
  private creatorToken: string = "";

  constructor(private api: ApiClient) {}

  /** Create a game with 7 human players, start it, and map power assignments. */
  static async create(api: ApiClient, name: string): Promise<GameFixture> {
    const fixture = new GameFixture(api);

    // Create 7 dev users
    const logins: { token: string; userId: string; name: string }[] = [];
    for (let i = 1; i <= 7; i++) {
      const playerName = `${name}_P${i}`;
      const login = await api.devLogin(playerName);
      logins.push({ ...login, name: playerName });
    }

    fixture.creatorToken = logins[0].token;

    // Player 1 creates the game (auto-joins, 6 bots fill)
    const game = await api.createGame(logins[0].token, name, {
      turnDuration: "10m",
    });
    fixture.gameId = game.id;

    // Players 2-7 join, replacing bots
    for (let i = 1; i < 7; i++) {
      await api.joinGame(logins[i].token, fixture.gameId);
    }

    // Creator starts the game
    const started = await api.startGame(logins[0].token, fixture.gameId);

    // Map user IDs → power assignments
    for (const player of started.players) {
      const login = logins.find((l) => l.userId === player.user_id);
      if (login && player.power) {
        fixture.players.set(player.power, {
          token: login.token,
          userId: login.userId,
          power: player.power,
        });
      }
    }

    return fixture;
  }

  /** Get the auth token for a given power. */
  tokenFor(power: string): string {
    const player = this.players.get(power);
    if (!player) throw new Error(`No player found for power: ${power}`);
    return player.token;
  }

  /** Submit orders for a specific power. */
  async submitOrders(power: string, orders: OrderInput[]) {
    return this.api.submitOrders(this.tokenFor(power), this.gameId, orders);
  }

  /**
   * Mark all 7 powers as ready, triggering early resolution.
   * Then poll until a new phase appears or the current phase has state_after set.
   */
  async resolvePhase(): Promise<Phase> {
    const beforePhase = await this.api.currentPhase(this.creatorToken, this.gameId);
    const beforeId = beforePhase.id;

    // Mark all powers ready
    for (const power of POWERS) {
      await this.api.markReady(this.tokenFor(power), this.gameId);
    }

    // Poll for phase change (new phase ID means resolution happened)
    const maxWait = 15_000;
    const interval = 250;
    const start = Date.now();
    while (Date.now() - start < maxWait) {
      await sleep(interval);
      try {
        const phase = await this.api.currentPhase(this.creatorToken, this.gameId);
        if (phase.id !== beforeId) {
          return phase;
        }
      } catch {
        // Phase may briefly 404 during transition — keep polling
      }
    }

    // If no new phase, the game may have ended — return the resolved old phase
    const phases = await this.api.listPhases(this.creatorToken, this.gameId);
    const resolved = phases.find((p) => p.id === beforeId);
    if (resolved?.state_after) return resolved;

    throw new Error(`Phase did not resolve within ${maxWait}ms`);
  }

  /** Get the current game state from the current phase's state_before. */
  async currentState(): Promise<GameState> {
    const phase = await this.api.currentPhase(this.creatorToken, this.gameId);
    return phase.state_before;
  }

  /** Get the resolved state_after from a specific phase. */
  async resolvedState(phaseId: string): Promise<GameState> {
    const phases = await this.api.listPhases(this.creatorToken, this.gameId);
    const phase = phases.find((p) => p.id === phaseId);
    if (!phase?.state_after) throw new Error(`Phase ${phaseId} has no state_after`);
    return phase.state_after;
  }

  /** Get orders for a resolved phase. */
  async phaseOrders(phaseId: string) {
    return this.api.getPhaseOrders(this.creatorToken, this.gameId, phaseId);
  }
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
