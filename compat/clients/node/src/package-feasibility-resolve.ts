import type {
  PackageManifest,
  PackageResolution,
  PackageIssue,
  PackageIssueCode,
  PackageDeclaration,
  PackageBinding,
} from "./package-feasibility-model.ts";

// Keys contain validated ASCII labels with no NUL. JSON tuples preserve null.
const tuple = (...parts: (string | null)[]) => JSON.stringify(parts);
const ordinal = (a: string, b: string) => (a < b ? -1 : a > b ? 1 : 0);
const issueKey = (i: PackageIssue) =>
  [
    i.code,
    i.packageId ?? "",
    i.componentKind ?? "",
    i.componentName ?? "",
  ].join("\0");
const bindingKey = (b: PackageBinding) =>
  [
    b.consumer.packageId,
    b.consumer.kind,
    b.consumer.name,
    b.provider.packageId,
    b.provider.kind,
    b.provider.name,
    b.provider.namespace ?? "",
  ].join("\0");

/** Only accepts plain, parser-owned models; no I/O or input mutation. */
export function resolvePackageManifest(
  manifest: PackageManifest,
): PackageResolution {
  const issues: PackageIssue[] = [];
  const issue = (
    code: PackageIssueCode,
    packageId: string,
    component?: PackageDeclaration["components"][number],
  ) => {
    issues.push({
      code,
      packageId,
      ...(component
        ? { componentKind: component.kind, componentName: component.name }
        : {}),
    });
  };
  const failure = (): PackageResolution => ({
    validation: "invalid",
    closure: [],
    bindings: [],
    issues: [...new Map(issues.map((i) => [issueKey(i), i])).values()].sort(
      (a, b) => ordinal(issueKey(a), issueKey(b)),
    ),
  });
  const packages = new Map<string, PackageDeclaration>();
  const roots = new Set<string>();
  for (const root of manifest.roots) {
    if (roots.has(root)) issue("duplicate-id", root);
    roots.add(root);
  }
  let duplicatePackages = false;
  for (const p of manifest.packages) {
    if (packages.has(p.id)) {
      issue("duplicate-id", p.id);
      duplicatePackages = true;
    }
    packages.set(p.id, p);
  }
  // An ambiguous package identity cannot supply a version, namespace or members.
  // Report identity errors deterministically rather than choose one declaration.
  if (duplicatePackages) return failure();
  for (const root of roots)
    if (!packages.has(root)) issue("missing-root", root);

  const members = new Map<string, Set<string>>();
  const dependencies = new Map<string, Set<string>>();
  const symbols = new Map<
    string,
    { packageId: string; component: PackageDeclaration["components"][number] }[]
  >();
  for (const p of manifest.packages) {
    const deps = new Set<string>();
    dependencies.set(p.id, deps);
    for (const d of p.dependencies) {
      if (deps.has(d.packageId)) issue("duplicate-dependency", p.id);
      deps.add(d.packageId);
      const provider = packages.get(d.packageId);
      if (!provider) issue("missing-package", p.id);
      else if (provider.version !== d.version) issue("version-mismatch", p.id);
    }
    const names = new Set<string>();
    members.set(p.id, names);
    for (const c of p.components) {
      const name = tuple(c.kind, c.name);
      if (names.has(name)) issue("duplicate-component", p.id, c);
      names.add(name);
      const symbol = tuple(p.namespace, c.kind, c.name);
      const owners = symbols.get(symbol) ?? [];
      owners.push({ packageId: p.id, component: c });
      symbols.set(symbol, owners);
      const refs = new Set<string>();
      for (const r of c.references) {
        const key = tuple(r.packageId, r.kind, r.name);
        if (refs.has(key)) issue("duplicate-reference", p.id, c);
        refs.add(key);
      }
    }
    const requirements = new Set<string>();
    for (const r of p.requirements) {
      if (requirements.has(r.id)) issue("duplicate-requirement", p.id);
      requirements.add(r.id);
    }
  }
  for (const owners of symbols.values())
    if (owners.length > 1) {
      // Attribute every owner, independent of package ordering.
      for (const { packageId, component } of owners)
        issue("symbol-collision", packageId, component);
    }
  for (const p of manifest.packages) {
    for (const c of p.components)
      for (const r of c.references) {
        if (!packages.has(r.packageId)) issue("missing-package", p.id, c);
        else {
          if (!members.get(r.packageId)!.has(tuple(r.kind, r.name)))
            issue("missing-component", p.id, c);
          if (r.packageId !== p.id && !dependencies.get(p.id)!.has(r.packageId))
            issue("undeclared-dependency", p.id, c);
        }
      }
    // Each node is tested for reachability back to itself: at most 128 nodes,
    // iterative traversal, deterministic attribution of every cyclic member.
    const visited = new Set<string>();
    const pending = [...dependencies.get(p.id)!];
    while (pending.length) {
      const id = pending.pop()!;
      if (id === p.id) {
        issue("dependency-cycle", p.id);
        break;
      }
      if (visited.has(id)) continue;
      visited.add(id);
      for (const next of dependencies.get(id) ?? [])
        if (!visited.has(next)) pending.push(next);
    }
  }
  if (issues.length) return failure();

  const closure = new Set<string>();
  const pending = [...roots];
  while (pending.length) {
    const id = pending.pop()!;
    if (closure.has(id)) continue;
    closure.add(id);
    for (const next of dependencies.get(id)!)
      if (!closure.has(next)) pending.push(next);
  }
  const bindings: PackageBinding[] = [];
  for (const id of closure) {
    const p = packages.get(id)!;
    for (const c of p.components)
      for (const r of c.references)
        bindings.push({
          consumer: { packageId: id, kind: c.kind, name: c.name },
          provider: {
            packageId: r.packageId,
            kind: r.kind,
            name: r.name,
            namespace: packages.get(r.packageId)!.namespace,
          },
        });
  }
  return {
    validation: "valid",
    issues: [],
    closure: [...closure]
      .sort(ordinal)
      .map((packageId) => ({
        packageId,
        version: packages.get(packageId)!.version,
      })),
    bindings: bindings.sort((a, b) => ordinal(bindingKey(a), bindingKey(b))),
  };
}
