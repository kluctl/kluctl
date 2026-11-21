import { useCallback, useMemo } from "react"
import { ChevronRight } from "lucide-react"
import { Link, useLocation, useSearchParams } from "react-router-dom"
import { getObject, getResult, type ObjectVersion } from "@/api"
import { refKey, refLabel, runOutcome, type Manifest } from "@/domain"
import { useLoad } from "@/hooks"
import type {
  CommandResult,
  CommandResultSummary,
  DeploymentItemConfig,
  DeploymentProjectConfig,
  ObjectRef,
  ResultObject,
  ShortName,
  VarsSource,
} from "@/models"
import { Changes, DiffView, Logs, Messages, categoriesOf } from "./diagnostics"
import {
  Empty,
  ErrorNotice,
  Heading,
  ManifestView,
  Properties,
  QueryTabs,
  Status,
  Time,
  Yaml,
  duration,
} from "./detail-primitives"
import { TabsContent } from "./ui/tabs"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "./ui/select"

export function Outcome({ run }: { run: CommandResultSummary }) {
  const label = runOutcome(run)
  return (
    <Status
      tone={
        label === "Errors"
          ? "bad"
          : label === "Warnings"
            ? "warn"
            : label === "Changes"
              ? "info"
              : "muted"
      }
    >
      {label}
    </Status>
  )
}
export function RunCounts({ run }: { run: CommandResultSummary }) {
  const counts = [
    ["New", run.newObjects],
    ["Changed", run.changedObjects],
    ["Deleted", run.deletedObjects],
    ["Orphaned", run.orphanObjects],
  ] as const
  const compact = counts.filter(([, count]) => count > 0)
  return (
    <span className="text-xs text-muted-foreground">
      {compact.length
        ? compact
            .map(([label, count]) => `${count} ${label.toLowerCase()}`)
            .join(" · ")
        : "No resource changes"}
    </span>
  )
}

function RunSummary({
  result,
  run,
}: {
  result: CommandResult
  run: CommandResultSummary
}) {
  const command = result.command ?? run.commandInfo
  const git = result.gitInfo ?? run.gitInfo
  const {
    args,
    images,
    startTime,
    endTime,
    command: name,
    initiator,
    ...options
  } = command ?? {}
  return (
    <div className="space-y-5">
      <Properties
        items={[
          ["Command", name],
          ["Initiator", initiator],
          ["Started", <Time value={startTime} />],
          ["Finished", <Time value={endTime} />],
          ["Duration", duration(startTime, endTime)],
          [
            "Cluster ID",
            result.clusterInfo?.clusterId || run.targetKey.clusterId,
          ],
          ["Source URL", git?.url],
          ["Subdirectory", git?.subDir],
          ["Commit", git?.commit],
          ["Working tree", git && (git.dirty ? "Dirty" : "Clean")],
          [
            "Object counts",
            <span>
              {run.renderedObjects ?? 0} rendered · {run.remoteObjects ?? 0}{" "}
              remote · {run.appliedObjects ?? 0} applied ·{" "}
              {run.appliedHookObjects ?? 0} hooks
            </span>,
          ],
          ["Changes", <RunCounts run={run} />],
          [
            "Messages",
            `${run.errors?.length ?? 0} errors · ${run.warnings?.length ?? 0} warnings`,
          ],
        ]}
      />
      <details className="group/execution rounded-lg border">
        <summary className="recorded-toggle min-h-11 text-sm text-foreground">
          <ChevronRight
            aria-hidden="true"
            className="size-4 group-open/execution:rotate-90"
          />{" "}
          Execution details
        </summary>
        <div className="space-y-4 border-t px-4 py-3.5">
          <Properties
            items={[
              ["Object hash", result.renderedObjectsHash],
              ["Result ID", result.id],
              ["Reconcile ID", result.reconcileId],
            ]}
          />
          {git?.ref != null && (
            <section className="space-y-2">
              <Heading title="Source ref" />
              <Yaml value={git.ref} />
            </section>
          )}
          <section className="space-y-2">
            <Heading title="Execution options" />
            <Yaml
              value={{
                ...options,
                ...(args != null && { args }),
                ...(images?.length && { images }),
              }}
            />
          </section>
          <section className="space-y-2">
            <Heading title="Resolved target" />
            <Yaml value={result.target} />
          </section>
          {result.overridesPatch != null && (
            <section className="space-y-2">
              <Heading title="Overrides" />
              <Yaml value={result.overridesPatch} />
            </section>
          )}
        </div>
      </details>
    </div>
  )
}

