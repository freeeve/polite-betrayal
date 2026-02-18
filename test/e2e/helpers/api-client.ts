import { APIRequestContext } from "@playwright/test";

/** Token pair returned by dev login. */
export interface AuthTokens {
  access_token: string;
  refresh_token: string;
  expires_in: number;
}

/** User model from GET /users/me. */
export interface User {
  id: string;
  provider: string;
  provider_id: string;
  display_name: string;
}

/** Player slot inside a Game. */
export interface GamePlayer {
  game_id: string;
  user_id: string;
  power: string;
  is_bot: boolean;
  bot_difficulty: string;
}

/** Game model returned by most game endpoints. */
export interface Game {
  id: string;
  name: string;
  creator_id: string;
  status: string;
  turn_duration: string;
  retreat_duration: string;
  build_duration: string;
  players: GamePlayer[];
  ready_count?: number;
}

/** Diplomacy unit inside GameState. */
export interface Unit {
  Type: number; // 0=Army, 1=Fleet
  Power: string;
  Province: string;
  Coast: string;
}

/** Dislodged unit in a retreat phase. */
export interface DislodgedUnit {
  Unit: Unit;
  DislodgedFrom: string;
  AttackerFrom: string;
}

/** Diplomacy game state (PascalCase JSON from engine). */
export interface GameState {
  Year: number;
  Season: string;
  Phase: string;
  Units: Unit[];
  SupplyCenters: Record<string, string>;
  Dislodged?: DislodgedUnit[];
}

/** Phase model returned by phase endpoints. */
export interface Phase {
  id: string;
  game_id: string;
  year: number;
  season: string;
  phase_type: string;
  state_before: GameState;
  state_after?: GameState;
  deadline: string;
  resolved_at?: string;
}

/** Single order input for submission. */
export interface OrderInput {
  unit_type: string;
  location: string;
  coast?: string;
  order_type: string;
  target?: string;
  target_coast?: string;
  aux_loc?: string;
  aux_target?: string;
  aux_unit_type?: string;
}

/** Order model returned by the API. */
export interface Order {
  id: string;
  phase_id: string;
  power: string;
  unit_type: string;
  location: string;
  order_type: string;
  target?: string;
  aux_loc?: string;
  aux_target?: string;
  aux_unit_type?: string;
  result?: string;
}

/** Response from the ready endpoint. */
export interface ReadyResponse {
  ready_count: number;
  total_powers: number;
  all_ready: boolean;
}

/** Encapsulates all API interactions for E2E gameplay tests. */
export class ApiClient {
  constructor(private baseUrl: string) {}

  private headers(token: string): Record<string, string> {
    return {
      Authorization: `Bearer ${token}`,
      "Content-Type": "application/json",
    };
  }

  /** Create a dev user and return token + user ID. */
  async devLogin(name: string): Promise<{ token: string; userId: string }> {
    const resp = await fetch(`${this.baseUrl}/auth/dev?name=${encodeURIComponent(name)}`);
    if (!resp.ok) throw new Error(`devLogin failed: ${resp.status} ${await resp.text()}`);
    const tokens: AuthTokens = await resp.json();

    const meResp = await fetch(`${this.baseUrl}/api/v1/users/me`, {
      headers: this.headers(tokens.access_token),
    });
    if (!meResp.ok) throw new Error(`getMe failed: ${meResp.status} ${await meResp.text()}`);
    const user: User = await meResp.json();

    return { token: tokens.access_token, userId: user.id };
  }

  /** Create a new game. Creator auto-joins and 6 bots are auto-filled. */
  async createGame(token: string, name: string, opts?: { turnDuration?: string }): Promise<Game> {
    const body: Record<string, string> = { name };
    if (opts?.turnDuration) body.turn_duration = opts.turnDuration;
    const resp = await fetch(`${this.baseUrl}/api/v1/games`, {
      method: "POST",
      headers: this.headers(token),
      body: JSON.stringify(body),
    });
    if (!resp.ok) throw new Error(`createGame failed: ${resp.status} ${await resp.text()}`);
    return resp.json();
  }

  /** Join a waiting game (replaces a bot if game already has 7 players). */
  async joinGame(token: string, gameId: string): Promise<void> {
    const resp = await fetch(`${this.baseUrl}/api/v1/games/${gameId}/join`, {
      method: "POST",
      headers: this.headers(token),
    });
    if (!resp.ok) throw new Error(`joinGame failed: ${resp.status} ${await resp.text()}`);
  }

  /** Start the game (must be creator, must have 7 players). */
  async startGame(token: string, gameId: string): Promise<Game> {
    const resp = await fetch(`${this.baseUrl}/api/v1/games/${gameId}/start`, {
      method: "POST",
      headers: this.headers(token),
    });
    if (!resp.ok) throw new Error(`startGame failed: ${resp.status} ${await resp.text()}`);
    return resp.json();
  }

  /** Get game details. */
  async getGame(token: string, gameId: string): Promise<Game> {
    const resp = await fetch(`${this.baseUrl}/api/v1/games/${gameId}`, {
      headers: this.headers(token),
    });
    if (!resp.ok) throw new Error(`getGame failed: ${resp.status} ${await resp.text()}`);
    return resp.json();
  }

  /** Get the current active phase for a game. */
  async currentPhase(token: string, gameId: string): Promise<Phase> {
    const resp = await fetch(`${this.baseUrl}/api/v1/games/${gameId}/phases/current`, {
      headers: this.headers(token),
    });
    if (!resp.ok) throw new Error(`currentPhase failed: ${resp.status} ${await resp.text()}`);
    return resp.json();
  }

  /** List all phases for a game. */
  async listPhases(token: string, gameId: string): Promise<Phase[]> {
    const resp = await fetch(`${this.baseUrl}/api/v1/games/${gameId}/phases`, {
      headers: this.headers(token),
    });
    if (!resp.ok) throw new Error(`listPhases failed: ${resp.status} ${await resp.text()}`);
    return resp.json();
  }

  /** Get orders for a specific phase. */
  async getPhaseOrders(token: string, gameId: string, phaseId: string): Promise<Order[]> {
    const resp = await fetch(
      `${this.baseUrl}/api/v1/games/${gameId}/phases/${phaseId}/orders`,
      { headers: this.headers(token) }
    );
    if (!resp.ok) throw new Error(`getPhaseOrders failed: ${resp.status} ${await resp.text()}`);
    return resp.json();
  }

  /** Submit orders for the current phase. */
  async submitOrders(token: string, gameId: string, orders: OrderInput[]): Promise<Order[]> {
    const resp = await fetch(`${this.baseUrl}/api/v1/games/${gameId}/orders`, {
      method: "POST",
      headers: this.headers(token),
      body: JSON.stringify({ orders }),
    });
    if (!resp.ok) throw new Error(`submitOrders failed: ${resp.status} ${await resp.text()}`);
    return resp.json();
  }

  /** Mark the player as ready. */
  async markReady(token: string, gameId: string): Promise<ReadyResponse> {
    const resp = await fetch(`${this.baseUrl}/api/v1/games/${gameId}/orders/ready`, {
      method: "POST",
      headers: this.headers(token),
    });
    if (!resp.ok) throw new Error(`markReady failed: ${resp.status} ${await resp.text()}`);
    return resp.json();
  }
}
