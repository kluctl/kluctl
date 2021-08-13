import threading


class ThreadSafeCache:
    def __init__(self):
        self.lock = threading.Lock()
        self.cache = {}

    def get(self, key, func):
        with self.lock:
            entry = self.cache.setdefault(key, {
                "done": False,
                "value": None,
                "exception": None
            })

            def do_ret():
                if entry["exception"] is not None:
                    raise entry["exception"]
                return entry["value"]

            if entry["done"]:
                return do_ret()

            try:
                entry["value"] = func()
            except Exception as e:
                entry["exception"] = e
            entry["done"] = True
            return do_ret()

class ThreadSafeMultiCache:
    def __init__(self):
        self.cache = ThreadSafeCache()

    def get(self, cache_key, key, func):
        cache = self.cache.get(cache_key, lambda: ThreadSafeCache())
        return cache.get(key, func)
