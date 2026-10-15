-- Run by hand, once, after deploying the bot that credits wins by game.HumanWins and game.ImpostorWins. It is not
-- applied at startup.
--
-- Before that release, the bot gave some matches' wins to the losing side:
--   5 (the impostors disconnected) and 6 (the crewmates disconnected) were credited the wrong way round, and
--   8 (the Hide and Seek seeker caught everyone) was credited to the crewmates.
-- Roles were recorded correctly, so in a match one side won, a player won exactly when their role (1 = impostor)
-- matches the winning side. This sets every such row by that rule; only rows that disagree are touched, so running
-- it again is a no-op. Matches nobody won (aborted, unknown endings) have no player rows and are left alone.
--
-- Afterwards, cached stats pages refresh when their TTL runs out.

BEGIN;

UPDATE users_games ug
SET player_won = ((ug.player_role = 1) = (g.win_type IN (2, 3, 4, 6, 8)))
FROM games g
WHERE g.game_id = ug.game_id
  AND g.win_type IN (0, 1, 5, 7, 2, 3, 4, 6, 8)
  AND ug.player_won <> ((ug.player_role = 1) = (g.win_type IN (2, 3, 4, 6, 8)));

-- Note the "UPDATE n" above, then check nothing is left to fix before committing; this should return 0.
SELECT COUNT(*) AS still_wrong
FROM users_games ug
JOIN games g ON g.game_id = ug.game_id
WHERE g.win_type IN (0, 1, 5, 7, 2, 3, 4, 6, 8)
  AND ug.player_won <> ((ug.player_role = 1) = (g.win_type IN (2, 3, 4, 6, 8)));

COMMIT;
