import functools
import zoneinfo
from datetime import datetime, timedelta

from jinja2.ext import Extension


class TimeExtension(Extension):
    def __init__(self, environment):
        super().__init__(environment)
        environment.globals["time"] = {
            "now": SimpleTime.now,
            "utcnow": SimpleTime.utcnow,
            "parse_iso": SimpleTime.parse_iso,
            "second": 1000000,
            "minute": 1000000 * 60,
            "hour": 1000000 * 60 * 60,
        }

@functools.total_ordering
class SimpleTime:
    def __init__(self, t):
        self.t = t

    @classmethod
    def now(cls):
        t = datetime.now()
        return SimpleTime(t)

    @classmethod
    def utcnow(cls):
        t = datetime.utcnow()
        return SimpleTime(t)

    @classmethod
    def parse_iso(cls, s):
        return SimpleTime(datetime.fromisoformat(s))

    def as_timezone(self, tz):
        tz = zoneinfo.ZoneInfo(tz)
        return SimpleTime(self.t.astimezone(tz))

    def weekday(self):
        return self.t.weekday()

    def hour(self):
        return self.t.hour

    def minute(self):
        return self.t.minute

    def second(self):
        return self.t.second

    def nanosecond(self):
        return self.t.microsecond * 1000

    def __str__(self):
        return self.t.isoformat()

    def __add__(self, other):
        d = timedelta(microseconds=other)
        self.t += d
        return self

    def __sub__(self, other):
        d = timedelta(microseconds=other)
        self.t -= d
        return self

    def __lt__(self, other):
        if not isinstance(other, SimpleTime):
            raise ValueError("value is not a SimpleTime")
        return self.t < other.t

    def __eq__(self, other):
        if not isinstance(other, SimpleTime):
            raise ValueError("value is not a SimpleTime")
        return self.t == other.t
