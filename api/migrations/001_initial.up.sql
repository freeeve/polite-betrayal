CREATE TABLE users (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider    TEXT NOT NULL,
    provider_id TEXT NOT NULL,
    display_name TEXT NOT NULL,
    avatar_url  TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(provider, provider_id)
);

CREATE TABLE games (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT NOT NULL,
    creator_id      UUID NOT NULL REFERENCES users(id),
    status          TEXT NOT NULL DEFAULT 'waiting', -- waiting, active, finished
    winner          TEXT, -- power name or 'draw'
    turn_duration   INTERVAL NOT NULL DEFAULT '24 hours',
    retreat_duration INTERVAL NOT NULL DEFAULT '12 hours',
    build_duration  INTERVAL NOT NULL DEFAULT '12 hours',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at      TIMESTAMPTZ,
    finished_at     TIMESTAMPTZ
);

CREATE TABLE game_players (
    game_id  UUID NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    user_id  UUID NOT NULL REFERENCES users(id),
    power    TEXT, -- austria, england, france, germany, italy, russia, turkey
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (game_id, user_id)
);

CREATE TABLE phases (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    game_id      UUID NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    year         INT NOT NULL,
    season       TEXT NOT NULL, -- spring, fall
    phase_type   TEXT NOT NULL, -- movement, retreat, build
    state_before JSONB NOT NULL,
    state_after  JSONB,
    deadline     TIMESTAMPTZ NOT NULL,
    resolved_at  TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_phases_game ON phases(game_id, year, season, phase_type);

CREATE TABLE orders (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    phase_id   UUID NOT NULL REFERENCES phases(id) ON DELETE CASCADE,
    power      TEXT NOT NULL,
    unit_type  TEXT NOT NULL, -- army, fleet
    location   TEXT NOT NULL,
    order_type TEXT NOT NULL, -- hold, move, support, convoy
    target     TEXT,
    aux_loc    TEXT, -- for support/convoy: the destination
    aux_target TEXT, -- for support: the target of the supported order
    aux_unit_type TEXT, -- for support: the unit type being supported
    result     TEXT, -- succeeded, failed, dislodged, bounced, cut, void
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_orders_phase ON orders(phase_id, power);

CREATE TABLE messages (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    game_id      UUID NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    sender_id    UUID NOT NULL REFERENCES users(id),
    recipient_id UUID REFERENCES users(id), -- NULL = public broadcast
    content      TEXT NOT NULL,
    phase_id     UUID REFERENCES phases(id),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_messages_game ON messages(game_id, created_at);
