import React from 'react';

import { NodeData } from "./NodeData";
import { PropertiesTable } from "../../PropertiesTable";
import { CodeViewer } from "../../CodeViewer";

import * as yaml from 'js-yaml';
import { SidePanelTab } from "../SidePanel";
import { DeployIcon } from '../../../icons/Icons';
import { LogsViewer } from "../../LogsViewer";
import { CommandResult } from "../../../models";

export class CommandResultNodeData extends NodeData {
    dumpedTargetYaml?: string

    constructor(commandResult: CommandResult, id: string) {
        super(commandResult, id, true, true);

        if (this.commandResult.command?.target) {
            this.dumpedTargetYaml = yaml.dump(this.commandResult.command.target)
        }
    }

    buildSidePanelTitle(): React.ReactNode {
        return "CommandResult";
    }

    buildIcon(): [React.ReactNode, string] {
        return [<DeployIcon />, "result"]
    }

    buildSidePanelTabs(): SidePanelTab[] {
        const tabs = [
            {label: "Summary", content: this.buildSummaryPage()}
        ]

        if (this.dumpedTargetYaml) {
            const page = this.buildTargetPage()
            tabs.push({label: "Target", content: page})
        }

        this.buildDiffAndHealthPages(tabs)

        tabs.push({label: "Logs", content: <LogsViewer
                cluster={this.commandResult.clusterInfo.clusterId}
                reconcileId={this.commandResult.id}/>
        })

        return tabs
    }

    buildSummaryPage(): React.ReactNode {
        const props = [
            {name: "Initiator", value: this.commandResult.command?.initiator},
            {name: "Command", value: this.commandResult.command?.command},
        ]

        return <>
            <PropertiesTable properties={props}/>
        </>
    }

    buildTargetPage(): React.ReactNode {
        return <CodeViewer code={this.dumpedTargetYaml!} language={"yaml"}/>
    }
}
