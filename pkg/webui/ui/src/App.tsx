import {
  lazy,
  Suspense,
  useEffect,
  useMemo,
  useRef,
  useState,
  type FormEvent,
} from "react"
import { ArrowRight, BookOpen, CloudOff, LogOut, Moon, Sun } from "lucide-react"
import {
  HashRouter,
  Link,
  NavLink,
  Navigate,
  Route,
  Routes,
  useLocation,
  useParams,
  useSearchParams,
} from "react-router-dom"
import {
  base,
  endpoint,
  get,
  getShortNames,
  HttpError,
  post,
  snapshot,
  type Capabilities,
  type User,
} from "./api"
import {
  deploymentId,
  findLinkedTarget,
  findTargetById,
  targetsFrom,
  type TargetRow,
} from "./domain"
import { useInventory, useLoad } from "./hooks"
import { Empty, ErrorNotice } from "./components/detail-primitives"
import { Button } from "./components/ui/button"
import { Input } from "./components/ui/input"

const Overview = lazy(() =>
  import("./components/overview").then((module) => ({
    default: module.Overview,
  }))
)
const TargetDetail = lazy(() =>
  import("./components/target-detail").then((module) => ({
    default: module.TargetDetail,
  }))
)

const offlineUser: User = { username: "admin", isAdmin: true }
const offlineCapabilities: Capabilities = {
  authEnabled: false,
  staticLoginEnabled: false,
  oidcEnabled: false,
}
const noShortNames: never[] = []

function Logo() {
  return (
    <span className="flex items-center gap-2.5 text-lg font-semibold tracking-tight">
      <span className="brand-icon">
        <img src={endpoint("kluctl-logo.svg")} alt="" className="size-5" />
      </span>
      Kluctl
    </span>
  )
}
function useTheme() {
  const [dark, setDark] = useState(() => {
    const saved = localStorage.getItem("kluctl-theme")
    return saved
      ? saved === "dark"
      : window.matchMedia("(prefers-color-scheme: dark)").matches
  })
  useEffect(() => {
    document.documentElement.classList.toggle("dark", dark)
    document.documentElement.style.colorScheme = dark ? "dark" : "light"
  }, [dark])
  function toggle() {
    localStorage.setItem("kluctl-theme", dark ? "light" : "dark")
    setDark(!dark)
  }
  return { dark, toggle }
}
function Login({
  info,
  onLogin,
}: {
  info: Capabilities
  onLogin: (user: User) => void
}) {
  const [username, setUsername] = useState("")
  const [password, setPassword] = useState("")
  const [pending, setPending] = useState(false)
  const [error, setError] = useState(
    new URLSearchParams(window.location.search).get("login_error") ?? ""
  )
  async function submit(event: FormEvent) {
    event.preventDefault()
    setError("")
    setPending(true)
    try {
      await post("auth/staticLogin", { username, password }, false)
      onLogin(await get<User>("auth/user", undefined, undefined, false))
      const url = new URL(window.location.href)
      url.searchParams.delete("login_error")
      window.history.replaceState(window.history.state, "", url)
    } catch (error) {
      setError(
        error instanceof HttpError && error.status === 401
          ? "Invalid username or password."
          : (error as Error).message
      )
    } finally {
      setPending(false)
    }
  }
  return (
    <main className="mx-auto w-full max-w-sm space-y-5 px-5 py-20">
      <Logo />
      <div>
        <h1 className="text-2xl font-semibold">Sign in</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Sign in to view your deployment targets.
        </p>
      </div>
      {error && <ErrorNotice title="Sign-in failed">{error}</ErrorNotice>}
      {info.staticLoginEnabled && (
        <form onSubmit={submit} className="space-y-4">
          <label className="block space-y-1.5 text-sm font-medium">
            Username
            <Input
              name="username"
              autoComplete="username"
              value={username}
              onChange={(event) => setUsername(event.target.value)}
              required
            />
          </label>
          <label className="block space-y-1.5 text-sm font-medium">
            Password
            <Input
              name="password"
              type="password"
              autoComplete="current-password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              required
            />
          </label>
          <Button className="w-full" type="submit" disabled={pending}>
            {pending ? "Signing in…" : "Sign in"}
          </Button>
        </form>
      )}
      {info.oidcEnabled && (
        <Button
          asChild
          variant={info.staticLoginEnabled ? "outline" : "default"}
          className="w-full"
        >
          <a href={endpoint("auth/login")}>
            Continue with {info.oidcName || "OIDC"}
            <ArrowRight />
          </a>
        </Button>
      )}
      {!info.staticLoginEnabled && !info.oidcEnabled && (
        <Empty>No sign-in method is configured.</Empty>
      )}
    </main>
  )
}

