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
    function connect() {
      const ws = new WebSocket(socketUrl("api/events"))
      socket = ws
      let first = true
      const markLoaded = () => {
        setConnection("live")
        setLoaded(true)
      }
      ws.onopen = () => {
        // Older servers don't send an initial frame if the result store is empty.
        fallback = setTimeout(() => {
          if (!disposed && first) {
            setInventory(emptyInventory())
            markLoaded()
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
          setInventory((previous) =>
            applyEvents(replace ? emptyInventory() : previous, events)
          )
          markLoaded()
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
