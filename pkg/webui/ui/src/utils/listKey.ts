import { sha256 } from "js-sha256";
import _ from "lodash";

export function buildListKey(o: any, noHash?: boolean) {
    const x = _.toPlainObject(o)
    const j = JSON.stringify(x, deterministicReplacer)
    if (noHash) {
        return j
    }
    return sha256(j)
}

const deterministicReplacer = (asd: any, v: any) =>
    typeof v !== 'object' || v === null || Array.isArray(v) ? v :
        Object.fromEntries(Object.entries(v).sort(([ka], [kb]) =>
            ka < kb ? -1 : ka > kb ? 1 : 0));
