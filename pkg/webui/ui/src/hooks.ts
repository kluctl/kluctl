import { useEffect, useState } from "react"
import { useLocation, useNavigationType } from "react-router-dom"
import { get, HttpError, snapshot, snapshotEvents, socketUrl } from "./api"
import { applyEvents, emptyInventory, type Event } from "./domain"

export type Connection =
  "connecting" | "live" | "offline" | "snapshot" | "error"
export function useInventory() {
  const [inventory, setInventory] = useState(emptyInventory)
  const [connection, setConnection] = useState<Connection>(
    snapshot ? "snapshot" : "connecting"
  )
  const [loaded, setLoaded] = useState(false)
  const [error, setError] = useState("")

  useEffect(() => {
    let disposed = false
    if (snapshot) {
      snapshotEvents()
        .then((events) => {
          if (disposed) return
          setInventory(applyEvents(emptyInventory(), events))
          setLoaded(true)
        })
        .catch((error: Error) => {
          if (disposed) return
          setError(error.message)
          setConnection("error")
        })
      return () => {
        disposed = true
      }
    }

    const lifetime = new AbortController()
    let socket: WebSocket | undefined
    let retry: ReturnType<typeof setTimeout>
    let fallback: ReturnType<typeof setTimeout>
    // Controllers can emit bursts of status updates. Apply them in batches so
    // the target list is rebuilt once per batch rather than once per frame.
    let pending: { replace: boolean; events: Event[] } | undefined
    let flush: ReturnType<typeof setTimeout> | undefined
    function apply() {
      flush = undefined
      const batch = pending
      pending = undefined
      if (disposed || !batch) return
      setInventory((previous) =>
        applyEvents(batch.replace ? emptyInventory() : previous, batch.events)
      )
      setConnection("live")
      setLoaded(true)
    }
    function enqueue(events: Event[], replace: boolean) {
      if (replace || !pending) pending = { replace, events: [...events] }
      else pending.events.push(...events)
      flush ??= setTimeout(apply, 50)
    }
    function connect() {
      const ws = new WebSocket(socketUrl("api/events"))
      socket = ws
      let first = true
      ws.onopen = () => {
        // Older servers don't send an initial frame if the result store is empty.
        fallback = setTimeout(() => {
          if (!disposed && first) {
            first = false
            enqueue([], true)
          }
        }, 2500)
      }
      ws.onmessage = (event) => {
        if (disposed) return
        try {
          const events = JSON.parse(event.data) as Event[]
          if (!Array.isArray(events)) return
          const replace = first
          first = false
          clearTimeout(fallback)
          enqueue(events, replace)
        } catch {
          /* ignore malformed frames */
        }
      }
      ws.onclose = () => {
        if (disposed) return
        clearTimeout(fallback)
        setConnection("offline")
        // A WebSocket close doesn't expose its HTTP status. Check whether the cookie expired.
        get("auth/user", undefined, lifetime.signal).catch((error: unknown) => {
          if (error instanceof HttpError && error.status === 401)
            clearTimeout(retry)
        })
        retry = setTimeout(connect, 5000)
      }
    }
    connect()
    return () => {
      disposed = true
      clearTimeout(fallback)
      clearTimeout(flush)
      clearTimeout(retry)
      lifetime.abort()
      socket?.close()
    }
  }, [])

  return { inventory, connection, loaded, error }
}

// Keep inputs responsive while the hash router applies query updates in a
// transition. Replacements are our own edits; back/forward or a new link
// resynchronizes the input with the URL.
export function useSearchDraft(
  fromUrl: string
): [string, (value: string) => void] {
  const { key } = useLocation()
  const navigation = useNavigationType()
  const [draft, setDraft] = useState({ key, value: fromUrl })
  if (navigation !== "REPLACE" && draft.key !== key) {
    setDraft({ key, value: fromUrl })
  }
  return [draft.value, (value) => setDraft({ key, value })]
}

export function useLoad<T>(
  key: string,
  load: (signal: AbortSignal) => Promise<T>
) {
  const [state, setState] = useState<{
    key: string
    value?: T
    error?: string
  }>()
  // The caller supplies the full resource identity as a key and a stable loader.
  useEffect(() => {
    const controller = new AbortController()
    load(controller.signal)
      .then((value) => {
        if (!controller.signal.aborted) setState({ key, value })
      })
      .catch((error: Error) => {
        if (!controller.signal.aborted) setState({ key, error: error.message })
      })
    return () => controller.abort()
  }, [key, load])
  return state?.key === key ? state : undefined
}
