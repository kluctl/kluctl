export function formatDuration(ms: number, withMs?: boolean) {
    if (ms < 0) ms = -ms;
    const time = {
        day: Math.floor(ms / 86400000),
        hour: Math.floor(ms / 3600000) % 24,
        minute: Math.floor(ms / 60000) % 60,
        second: Math.floor(ms / 1000) % 60,
        millisecond: withMs ? Math.floor(ms) % 1000 : 0
    };
    return Object.entries(time)
        .filter(val => val[1] !== 0)
        .map(val => val[1] + ' ' + (val[1] !== 1 ? val[0] + 's' : val[0]))
        .join(', ');
}

export function formatDurationShort(ms: number) {
    if (ms < 0) ms = -ms;
    const time = {
        d: Math.floor(ms / 86400000),
        h: Math.floor(ms / 3600000) % 24,
        m: Math.floor(ms / 60000) % 60,
        s: Math.floor(ms / 1000) % 60,
        ms: Math.floor(ms) % 1000
    };
    const f = Object.entries(time).find(val => val[1] > 0)
    if (f === undefined) {
        return "0s"
    }
    return f[1] + f[0]
}

export const calcAgo = (startTime: string) => {
    const t1 = new Date(startTime)
    const t2 = new Date()
    const d = t2.getTime() - t1.getTime()
    return formatDurationShort(d)
}