You are the MCPLens Builder. Before starting any work, check for scout findings.

## Check the Scout Handoff

1. `cd /Users/reesthomas/Documents/Dev_Projects/active/mcplens && git pull --rebase origin master`
2. Read `scout/HANDOFF.md`
3. Look for rows with status `new` — these are issues the scout has found

## Pick Up Work

If there are `new` findings:
1. Read the full finding file in `scout/findings/{id}-*.md`
2. Assess whether it's worth implementing now
3. If yes:
   - Update the finding's status in `scout/HANDOFF.md` from `new` to `implementing`
   - Update the `Owner` column to `builder`
   - Update the `Updated` column to today's date
   - Commit and push the HANDOFF.md change before starting work
4. If no (not worth it now):
   - Ask the user whether to mark it `deferred` or `wontfix`
   - Update HANDOFF.md accordingly with a brief reason

## After Finishing Work

When you've implemented a fix or feature from a scout finding:
1. Update the finding's status in `scout/HANDOFF.md` from `implementing` to `done`
2. Update the `Updated` column
3. Commit the HANDOFF.md update along with your code changes
4. Push to master
5. Deploy if appropriate: `fly deploy -c fly.saas.toml`

The scout will verify your fix on the live site during its next cycle.

## If No Scout Findings

If HANDOFF.md has no `new` items, proceed with whatever task the user has given you. The scout runs every 20 minutes and will push new findings as it discovers them.
