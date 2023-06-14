import {
    CommandResult,
    CommandResultSummary,
    ObjectRef,
    ProjectKey,
    ResultObject,
    ShortName,
    TargetKey
} from "./models";
import _ from "lodash";
import React from "react";
import { Box, Typography } from "@mui/material";
import Tooltip from "@mui/material/Tooltip";
import "./staticbuild.d.ts"
import { loadScript } from "./loadscript";
import { sleep } from "./utils/misc";

const staticPath = "./staticdata"

export enum ObjectType {
    Rendered = "rendered",
    Remote = "remote",
    Applied = "applied",
}

export interface User {
    username: string
    isAdmin: boolean
}

export interface Api {
    getShortNames(): Promise<ShortName[]>
    listenUpdates(filterProject: string | undefined, filterSubDir: string | undefined, handle: (msg: any) => void): Promise<() => void>
    getResult(resultId: string): Promise<CommandResult>
    getResultSummary(resultId: string): Promise<CommandResultSummary>
    getResultObject(resultId: string, ref: ObjectRef, objectType: string): Promise<any>
    validateNow(project: ProjectKey, target: TargetKey): Promise<Response>
    reconcileNow(cluster: string, name: string, namespace: string): Promise<Response>
    deployNow(cluster: string, name: string, namespace: string): Promise<Response>
}

export async function checkStaticBuild() {
    const p = loadScript(staticPath + "/summaries.js")
    try {
        await p
        return true
    } catch (error) {
        return false
    }
}

export class RealApi implements Api {
    getToken?: (() => string);
    onUnauthorized?: () => void;
    onTokenRefresh?: (newToken: string) => void;

    constructor(getToken?: (() => string), onUnauthorized?: () => void, onTokenRefresh?: (newToken: string) => void) {
        this.getToken = getToken
        this.onUnauthorized = onUnauthorized
        this.onTokenRefresh = onTokenRefresh
    }

    setAuthorizationHeader(headers: any) {
        if (!this.getToken) {
            return
        }
        headers['Authorization'] = "Bearer " + this.getToken()
    }

    async refreshToken() {
        const headers = {
            'Accept': 'application/json',
        }
        this.setAuthorizationHeader(headers)
        const resp = await fetch("/auth/refresh", {
            method: "POST",
            headers: headers,
        })
        if (resp.status === 401) {
            if (this.onUnauthorized) {
                this.onUnauthorized()
            }
            throw Error(resp.statusText)
        }
        const j = await resp.json()
        if (this.onTokenRefresh) {
            this.onTokenRefresh(j.token)
        }
    }

    async handleErrors(response: Response, retry?: () => Promise<Response>): Promise<Response> {
        if (!response.ok) {
            if (response.status === 401) {
                if (this.getToken && retry) {
                    console.log("retrying with token refresh")
                    await this.refreshToken()
                    const newResp = await retry()
                    return await this.handleErrors(newResp)
                } else {
                    if (this.onUnauthorized) {
                        this.onUnauthorized()
                    }
                }
            }
            throw Error(response.statusText)
        }
        return response
    }

    async doGet(path: string, params?: URLSearchParams) {
        let url = path
        if (params) {
            url += "?" + params.toString()
        }
        const doFetch = () => {
            const headers = {
                'Accept': 'application/json',
            }
            this.setAuthorizationHeader(headers)
            return fetch(url, {
                method: "GET",
                headers: headers,
            })
        }
        let resp = await doFetch()
        resp = await this.handleErrors(resp, doFetch)
        return resp.json()
    }

    async doPost(path: string, body: any) {
        let url = path
        const doFetch = () => {
            const headers = {
                'Accept': 'application/json',
                'Content-Type': 'application/json'
            }
            this.setAuthorizationHeader(headers)
            return fetch(url, {
                method: "POST",
                body: JSON.stringify(body),
                headers: headers,
            })
        }
        let resp = await doFetch()
        resp = await this.handleErrors(resp, doFetch)
        return resp
    }

    async getShortNames(): Promise<ShortName[]> {
        return this.doGet("/api/getShortNames")
    }

    async listenUpdates(filterProject: string | undefined, filterSubDir: string | undefined, handle: (msg: any) => void): Promise<() => void> {
        let host = window.location.host
        if (process.env.NODE_ENV === 'development') {
            host = "localhost:9090"
        }
        let url = `ws://${host}/api/ws`
        const params = new URLSearchParams()
        if (filterProject) {
            params.set("filterProject", filterProject)
        }
        if (filterSubDir) {
            params.set("filterSubDir", filterSubDir)
        }
        url += "?" + params.toString()

        const getToken = this.getToken
        let ws: WebSocket | undefined;
        let cancelled = false

        const connect = async () => {
            if (cancelled) {
                return
            }

            console.log("ws connect: " + url)
            ws = new WebSocket(url);
            ws.onopen = function () {
                console.log("ws connected")
                if (getToken) {
                    ws!.send(JSON.stringify({ "type": "auth", "token": getToken() }))
                }
            }
            ws.onclose = function (event) {
                console.log("ws close")
                if (!cancelled) {
                    sleep(5000).then(connect)
                }
            }
            ws.onerror = function (event) {
                console.log("ws error", event)
            }
            ws.onmessage = function (event: MessageEvent) {
                if (cancelled) {
                    return
                }
                const msg = JSON.parse(event.data)
                handle(msg)
            }
        }

        await connect()

        return () => {
            console.log("ws cancel")
            cancelled = true
            if (ws) {
                ws.close()
            }
        }
    }

