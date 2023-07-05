import {
    CommandResult,
    CommandResultSummary,
    ObjectRef,
    ProjectKey,
    ResultObject,
    ShortName,
    TargetKey,
    ValidateResult
} from "./models";
import _ from "lodash";
import React from "react";
import { Box, Typography } from "@mui/material";
import Tooltip from "@mui/material/Tooltip";
import "./staticbuild.d.ts"
import { loadScript } from "./loadscript";
import { GitRef } from "./models-static";

console.log(window.location)

export let rootPath = window.location.pathname
if (rootPath.endsWith("/")) {
    rootPath = rootPath.substring(0, rootPath.length-1)
}
if (rootPath.endsWith("index.html")) {
    rootPath = rootPath.substring(0, rootPath.length-"index.html".length-1)
}
const staticPath = rootPath + "/staticdata"

console.log("rootPath=" + rootPath)
console.log("staticPath=" + staticPath)

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
    getCommandResult(resultId: string): Promise<CommandResult>
    getCommandResultSummary(resultId: string): Promise<CommandResultSummary>
    getCommandResultObject(resultId: string, ref: ObjectRef, objectType: string): Promise<any>
    getValidateResult(resultId: string): Promise<ValidateResult>
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

    async doGet(path: string, params?: URLSearchParams, abort?: AbortSignal) {
        let url = rootPath + path
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
                signal: abort ? abort : null,
            })
        }
        let resp = await doFetch()
        resp = await this.handleErrors(resp, doFetch)
        return resp.json()
    }

    async doPost(path: string, body: any) {
        let url = rootPath + path
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
        const params = new URLSearchParams()
        if (filterProject) {
            params.set("filterProject", filterProject)
        }
        if (filterSubDir) {
            params.set("filterSubDir", filterSubDir)
        }

        let seq = 0
        const abort = new AbortController()

        const doGetEvents = async () => {
            if (abort.signal.aborted) {
                return
            }

            params.set("seq", seq + "")
            let resp: any
            try {
                resp = await this.doGet("/api/events", params, abort.signal)
            } catch (error) {
                console.log("events error", error)
                seq = 0
                await new Promise(r => setTimeout(r, 5000));
                doGetEvents()
                return
            }

            seq = resp.nextSeq
            const events = resp.events

            events.forEach((e: any) => {
                handle(e)
            })

            doGetEvents()
        }

        doGetEvents()

        return () => {
            console.log("events cancel")
            abort.abort()
        }
    }

    async getCommandResult(resultId: string) {
        const params = new URLSearchParams()
        params.set("resultId", resultId)
        const json = await this.doGet("/api/getCommandResult", params)
        return new CommandResult(json)
    }

    async getCommandResultSummary(resultId: string) {
        const params = new URLSearchParams()
        params.set("resultId", resultId)
        const json = await this.doGet("/api/getCommandResultSummary", params)
        return new CommandResultSummary(json)
    }

    async getCommandResultObject(resultId: string, ref: ObjectRef, objectType: string) {
        const params = new URLSearchParams()
        params.set("resultId", resultId)
        params.set("objectType", objectType)
        buildRefParams(ref, params)
        return await this.doGet("/api/getCommandResultObject", params)
    }

    async getValidateResult(resultId: string) {
        const params = new URLSearchParams()
        params.set("resultId", resultId)
        const json = await this.doGet("/api/getValidateResult", params)
        return new ValidateResult(json)
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

    async getCommandResult(resultId: string): Promise<CommandResult> {
        await loadScript(staticPath + `/result-${resultId}.js`)
        return staticResults.get(resultId)
    }

    async getCommandResultSummary(resultId: string): Promise<CommandResultSummary> {
        await loadScript(staticPath + "/summaries.js")
        return staticSummaries.filter(s => {
            return s.id === resultId
        }).at(0)
    }

    async getCommandResultObject(resultId: string, ref: ObjectRef, objectType: string): Promise<any> {
        const result = await this.getCommandResult(resultId)
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

    async getValidateResult(resultId: string): Promise<ValidateResult> {
        throw new Error("not implemented")
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

export function buildGitRefString(ref?: GitRef) {
    if (!ref) {
        return "HEAD"
    }
    if (ref.branch) {
        return ref.branch
    } else if (ref.tag) {
        return ref.tag
    } else {
        return "<unknown>"
    }
}

export function findObjectByRef(l: ResultObject[] | undefined, ref: ObjectRef, filter?: (o: ResultObject) => boolean): ResultObject | undefined {
    return l?.find(x => {
        if (filter && !filter(x)) {
            return false
        }
        return _.isEqual(x.ref, ref)
    })
}
