import { resolve, dirname } from "node:path";
import type { Values } from "./cli-types.ts";

type Handler = (
  values: Values,
  say: (s: string) => void,
  setExit: (code: number) => void,
) => Promise<void>;

const exitFor = (
  outcome: string,
  ok: readonly string[],
  blocked: string,
): number => (ok.includes(outcome) ? 0 : outcome === blocked ? 1 : 2);

function privateJson(result: object): boolean {
  const r = result as {
    collection?: string;
    cleanup?: { outcome?: string };
    outcome?: string;
  };
  return "collection" in r
    ? r.collection === "complete" && r.cleanup?.outcome === "clean"
    : r.outcome === "clean";
}

// Private evidence handlers load their modules lazily so public commands such as
// preflight never import recovery code or read private contracts.
async function writePrivateJson(
  out: string,
  produce: () => Promise<unknown>,
): Promise<{ value: unknown; target: string }> {
  const { privateDirectory } = await import("./metadata-files.ts");
  const { open } = await import("node:fs/promises");
  const target = resolve(out);
  await privateDirectory(dirname(target));
  if (!target.endsWith(".json"))
    throw new Error("Private output must end in .json");
  const output = await open(target, "wx", 0o600);
  try {
    const value = await produce();
    await output.writeFile(JSON.stringify(value) + "\n");
    await output.sync();
    return { value, target };
  } finally {
    await output.close();
  }
}

async function assessExternalId(
  values: Values,
  say: (s: string) => void,
  setExit: (code: number) => void,
  label: string,
  produce: () => Promise<{ outcome: string }>,
  ok: readonly string[],
  blocked: string,
) {
  if (
    !values.first ||
    !values.second ||
    !values.out ||
    Object.keys(values).some((k) => !["first", "second", "out"].includes(k))
  )
    throw new Error(`${label} requires --first, --second and --out only`);
  const { value, target } = await writePrivateJson(values.out, produce);
  const result = value as { outcome: string };
  say(`${result.outcome}: private output ${target}`);
  setExit(exitFor(result.outcome, ok, blocked));
}

export const inventoryHandlers: Record<string, Handler> = {
  "validate-target-inventory": async (values, say, setExit) => {
    const { validateTargetInventoryFile } =
      await import("./target-inventory-command.ts");
    const { report, exitCode } = await validateTargetInventoryFile(
      values.input!,
      values.out,
    );
    if (values.out)
      say(
        `${report.validation}: ${report.readiness}; ${exitCode === 2 ? "report unavailable" : "private report written"}`,
      );
    else say(JSON.stringify(report));
    setExit(exitCode);
  },
  discoverMetadata: async (values, say, setExit) => {
    if (Object.keys(values).some((k) => !["config", "out"].includes(k)))
      throw new Error("Discovery accepts only --config and --out");
    if (!values.config || !values.out)
      throw new Error("--config and --out private files required");
    const { discoverMetadataCommand } = await import("./metadata-command.ts");
    const inventory = await discoverMetadataCommand(
      values.config,
      values.out,
      process.env,
    );
    say(
      `${inventory.outcome}: ${inventory.facts.length} facts, ${inventory.gaps.length} gaps; private inventory ${resolve(values.out)}`,
    );
    setExit(inventory.outcome === "complete" ? 0 : 2);
  },
};

export const externalIdHandlers: Record<string, Handler> = {
  "assess-external-id-normalization": async (values, say, setExit) =>
    assessExternalId(
      values,
      say,
      setExit,
      "Normalization assessment",
      async () => {
        const { assessNormalizationPair } =
          await import("./external-id-normalization-assess.ts");
        return assessNormalizationPair(values.first!, values.second!);
      },
      ["repeatable"],
      "drift",
    ),
  "probe-external-id-normalization": async (values, say, setExit) =>
    externalIdMode("probe", "Normalization", values, say, setExit),
  "recover-external-id-normalization": async (values, say, setExit) =>
    externalIdMode("recover", "Normalization", values, say, setExit),
  "assess-external-id-body": async (values, say, setExit) =>
    assessExternalId(
      values,
      say,
      setExit,
      "Body assessment",
      async () => {
        const { assessBodyPair } = await import("./external-id-body-assess.ts");
        return assessBodyPair(values.first!, values.second!);
      },
      ["ready"],
      "blocked",
    ),
  "probe-external-id-body": async (values, say, setExit) =>
    externalIdMode("probe", "Body", values, say, setExit),
  "recover-external-id-body": async (values, say, setExit) =>
    externalIdMode("recover", "Body", values, say, setExit),
  "probe-external-id": async (values, say, setExit) =>
    legacyExternalId("probe", values, say, setExit),
  "recover-external-id": async (values, say, setExit) =>
    legacyExternalId("recover", values, say, setExit),
};

