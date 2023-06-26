export class GitRef {
    branch?: string;
    tag?: string;
    commit?: string;

    ref?: string

    constructor(source: any = {}) {
        if ('string' === typeof source) {
            // this is needed to parse legacy string refs
            this.ref = source
            return
        }

        this.branch = source["branch"];
        this.tag = source["tag"];
        this.commit = source["commit"];
    }
}
