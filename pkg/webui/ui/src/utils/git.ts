import { GitRef } from "../models-static";

export const gitRefToString = (ref?: GitRef) => {
    if (!ref) {
        return ""
    }
    if (ref.tag) {
        return "tag " + ref.tag
    } else if (ref.branch) {
        return "branch " + ref.branch
    } else if (ref.commit) {
        return "commit " + ref.commit
    } else {
        return JSON.stringify(ref)
    }
}