import { ScenarioResult, CategoryResult, CategoryName } from "./types.js";

/**
 * Computes the average score of scenarios with status "passed" or "failed".
 * Skips scenarios with status "skipped" or "setup-failed".
 * Returns 0 for empty input or if no scoreable scenarios exist.
 */
export function computeCategoryScore(scenarios: ScenarioResult[]): number {
  const scoreable = scenarios.filter(
    (s) => s.status === "passed" || s.status === "failed"
  );
  if (scoreable.length === 0) return 0;
  const total = scoreable.reduce((sum, s) => sum + s.score, 0);
  return total / scoreable.length;
}

/**
 * Applies score killers to a score.
 * If killers are present, returns min(score, lowestCap).
 * Never raises the score above the original value.
 * Returns the score unchanged if no killers are provided.
 */
export function applyScoreKillers(
  score: number,
  killers: Array<{ condition: string; cap: number }>
): number {
  if (killers.length === 0) return score;
  const lowestCap = Math.min(...killers.map((k) => k.cap));
  return Math.min(score, lowestCap);
}

/**
 * Computes the composite score from category results.
 * Filters to tested categories, applies proportional weight scaling,
 * and returns a weighted average of cappedScore values rounded to an integer.
 */
export function computeCompositeScore(categories: CategoryResult[]): {
  compositeScore: number;
  testedCategories: CategoryName[];
  categories: CategoryResult[];
} {
  const tested = categories.filter((c) => c.tested);

  if (tested.length === 0) {
    return {
      compositeScore: 0,
      testedCategories: [],
      categories,
    };
  }

  const weightSum = tested.reduce((sum, c) => sum + c.weight, 0);

  const weightedScore = tested.reduce((sum, c) => {
    const effectiveWeight = c.weight / weightSum;
    return sum + c.cappedScore * effectiveWeight;
  }, 0);

  return {
    compositeScore: Math.round(weightedScore),
    testedCategories: tested.map((c) => c.category),
    categories,
  };
}
