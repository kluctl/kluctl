"use strict";
(self["webpackChunkkluctl_webui"] = self["webpackChunkkluctl_webui"] || []).push([[787],{

/***/ 787:
/***/ (function(__unused_webpack_module, __webpack_exports__, __webpack_require__) {

__webpack_require__.r(__webpack_exports__);
/* harmony export */ __webpack_require__.d(__webpack_exports__, {
/* harmony export */   getCLS: function() { return /* binding */ h; },
/* harmony export */   getFCP: function() { return /* binding */ d; },
/* harmony export */   getFID: function() { return /* binding */ L; },
/* harmony export */   getLCP: function() { return /* binding */ F; },
/* harmony export */   getTTFB: function() { return /* binding */ P; }
/* harmony export */ });
var e,
  t,
  n,
  i,
  r = function r(e, t) {
    return {
      name: e,
      value: void 0 === t ? -1 : t,
      delta: 0,
      entries: [],
      id: "v2-".concat(Date.now(), "-").concat(Math.floor(8999999999999 * Math.random()) + 1e12)
    };
  },
  a = function a(e, t) {
    try {
      if (PerformanceObserver.supportedEntryTypes.includes(e)) {
        if ("first-input" === e && !("PerformanceEventTiming" in self)) return;
        var n = new PerformanceObserver(function (e) {
          return e.getEntries().map(t);
        });
        return n.observe({
          type: e,
          buffered: !0
        }), n;
      }
    } catch (e) {}
  },
  o = function o(e, t) {
    var n = function n(i) {
      "pagehide" !== i.type && "hidden" !== document.visibilityState || (e(i), t && (removeEventListener("visibilitychange", n, !0), removeEventListener("pagehide", n, !0)));
    };
    addEventListener("visibilitychange", n, !0), addEventListener("pagehide", n, !0);
  },
  u = function u(e) {
    addEventListener("pageshow", function (t) {
      t.persisted && e(t);
    }, !0);
  },
  c = function c(e, t, n) {
    var i;
    return function (r) {
      t.value >= 0 && (r || n) && (t.delta = t.value - (i || 0), (t.delta || void 0 === i) && (i = t.value, e(t)));
    };
  },
  f = -1,
  s = function s() {
    return "hidden" === document.visibilityState ? 0 : 1 / 0;
  },
  m = function m() {
    o(function (e) {
      var t = e.timeStamp;
      f = t;
    }, !0);
  },
  v = function v() {
    return f < 0 && (f = s(), m(), u(function () {
      setTimeout(function () {
        f = s(), m();
      }, 0);
    })), {
      get firstHiddenTime() {
        return f;
      }
    };
  },
  d = function d(e, t) {
    var n,
      i = v(),
      o = r("FCP"),
      f = function f(e) {
        "first-contentful-paint" === e.name && (m && m.disconnect(), e.startTime < i.firstHiddenTime && (o.value = e.startTime, o.entries.push(e), n(!0)));
      },
      s = window.performance && performance.getEntriesByName && performance.getEntriesByName("first-contentful-paint")[0],
      m = s ? null : a("paint", f);
    (s || m) && (n = c(e, o, t), s && f(s), u(function (i) {
      o = r("FCP"), n = c(e, o, t), requestAnimationFrame(function () {
        requestAnimationFrame(function () {
          o.value = performance.now() - i.timeStamp, n(!0);
        });
      });
    }));
  },
  p = !1,
  l = -1,
  h = function h(e, t) {
    p || (d(function (e) {
      l = e.value;
    }), p = !0);
    var n,
      i = function i(t) {
        l > -1 && e(t);
      },
      f = r("CLS", 0),
      s = 0,
      m = [],
      v = function v(e) {
        if (!e.hadRecentInput) {
          var t = m[0],
            i = m[m.length - 1];
          s && e.startTime - i.startTime < 1e3 && e.startTime - t.startTime < 5e3 ? (s += e.value, m.push(e)) : (s = e.value, m = [e]), s > f.value && (f.value = s, f.entries = m, n());
        }
      },
      h = a("layout-shift", v);
    h && (n = c(i, f, t), o(function () {
      h.takeRecords().map(v), n(!0);
    }), u(function () {
      s = 0, l = -1, f = r("CLS", 0), n = c(i, f, t);
    }));
  },
  T = {
    passive: !0,
    capture: !0
  },
  y = new Date(),
  g = function g(i, r) {
    e || (e = r, t = i, n = new Date(), w(removeEventListener), E());
  },
  E = function E() {
    if (t >= 0 && t < n - y) {
      var r = {
        entryType: "first-input",
        name: e.type,
        target: e.target,
        cancelable: e.cancelable,
        startTime: e.timeStamp,
        processingStart: e.timeStamp + t
      };
      i.forEach(function (e) {
        e(r);
      }), i = [];
    }
  },
  S = function S(e) {
    if (e.cancelable) {
      var t = (e.timeStamp > 1e12 ? new Date() : performance.now()) - e.timeStamp;
      "pointerdown" == e.type ? function (e, t) {
        var n = function n() {
            g(e, t), r();
          },
          i = function i() {
            r();
          },
          r = function r() {
            removeEventListener("pointerup", n, T), removeEventListener("pointercancel", i, T);
          };
        addEventListener("pointerup", n, T), addEventListener("pointercancel", i, T);
      }(t, e) : g(t, e);
    }
  },
  w = function w(e) {
    ["mousedown", "keydown", "touchstart", "pointerdown"].forEach(function (t) {
      return e(t, S, T);
    });
  },
  L = function L(n, f) {
    var s,
      m = v(),
      d = r("FID"),
      p = function p(e) {
        e.startTime < m.firstHiddenTime && (d.value = e.processingStart - e.startTime, d.entries.push(e), s(!0));
      },
      l = a("first-input", p);
    s = c(n, d, f), l && o(function () {
      l.takeRecords().map(p), l.disconnect();
    }, !0), l && u(function () {
      var a;
      d = r("FID"), s = c(n, d, f), i = [], t = -1, e = null, w(addEventListener), a = p, i.push(a), E();
    });
  },
  b = {},
  F = function F(e, t) {
    var n,
      i = v(),
      f = r("LCP"),
      s = function s(e) {
        var t = e.startTime;
        t < i.firstHiddenTime && (f.value = t, f.entries.push(e), n());
      },
      m = a("largest-contentful-paint", s);
    if (m) {
      n = c(e, f, t);
      var d = function d() {
        b[f.id] || (m.takeRecords().map(s), m.disconnect(), b[f.id] = !0, n(!0));
      };
      ["keydown", "click"].forEach(function (e) {
        addEventListener(e, d, {
          once: !0,
          capture: !0
        });
      }), o(d, !0), u(function (i) {
        f = r("LCP"), n = c(e, f, t), requestAnimationFrame(function () {
          requestAnimationFrame(function () {
            f.value = performance.now() - i.timeStamp, b[f.id] = !0, n(!0);
          });
        });
      });
    }
  },
  P = function P(e) {
    var t,
      n = r("TTFB");
    t = function t() {
      try {
        var t = performance.getEntriesByType("navigation")[0] || function () {
          var e = performance.timing,
            t = {
              entryType: "navigation",
              startTime: 0
            };
          for (var n in e) "navigationStart" !== n && "toJSON" !== n && (t[n] = Math.max(e[n] - e.navigationStart, 0));
          return t;
        }();
        if (n.value = n.delta = t.responseStart, n.value < 0 || n.value > performance.now()) return;
        n.entries = [t], e(n);
      } catch (e) {}
    }, "complete" === document.readyState ? setTimeout(t, 0) : addEventListener("load", function () {
      return setTimeout(t, 0);
    });
  };


/***/ })

}]);
//# sourceMappingURL=787.chunk.js.map?h=3227734ac7b196096e87ef1fcdf6564d8d045a68bf1ab467959f57b70d2f6709