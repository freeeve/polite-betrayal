# Investigate Missing Games in UI Game History

## Goal
Games simulated by the API don't appear in the UI game history. Investigate why and provide visibility.

## Root Cause
`ListByUser` SQL query in `game_repo.go` used INNER JOIN on `game_players`, but bot-only games never add the creator as a player (`game_service.go:64` skips JoinGame when botOnly=true). Creator's user_id is only in `games.creator_id`, not `game_players`, so the join excluded these games.

## Resolution
- Changed `game_repo.go:84-90` to LEFT JOIN + check both `gp.user_id` and `g.creator_id`
- Updated mock in `mock_test.go` to match new behavior
- Added `TestListGamesBotOnlyVisibleToCreator` test in `game_service_test.go`
- No UI changes needed — frontend already handles the data correctly

## Key Files
- `api/internal/repository/postgres/game_repo.go`
- `api/internal/service/mock_test.go`
- `api/internal/service/game_service_test.go`
