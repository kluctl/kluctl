import { sha256 } from "js-sha256";

export function buildListKey(o: any) {
    const j = JSON.stringify(o)
    return sha256(j)
}
