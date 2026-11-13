import type { Context } from "./scenarios.ts";
import { foundationFieldScenarios } from "./foundation-field-scenarios.ts";
import {
  routesScenario,
  paginationScenario,
  cursorsScenario,
  idsScenario,
  errorsScenario,
} from "./foundation-scenarios.ts";

export const foundationScenarios: Record<
  string,
  (c: Context) => Promise<void>
> = {
  "foundation-routes": routesScenario,
  "foundation-pagination": paginationScenario,
  "foundation-cursors": cursorsScenario,
  "foundation-ids": idsScenario,
  ...foundationFieldScenarios,
  "foundation-errors": errorsScenario,
};
