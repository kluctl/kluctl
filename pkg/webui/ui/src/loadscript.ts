
interface loadResult {
    status: boolean
    message?: string
}
const loadedScripts = new Map<string, Promise<loadResult>>()

export async function loadScript(fileUrl: string) {
    let p = loadedScripts.get(fileUrl)
    if (!p) {
        p = loadScript2(fileUrl)
        loadedScripts.set(fileUrl, p)
    }

    const r = await p
    if (!r.status) {
        throw new Error(r.message)
    }
}

async function loadScript2(fileUrl: string): Promise<loadResult> {
    const r = await fetch(fileUrl)

    const contentType = r.headers.get("Content-Type")
    if (!contentType) {
        return {
            status: false,
            message: "not javascript"
        }
    }

    console.log("ct="+contentType)
    if (!contentType.startsWith("application/javascript")) {
        console.log("error")
        return {
            status: false,
            message: "not javascript"
        }
    }

    const p = new Promise<loadResult>((resolve, reject) => {
        try {
            const scriptEle = document.createElement("script");
            scriptEle.type = "application/javascript";
            scriptEle.async = true;
            scriptEle.src = fileUrl;

            scriptEle.addEventListener("load", (ev) => {
                resolve({ status: true });
            });

            scriptEle.addEventListener("error", (ev) => {
                resolve({
                    status: false,
                    message: `failed to load the script`
                });
            });

            document.body.appendChild(scriptEle);
        } catch (error) {
            reject(error);
        }
    });

    return p
}