function ObjectManifest({
  result,
  ref_,
  version,
}: {
  result: CommandResult
  ref_: ObjectRef
  version: ObjectVersion
}) {
  const load = useCallback(
    (signal: AbortSignal) => getObject(result, ref_, version, signal),
    [result, ref_, version]
  )
  const state = useLoad(`${result.id}/${refKey(ref_)}/${version}`, load)
  if (state?.error)
    return (
      <ErrorNotice title="Could not load manifest">{state.error}</ErrorNotice>
    )
  if (!state)
    return (
      <p role="status" className="text-sm text-muted-foreground">
        Loading manifest…
      </p>
    )
  return state.value ? (
    <ManifestView
      key={`${refKey(ref_)}/${version}`}
      manifest={state.value as Manifest}
      initialStatus
    />
  ) : (
    <Empty>No manifest was stored for this version.</Empty>
  )
}
function ObjectDetails({
  result,
  object,
  admin,
  errorList,
  warningList,
}: {
  result: CommandResult
  object: ResultObject
  admin: boolean
  errorList?: CommandResult["errors"]
  warningList?: CommandResult["warnings"]
}) {
  const [params, setParams] = useSearchParams()
  const versions = (["rendered", "remote", "applied"] as const).filter(
    (version) =>
      object[version] &&
      (admin ||
        (version === "rendered" &&
          !(!object.ref.group && object.ref.kind === "Secret")))
  )
  const version =
    versions.find((version) => params.get("version") === version) ?? versions[0]
  const annotations = versions.includes("rendered")
    ? (object.rendered as Manifest | undefined)?.metadata?.annotations
    : undefined
  return (
    <div className="space-y-4">
      <Heading title={refLabel(object.ref)}>
        {[object.ref.group, object.ref.version].filter(Boolean).join("/")}
      </Heading>
      {annotations &&
        Object.keys(annotations).some((key) =>
          key.startsWith("kluctl.io/")
        ) && (
          <Properties
            items={Object.entries(annotations).filter(([key]) =>
              key.startsWith("kluctl.io/")
            )}
          />
        )}
      {(!!errorList?.length || !!warningList?.length) && (
        <Messages errors={errorList} warnings={warningList} />
      )}
      {categoriesOf(object).length > 0 && <DiffView object={object} />}
      {version ? (
        <div className="space-y-2">
          <Select
            value={version}
            onValueChange={(value) =>
              setParams(
                (previous) => {
                  const next = new URLSearchParams(previous)
                  next.set("version", value)
                  return next
                },
                { preventScrollReset: true, replace: true }
              )
            }
          >
            <SelectTrigger aria-label="Manifest version">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {versions.map((value) => (
                <SelectItem key={value} value={value}>
                  {value[0].toUpperCase() + value.slice(1)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <ObjectManifest
            key={version}
            result={result}
            ref_={object.ref}
            version={version}
          />
        </div>
      ) : (
        <Empty>No manifest is available for your role.</Empty>
      )}
    </div>
  )
}

type TreeNode = {
  id: string
  title: string
  config?: unknown
  vars?: VarsSource
  ref?: ObjectRef
  children?: TreeNode[]
}
function resourceTree(
  result: CommandResult,
  shortNames: ShortName[]
): TreeNode {
  const used = new Set<string>()
  const abbreviations = new Map(
    shortNames.map(({ group, kind, shortName }) => [
      `${group ?? ""}/${kind}`,
      shortName,
    ])
  )
  const resource = (ref: ObjectRef): TreeNode => {
    used.add(refKey(ref))
    const short = abbreviations.get(`${ref.group ?? ""}/${ref.kind}`)
    return {
      id: refKey(ref),
      ref,
      title: short
        ? `${short} ${ref.namespace ? `${ref.namespace}/` : ""}${ref.name}`
        : refLabel(ref),
    }
  }
  const variables = (
    sources: VarsSource[] | undefined,
    path: string
  ): TreeNode[] => {
    if (!sources?.length) return []
    const names = [
      "values",
      "file",
      "git",
      "gitFiles",
      "clusterConfigMap",
      "clusterSecret",
      "clusterObject",
      "systemEnvVars",
      "http",
      "awsSecretsManager",
      "gcpSecretManager",
      "vault",
      "azureKeyVault",
    ] as const
    return [
      {
        id: `${path}/vars`,
        title: "Variables",
        children: sources.map((vars, index) => ({
          id: `${path}/vars/${index}`,
          title: `${index + 1}. ${names.find((key) => vars[key] != null) ?? "Variable source"}`,
          vars,
        })),
      },
    ]
  }
  function project(
    {
      vars,
      deployments,
      ...config
    }: Omit<DeploymentProjectConfig, "convertValues">,
    id: string,
    title: string
  ): TreeNode {
    return {
      id,
      title,
      config,
      children: [
        ...variables(vars, id),
        ...(deployments ?? []).map((value, index) =>
          item(value, `${id}/${index}`)
        ),
      ],
    }
  }
  function item(
    { renderedInclude, renderedObjects, vars, ...config }: DeploymentItemConfig,
    id: string
  ): TreeNode {
    return {
      id,
      title:
        config.path ??
        config.include ??
        config.git?.url ??
        config.oci?.url ??
        "Deployment item",
      config,
      children: [
        ...variables(vars, id),
        ...(renderedInclude
          ? [project(renderedInclude, `${id}/include`, "Included deployment")]
          : []),
        ...(renderedObjects ?? []).map(resource),
      ],
    }
  }
  const root = project(result.deployment ?? {}, "root", "Rendered deployment")
  const deleted = (result.objects ?? []).filter((object) => object.deleted)
  const orphaned = (result.objects ?? []).filter((object) => object.orphan)
  if (deleted.length)
    root.children!.push({
      id: "deleted",
      title: "Deleted resources",
      children: deleted.map((object) => resource(object.ref)),
    })
  if (orphaned.length)
    root.children!.push({
      id: "orphaned",
      title: "Orphaned resources",
      children: orphaned.map((object) => resource(object.ref)),
    })
  const remaining = (result.objects ?? []).filter(
    (object) => !used.has(refKey(object.ref))
  )
  if (remaining.length)
    root.children!.push({
      id: "other",
      title: "Other resources",
      children: remaining.map((object) => resource(object.ref)),
    })
  return root
}
function TreeBranch({
  node,
  selected,
  nodeLink,
}: {
  node: TreeNode
  selected: string
  nodeLink: (id: string) => string
}) {
  const active = node.id === selected
  const linked = (
    <Link
      preventScrollReset
      to={nodeLink(node.id)}
      aria-current={active ? "true" : undefined}
      title={node.ref ? refLabel(node.ref) : node.title}
      className={`inline-block max-w-full py-0.5 font-medium break-all hover:underline ${active ? "text-primary underline" : "text-foreground"}`}
    >
      {node.title}
    </Link>
  )
  if (!node.children?.length) return <li className="py-0.5 pl-3">{linked}</li>
  function contains(child: TreeNode): boolean {
    return child.id === selected || !!child.children?.some(contains)
  }
  return (
    <li className="py-0.5">
      <details
        open={
          node.id === "root" || (selected !== "root" && contains(node))
            ? true
            : undefined
        }
      >
        <summary className="cursor-pointer">{linked}</summary>
        <ul className="ml-1.5 border-l pl-2">
          {node.children.map((child, index) => (
            <TreeBranch
              key={`${child.id}-${index}`}
              node={child}
              selected={selected}
              nodeLink={nodeLink}
            />
          ))}
        </ul>
      </details>
    </li>
  )
}
function Resources({
  result,
  admin,
  shortNames,
  nodeLink,
}: {
  result: CommandResult
  admin: boolean
  shortNames: ShortName[]
  nodeLink: (id: string) => string
}) {
  const [params] = useSearchParams()
  const selected = params.get("node") ?? "root"
  const { tree, nodes, objects, errors, warnings } = useMemo(() => {
    const tree = resourceTree(result, shortNames)
    const nodes = new Map<string, TreeNode>()
    function visit(node: TreeNode) {
      nodes.set(node.id, node)
      node.children?.forEach(visit)
    }
    visit(tree)
    const groupMessages = (messages?: CommandResult["errors"]) => {
      const grouped = new Map<string, NonNullable<CommandResult["errors"]>>()
      for (const message of messages ?? []) {
        if (!message.ref) continue
        const key = refKey(message.ref)
        if (!grouped.has(key)) grouped.set(key, [])
        grouped.get(key)!.push(message)
      }
      return grouped
    }
    return {
      tree,
      nodes,
      objects: new Map(
        result.objects?.map((object) => [refKey(object.ref), object])
      ),
      errors: groupMessages(result.errors),
      warnings: groupMessages(result.warnings),
    }
  }, [result, shortNames])
  const node = nodes.get(selected)
  const object = objects.get(selected)
  let content
  if (!node) content = <Empty>This item is not part of the saved run.</Empty>
  else if (object)
    content = (
      <ObjectDetails
        key={selected}
        result={result}
        object={object}
        admin={admin}
        errorList={errors.get(selected)}
        warningList={warnings.get(selected)}
      />
    )
  else if (node.vars) {
    const { renderedSensitive, sensitive, renderedVars, ...source } = node.vars
    const restricted = renderedSensitive || sensitive
    content = (
      <div className="space-y-3">
        <Heading title={node.title} />
        <Yaml value={{ ...source, ...(sensitive != null && { sensitive }) }} />
        <Heading title="Rendered values" />
        {restricted && (
          <p className="text-sm text-amber-800 dark:text-amber-300">
            {admin
              ? "These values are marked sensitive."
              : "Sensitive values are only available to admins."}
          </p>
        )}
        {(!restricted || admin) && <Yaml value={renderedVars} />}
      </div>
    )
  } else
    content = (
      <div className="space-y-3">
        <Heading title={node.ref ? refLabel(node.ref) : node.title} />
        {node.config && Object.keys(node.config).length ? (
          <Yaml value={node.config} />
        ) : (
          <Empty>Select a child item to inspect its details.</Empty>
        )}
      </div>
    )
  return (
    <div className="grid min-w-0 gap-4 xl:grid-cols-[minmax(10rem,1fr)_minmax(0,2fr)]">
      <nav
        aria-label="Rendered deployment tree"
        className="max-h-[30rem] overflow-auto rounded-md border bg-muted/25 p-2.5 text-sm"
      >
        <ul>
          <TreeBranch node={tree} selected={selected} nodeLink={nodeLink} />
        </ul>
      </nav>
      <div className="min-w-0">{content}</div>
    </div>
  )
}

export function RunDetail({
  run,
  admin,
  logs,
  shortNames,
}: {
  run: CommandResultSummary
  admin: boolean
  logs: boolean
  shortNames: ShortName[]
}) {
  const [params] = useSearchParams()
  const { pathname } = useLocation()
  const load = useCallback(
    (signal: AbortSignal) => getResult(run.id, signal),
    [run.id]
  )
  const state = useLoad(run.id, load)
  if (state?.error)
    return (
      <ErrorNotice title="Could not load the run">{state.error}</ErrorNotice>
    )
  if (!state?.value)
    return (
      <p role="status" className="text-sm text-muted-foreground">
        Loading run details…
      </p>
    )
  const result = state.value
  function nodeLink(id: string) {
    const next = new URLSearchParams(params)
    next.set("view", "resources")
    next.set("node", id)
    next.delete("version")
    next.delete("full")
    return `${pathname}?${next}`
  }
  const tabs = [
    {
      id: "summary",
      label: "Summary",
      content: (
        <TabsContent value="summary">
          <RunSummary result={result} run={run} />
        </TabsContent>
      ),
    },
    {
      id: "changes",
      label: "Changes",
      content: (
        <TabsContent value="changes">
          <Changes
            key={run.id}
            objects={result.objects}
            openResource={(ref) =>
              result.objects?.some(
                (object) => refKey(object.ref) === refKey(ref)
              ) && (
                <Link
                  className="text-sm underline"
                  preventScrollReset
                  to={nodeLink(refKey(ref))}
                >
                  Inspect resource
                </Link>
              )
            }
          />
        </TabsContent>
      ),
    },
    {
      id: "resources",
      label: "Resources",
      content: (
        <TabsContent value="resources">
          <Resources
            result={result}
            admin={admin}
            shortNames={shortNames}
            nodeLink={nodeLink}
          />
        </TabsContent>
      ),
    },
    {
      id: "messages",
      label: "Messages",
      content: (
        <TabsContent value="messages">
          <Messages errors={result.errors} warnings={result.warnings} />
        </TabsContent>
      ),
    },
  ]
  if (logs && result.kluctlDeployment && result.reconcileId)
    tabs.push({
      id: "logs",
      label: "Logs",
      content: (
        <TabsContent value="logs">
          <Logs
            key={result.reconcileId}
            query={{
              cluster: result.kluctlDeployment.clusterId,
              name: result.kluctlDeployment.name,
              namespace: result.kluctlDeployment.namespace,
              reconcileId: result.reconcileId,
            }}
          />
        </TabsContent>
      ),
    })
  const view =
    params.get("view") ?? (params.get("full") === "1" ? "resources" : "summary")
  return (
    <QueryTabs
      name="view"
      value={tabs.some((tab) => tab.id === view) ? view : "summary"}
      tabs={tabs}
    />
  )
}