function Entry({ rows, loaded }: { rows: TargetRow[]; loaded: boolean }) {
  const [hashParams] = useSearchParams()
  const params = new URLSearchParams(window.location.search)
  hashParams.forEach((value, key) => params.set(key, value))
  const path = findLinkedTarget(rows, params)
  if (path) return <Navigate to={path} replace />
  return loaded ? (
    <div className="space-y-3">
      <Empty>No available target or run matches this link.</Empty>
      <Link className="underline" to="/targets">
        View projects
      </Link>
    </div>
  ) : (
    <p role="status">Finding the linked target…</p>
  )
}
function TargetRoute({
  rows,
  loaded,
  user,
  logs,
  shortNames,
  inventoryPath,
}: {
  rows: TargetRow[]
  loaded: boolean
  user: User
  logs: boolean
  shortNames: Awaited<ReturnType<typeof getShortNames>>
  inventoryPath: string
}) {
  const { id, section = "summary", runId } = useParams()
  // A deployment can be bookmarked while its controller is still preparing it.
  const row = findTargetById(rows, id)
  if (row)
    return (
      <Suspense fallback={<p role="status">Loading target details…</p>}>
        <TargetDetail
          key={
            row.deployment && row.instance ? deploymentId(row.instance) : row.id
          }
          row={row}
          inventoryPath={inventoryPath}
          section={section}
          runId={runId}
          admin={user.isAdmin}
          logs={logs}
          shortNames={shortNames}
        />
      </Suspense>
    )
  return loaded ? (
    <div className="space-y-3">
      <Empty>This deployment or recorded target is not available.</Empty>
      <Link className="underline" to={inventoryPath}>
        View projects
      </Link>
    </div>
  ) : (
    <p role="status">Loading target…</p>
  )
}
function ScrollOnPageChange() {
  const { pathname } = useLocation()
  const previous = useRef(pathname)
  useEffect(() => {
    if (pathname !== previous.current) {
      // Preserve position when stepping between two runs in the same history.
      const history = /^(\/targets\/[^/]+\/results)\/[^/]+$/
      const oldRun = previous.current.match(history)
      if (!oldRun || oldRun[1] !== pathname.match(history)?.[1])
        window.scrollTo(0, 0)
      if (pathname === "/targets" || previous.current === "/targets") {
        document
          .getElementById("workspace-content")
          ?.focus({ preventScroll: true })
      }
      previous.current = pathname
    }
  }, [pathname])
  return null
}

function WorkspaceRoutes(
  props: Omit<Parameters<typeof TargetRoute>[0], "inventoryPath">
) {
  const { pathname, search } = useLocation()
  const [lastInventory, setLastInventory] = useState("/targets")
  // Keep the filters when returning from any detail tab, without putting
  // navigation context in a shareable deployment/run link.
  const inventoryPath =
    pathname === "/targets" ? pathname + search : lastInventory
  if (inventoryPath !== lastInventory) setLastInventory(inventoryPath)
  const target = <TargetRoute {...props} inventoryPath={inventoryPath} />
  const { rows, loaded } = props
  return (
    <Routes>
      <Route path="/" element={<Entry rows={rows} loaded={loaded} />} />
      <Route
        path="/targets"
        element={
          <Suspense fallback={<p role="status">Loading projects…</p>}>
            <Overview rows={rows} loaded={loaded} />
          </Suspense>
        }
      />
      <Route path="/targets/:id" element={target} />
      <Route path="/targets/:id/:section" element={target} />
      <Route path="/targets/:id/:section/:runId" element={target} />
      <Route path="*" element={<Navigate to="/targets" replace />} />
    </Routes>
  )
}