    async getResult(resultId: string) {
        const params = new URLSearchParams()
        params.set("resultId", resultId)
        const json = await this.doGet("/api/getResult", params)
        return new CommandResult(json)
    }

    async getResultSummary(resultId: string) {
        const params = new URLSearchParams()
        params.set("resultId", resultId)
        const json = await this.doGet("/api/getResultSummary", params)
        return new CommandResultSummary(json)
    }

    async getResultObject(resultId: string, ref: ObjectRef, objectType: string) {
        const params = new URLSearchParams()
        params.set("resultId", resultId)
        params.set("objectType", objectType)
        buildRefParams(ref, params)
        return await this.doGet("/api/getResultObject", params)
    }

    async validateNow(project: ProjectKey, target: TargetKey) {
        return this.doPost("/api/validateNow", {
            "project": project,
            "target": target,
        })
    }

    async deployNow(cluster: string, name: string, namespace: string): Promise<Response> {
        return this.doPost("/api/deployNow", {
            "cluster": cluster,
            "name": name,
            "namespace": namespace,
        })
    }

    async reconcileNow(cluster: string, name: string, namespace: string): Promise<Response> {
        return this.doPost("/api/reconcileNow", {
            "cluster": cluster,
            "name": name,
            "namespace": namespace,
        })
    }
}

export class StaticApi implements Api {
    async getShortNames(): Promise<ShortName[]> {
        await loadScript(staticPath + "/shortnames.js")
        return staticShortNames
    }

    async listenUpdates(filterProject: string | undefined, filterSubDir: string | undefined, handle: (msg: any) => void): Promise<() => void> {
        await loadScript(staticPath + "/summaries.js")

        staticSummaries.forEach(rs => {
            if (filterProject && filterProject !== rs.project.normalizedGitUrl) {
                return
            }
            if (filterSubDir && filterSubDir !== rs.project.subDir) {
                return
            }
            handle({
                "type": "update_summary",
                "summary": rs,
            })
        })
        return () => {
        }
    }

    async getResult(resultId: string): Promise<CommandResult> {
        await loadScript(staticPath + `/result-${resultId}.js`)
        return staticResults.get(resultId)
    }

    async getResultSummary(resultId: string): Promise<CommandResultSummary> {
        await loadScript(staticPath + "/summaries.js")
        return staticSummaries.filter(s => {
            return s.id === resultId
        }).at(0)
    }

    async getResultObject(resultId: string, ref: ObjectRef, objectType: string): Promise<any> {
        const result = await this.getResult(resultId)
        const object = result.objects?.find(x => _.isEqual(x.ref, ref))
        if (!object) {
            throw new Error("object not found")
        }
        switch (objectType) {
            case ObjectType.Rendered:
                return object.rendered
            case ObjectType.Remote:
                return object.remote
            case ObjectType.Applied:
                return object.applied
            default:
                throw new Error("unknown object type " + objectType)
        }
    }

    validateNow(project: ProjectKey, target: TargetKey): Promise<Response> {
        throw new Error("not implemented")
    }

    reconcileNow(cluster: string, name: string, namespace: string): Promise<Response> {
        throw new Error("not implemented")
    }

    deployNow(cluster: string, name: string, namespace: string): Promise<Response> {
        throw new Error("not implemented")
    }
}

function buildRefParams(ref: ObjectRef, params: URLSearchParams) {
    params.set("kind", ref.kind)
    params.set("name", ref.name)
    if (ref.group) {
        params.set("group", ref.group)
    }
    if (ref.version) {
        params.set("version", ref.version)
    }
    if (ref.namespace) {
        params.set("namespace", ref.namespace)
    }
}

export function buildRefString(ref: ObjectRef): string {
    if (ref.namespace) {
        return `${ref.namespace}/${ref.kind}/${ref.name}`
    } else {
        if (ref.name) {
            return `${ref.kind}/${ref.name}`
        } else {
            return ref.kind
        }
    }
}

export function buildRefKindElement(ref: ObjectRef, element?: React.ReactElement): React.ReactElement {
    const tooltip = <Box zIndex={1000}>
        <Typography>ApiVersion: {[ref.group, ref.version].filter(x => x).join("/")}</Typography><br/>
        <Typography>Kind: {ref.kind}</Typography>
    </Box>
    return <Tooltip title={tooltip}>
        {element ? element : <Typography>{ref.kind}</Typography>}
    </Tooltip>
}

export function buildObjectRefFromObject(obj: any): ObjectRef {
    const apiVersion: string = obj.apiVersion
    const s = apiVersion.split("/", 2)
    let ref = new ObjectRef()
    if (s.length === 1) {
        ref.version = s[0]
    } else {
        ref.group = s[0]
        ref.version = s[1]
    }
    ref.kind = obj.kind
    ref.namespace = obj.metadata.namespace
    ref.name = obj.metadata.name
    return ref
}

export function findObjectByRef(l: ResultObject[] | undefined, ref: ObjectRef, filter?: (o: ResultObject) => boolean): ResultObject | undefined {
    return l?.find(x => {
        if (filter && !filter(x)) {
            return false
        }
        return _.isEqual(x.ref, ref)
    })
}
