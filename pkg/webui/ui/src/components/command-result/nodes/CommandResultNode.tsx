import React from 'react';

import { NodeData } from "./NodeData";
import { PropertiesTable } from "../../PropertiesTable";

import { SidePanelTab } from "../SidePanel";
import { DeployIcon } from '../../../icons/Icons';
import { LogsViewer } from "../../LogsViewer";
import { CommandResult } from "../../../models";
import { YamlViewer } from "../../YamlViewer";

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
        const props = [
            {name: "Initiator", value: this.commandResult.command?.initiator},
            {name: "Command", value: this.commandResult.command?.command},
        ]

        return <>
            <PropertiesTable properties={props}/>
        </>
    }

    buildTargetPage(): React.ReactNode {
        return <YamlViewer obj={this.commandResult.target} />
    }
}
