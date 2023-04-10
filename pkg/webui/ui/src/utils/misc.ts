export function getLastPathElement(url?: string): string | undefined {
    if (url === undefined) {
        return undefined
    }
    if (!url) {
        return ""
    }
    const s= url.split("/")
    return s[s.length - 1]
}

export function sleep(ms: number) {
    return new Promise(resolve => setTimeout(resolve, ms));
}