function Workspace({
  user,
  info,
  theme,
}: {
  user: User
  info: Capabilities
  theme: ReturnType<typeof useTheme>
}) {
  const { inventory, connection, loaded, error } = useInventory()
  const rows = useMemo(() => targetsFrom(inventory), [inventory])
  const names = useLoad("shortnames", getShortNames)
  const logout = `${endpoint("auth/logout")}?${new URLSearchParams({ returnUrl: base.href })}`
  const state =
    connection === "snapshot"
      ? "Snapshot"
      : connection === "live"
        ? "Live"
        : connection === "offline"
          ? "Reconnecting"
          : connection === "error"
            ? "Unavailable"
            : "Connecting"
  return (
    <HashRouter>
      <ScrollOnPageChange />
      <a
        href="#workspace-content"
        className="sr-only z-50 rounded-md bg-card p-3 text-primary focus:not-sr-only focus:fixed focus:top-2 focus:left-2"
        onClick={(event) => {
          // A normal fragment link would change the HashRouter route.
          event.preventDefault()
          document.getElementById("workspace-content")?.focus()
        }}
      >
        Skip to content
      </a>
      <header className="border-b bg-card">
        <div className="workspace-width flex min-h-15 items-center justify-between gap-3">
          <div className="flex min-w-0 items-center gap-4 sm:gap-6">
            <Link to="/targets" aria-label="Kluctl home">
              <Logo />
            </Link>
            <nav
              aria-label="Workspace"
              className="hidden items-center border-l pl-5 min-[360px]:flex sm:pl-6"
            >
              <NavLink
                to="/targets"
                className="rounded-md px-2.5 py-1.5 text-sm font-medium text-muted-foreground transition-colors hover:bg-muted aria-[current=page]:bg-accent aria-[current=page]:text-primary"
              >
                Projects
              </NavLink>
            </nav>
          </div>
          <div className="flex shrink-0 items-center gap-1.5 sm:gap-3">
            <span
              role="status"
              title={
                connection === "snapshot" ? "Read-only snapshot" : undefined
              }
              className="signal-ink inline-flex items-center gap-1.5 rounded-full border bg-background px-2.5 py-1 text-xs font-medium"
              data-tone={
                connection === "live"
                  ? "good"
                  : connection === "offline" || connection === "error"
                    ? "warn"
                    : "muted"
              }
            >
              <span
                aria-hidden="true"
                className="size-1.5 rounded-full bg-current"
              />
              {state}
            </span>
            <Button
              variant="ghost"
              size="icon"
              className="hidden text-muted-foreground sm:inline-flex"
              asChild
            >
              <a
                href="https://kluctl.io/docs/webui/"
                target="_blank"
                rel="noreferrer"
                aria-label="Web UI documentation"
                title="Web UI documentation"
              >
                <BookOpen aria-hidden="true" />
              </a>
            </Button>
            <Button
              variant="ghost"
              size="icon"
              className="text-muted-foreground"
              aria-label={theme.dark ? "Use light theme" : "Use dark theme"}
              title={theme.dark ? "Use light theme" : "Use dark theme"}
              onClick={theme.toggle}
            >
              {theme.dark ? (
                <Sun aria-hidden="true" />
              ) : (
                <Moon aria-hidden="true" />
              )}
            </Button>
            {info.authEnabled && (
              <>
                <span
                  className="hidden max-w-40 truncate text-sm text-muted-foreground lg:inline"
                  title={user.username}
                >
                  {user.username} · {user.isAdmin ? "Admin" : "Viewer"}
                </span>
                <Button
                  variant="ghost"
                  size="icon"
                  className="text-muted-foreground"
                  asChild
                >
                  <a href={logout} aria-label="Sign out" title="Sign out">
                    <LogOut aria-hidden="true" />
                  </a>
                </Button>
              </>
            )}
          </div>
        </div>
      </header>
      <main
        id="workspace-content"
        tabIndex={-1}
        className="workspace-width space-y-4 py-5 outline-none sm:py-7"
      >
        {connection === "offline" && (
          <p
            role="status"
            data-tone="warn"
            className="signal-ink flex items-center gap-2 rounded-lg border bg-card px-3 py-2.5 text-sm"
          >
            <CloudOff aria-hidden="true" className="size-4 shrink-0" />
            {loaded
              ? "Connection lost. Showing the last known state while reconnecting."
              : "Waiting to reconnect to Kluctl."}
          </p>
        )}
        {error ? (
          <ErrorNotice title="Could not load snapshot">{error}</ErrorNotice>
        ) : (
          <WorkspaceRoutes
            rows={rows}
            loaded={loaded}
            user={user}
            logs={!snapshot && (info.logsEnabled ?? true)}
            shortNames={names?.value ?? noShortNames}
          />
        )}
      </main>
    </HashRouter>
  )
}

export default function App() {
  const [info, setInfo] = useState<Capabilities | undefined>(
    snapshot ? offlineCapabilities : undefined
  )
  const [user, setUser] = useState<User | null | undefined>(
    snapshot ? offlineUser : undefined
  )
  const [error, setError] = useState("")
  const [attempt, setAttempt] = useState(0)
  const theme = useTheme()
  useEffect(() => {
    if (snapshot) return
    const controller = new AbortController()
    const expire = () => setUser(null)
    window.addEventListener("session-expired", expire)
    async function start() {
      try {
        const info = await get<Capabilities>(
          "auth/info",
          undefined,
          controller.signal
        )
        if (controller.signal.aborted) return
        setInfo(info)
        try {
          const current = await get<User>(
            "auth/user",
            undefined,
            controller.signal
          )
          if (!controller.signal.aborted) setUser(current)
        } catch (error) {
          if (error instanceof HttpError && error.status === 401) return
          throw error
        }
      } catch (error) {
        if (!controller.signal.aborted) setError((error as Error).message)
      }
    }
    void start()
    return () => {
      window.removeEventListener("session-expired", expire)
      controller.abort()
    }
  }, [attempt])
  if (error)
    return (
      <main className="mx-auto max-w-md space-y-4 px-4 py-12">
        <Logo />
        <ErrorNotice title="Could not connect to Kluctl">{error}</ErrorNotice>
        <Button
          onClick={() => {
            setError("")
            setAttempt((n) => n + 1)
          }}
        >
          Try again
        </Button>
      </main>
    )
  if (!info || user === undefined)
    return (
      <p role="status" className="p-6 text-sm text-muted-foreground">
        Connecting to Kluctl…
      </p>
    )
  if (!user) return <Login info={info} onLogin={setUser} />
  return <Workspace user={user} info={info} theme={theme} />
}
