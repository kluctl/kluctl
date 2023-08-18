import React from 'react';

import { NodeData } from "./NodeData";
import { PropertiesEntry, PropertiesTable, pushProp } from "../../PropertiesTable";

import { SidePanelTab } from "../SidePanel";
import { DeployIcon } from '../../../icons/Icons';
import { LogsViewer } from "../../LogsViewer";
import { CommandResult } from "../../../models";
import { YamlViewer } from "../../YamlViewer";
import { gitRefToString } from "../../../utils/git";

export class CommandResultNodeData extends NodeData {
    constructor(commandResult: CommandResult, id: string) {
        super(commandResult, id, true, true);
    }

    buildSidePanelTitle(): React.ReactNode {
        return "CommandResult";
    }

    buildIcon(): [React.ReactNode, string] {
        return [<DeployIcon />, "result"]
    }

    buildSidePanelTabs(): SidePanelTab[] {
        const tabs = [
            {label: "Summary", content: this.buildSummaryPage()},
            {label: "Target", content: this.buildTargetPage()},
        ]

        this.buildDiffAndHealthPages(tabs)

        tabs.push({label: "Logs", content: <LogsViewer
                cluster={this.commandResult.clusterInfo.clusterId}
                reconcileId={this.commandResult.id}/>
        })

        return tabs
    }

    buildSummaryPage(): React.ReactNode {
        const props: PropertiesEntry[] = [
            {name: "Initiator", value: this.commandResult.command?.initiator},
        ]

        let args = this.commandResult.command?.args
        if (args && Object.keys(args).length === 0) {
            args = undefined
        }

        pushProp(props, "Command", this.commandResult.command?.command)
        pushProp(props, "Start Time", this.commandResult.command?.startTime)
        pushProp(props, "End Time", this.commandResult.command?.endTime)
        pushProp(props, "Target", this.commandResult.command?.target)
        pushProp(props, "Target Name Override", this.commandResult.command?.targetNameOverride)
        pushProp(props, "Context Override", this.commandResult.command?.contextOverride)
        pushProp(props, "Args", args, () => <YamlViewer obj={args}/>)
        pushProp(props, "Dry Run", this.commandResult.command?.dryRun)
        pushProp(props, "No Wait", this.commandResult.command?.noWait)
        pushProp(props, "Force Apply", this.commandResult.command?.forceApply)
        pushProp(props, "Replace On Error", this.commandResult.command?.replaceOnError)
        pushProp(props, "Force Replace On Error", this.commandResult.command?.forceReplaceOnError)
        pushProp(props, "Abort On Error", this.commandResult.command?.abortOnError)
        pushProp(props, "Include Tags", this.commandResult.command?.includeTags)
        pushProp(props, "Exclude Tags", this.commandResult.command?.excludeTags)
        pushProp(props, "Include Deployment Dirs", this.commandResult.command?.includeDeploymentDirs)
        pushProp(props, "Exclude Deployment Dirs", this.commandResult.command?.excludeDeploymentDirs)

        if (this.commandResult.gitInfo) {
            pushProp(props, "Source Url", this.commandResult.gitInfo.url)
            pushProp(props, "Source Ref", gitRefToString(this.commandResult.gitInfo.ref))
            pushProp(props, "Source Path", this.commandResult.gitInfo.subDir)
            pushProp(props, "Source Commit", this.commandResult.gitInfo.commit)
            pushProp(props, "Source Dirty", this.commandResult.gitInfo.dirty)
        }

        pushProp(props, "Rendered Objects Hash", this.commandResult.renderedObjectsHash)
        pushProp(props, "New Objects", this.diffStatus?.newObjects.length)
        pushProp(props, "Deleted Objects", this.diffStatus?.deletedObjects.length)
        pushProp(props, "Orphan Objects", this.diffStatus?.orphanObjects.length)
        pushProp(props, "Changed Objects", this.diffStatus?.changedObjects.length)
        pushProp(props, "Errors", this.healthStatus?.errors?.length)
        pushProp(props, "Warnings", this.healthStatus?.warnings?.length)

        return <>
            <PropertiesTable properties={props}/>
        </>
    }

    buildTargetPage(): React.ReactNode {
        return <YamlViewer obj={this.commandResult.target} />
    }
}
