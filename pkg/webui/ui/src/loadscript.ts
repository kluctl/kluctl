const loadedScripts = new Map<string, any>()

export const loadScript = (fileUrl: string, async = true, type = "text/javascript") => {
    if (loadedScripts.has(fileUrl)) {
        return loadedScripts.get(fileUrl)
    }

    const p = new Promise((resolve, reject) => {
        try {
            const scriptEle = document.createElement("script");
            scriptEle.type = type;
            scriptEle.async = async;
            scriptEle.src = fileUrl;

            scriptEle.addEventListener("load", (ev) => {
                resolve({ status: true });
            });

            scriptEle.addEventListener("error", (ev) => {
                reject({
                    status: false,
                    message: `Failed to load the script ＄{FILE_URL}`
                });
            });

            document.body.appendChild(scriptEle);
        } catch (error) {
            reject(error);
        }
    });

    loadedScripts.set(fileUrl, p)

    return p
};