async function externalIdMode(
  mode: "probe" | "recover",
  label: string,
  values: Values,
  say: (s: string) => void,
  setExit: (code: number) => void,
): Promise<void> {
  const allowed =
    mode === "probe" ? ["config", "out"] : ["config", "out", "first"];
  if (
    !values.config ||
    !values.out ||
    (mode === "recover" && !values.first) ||
    Object.keys(values).some((k) => !allowed.includes(k))
  )
    throw new Error(
      `${label} command requires private --config and --out; recovery also requires --first`,
    );
  const { normalizationCommand } =
    await import("./external-id-normalization-command.ts");
  const { bodyCommand } = await import("./external-id-body-command.ts");
  const command = label === "Body" ? bodyCommand : normalizationCommand;
  const result = await command(
    mode,
    values.config,
    values.out,
    process.env,
    values.first,
  );
  const clean = privateJson(result);
  say(
    `${clean ? "complete" : "incomplete/blocked"}: private output ${resolve(values.out!)}`,
  );
  setExit(clean ? 0 : 2);
}

async function legacyExternalId(
  mode: "probe" | "recover",
  values: Values,
  say: (s: string) => void,
  setExit: (code: number) => void,
): Promise<void> {
  const allowed =
    mode === "probe" ? ["config", "out"] : ["config", "out", "first"];
  if (
    Object.keys(values).some((k) => !allowed.includes(k)) ||
    !values.config ||
    !values.out ||
    (mode === "recover" && !values.first)
  )
    throw new Error(
      "External-ID command requires private config/output and recovery run directory when recovering",
    );
  const { externalIdCommand } = await import("./external-id-command.ts");
  const result = await externalIdCommand(
    mode,
    values.config,
    values.out,
    process.env,
    values.first,
  );
  const complete =
    "collection" in result
      ? result.collection === "complete" && result.cleanup === "clean"
      : result.outcome === "clean";
  say(
    `External-ID ${mode}: ${complete ? "complete" : "incomplete/blocked"}; private output ${resolve(values.out)}`,
  );
  setExit(complete ? 0 : 2);
}

export const foundationHandlers: Record<string, Handler> = {
  "capture-foundation-assumed": async (values, say, setExit) => {
    if (
      values.target !== "reference" ||
      !values.out ||
      Object.keys(values).some((k) => !["target", "out"].includes(k))
    )
      throw new Error(
        "Assumed foundation capture requires --target reference and --out private.json only",
      );
    const { runAssumedFoundationCapture } =
      await import("./foundation-command.ts");
    const result = await runAssumedFoundationCapture(values.out, process.env);
    say(
      `${result.sidecar ? "policy capture recorded" : "blocked"}: private output ${resolve(values.out!)}`,
    );
    setExit(result.sidecar ? 0 : 2);
  },
  "assess-foundation": async (values, say, setExit) => {
    const allowed = [
      "first",
      "second",
      "first-policy",
      "second-policy",
      "first-cleanup",
      "second-cleanup",
      "out",
    ];
    if (
      Object.keys(values).some((key) => !allowed.includes(key)) ||
      allowed.some((key) => !values[key as keyof Values])
    )
      throw new Error(
        "Foundation assessment requires two captures, policies, cleanup attestations and private output",
      );
    const { assessFoundationFiles } =
      await import("./foundation-assessment-command.ts");
    const result = await assessFoundationFiles({
      first: values.first!,
      second: values.second!,
      firstPolicy: values["first-policy"]!,
      secondPolicy: values["second-policy"]!,
      firstCleanup: values["first-cleanup"]!,
      secondCleanup: values["second-cleanup"]!,
      out: values.out!,
    });
    say(
      `${result.outcome}: private foundation assessment ${resolve(values.out!)}`,
    );
    setExit(exitFor(result.outcome, ["ready-with-assumptions"], "drift"));
  },
  "verify-foundation-cleanup": async (values, say) => {
    if (
      !values.capture ||
      !values.policy ||
      !values.out ||
      Object.keys(values).some(
        (key) => !["capture", "policy", "out"].includes(key),
      )
    )
      throw new Error(
        "Foundation cleanup verification requires --capture, --policy and --out private JSON paths",
      );
    const { verifyFoundationCleanupFiles } =
      await import("./foundation-cleanup-command.ts");
    await verifyFoundationCleanupFiles(
      values.capture!,
      values.policy!,
      values.out!,
      process.env,
    );
    say(`verified absence: private output ${resolve(values.out!)}`);
  },
};
