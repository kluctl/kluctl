import React from 'react';

import { NodeData } from "./NodeData";
import { CloudSync } from "@mui/icons-material";
import { PropertiesTable } from "../../PropertiesTable";
import { CodeViewer } from "../../CodeViewer";
import { CommandResultProps } from "../CommandResultView";

import * as yaml from 'js-yaml';
import { SidePanelTab } from "../SidePanel";

export class CommandResultNodeData extends NodeData {
    dumpedTargetYaml?: string

    constructor(props: CommandResultProps, id: string) {
        super(props, id, true, true);

        if (this.props.commandResult.command?.target) {
            this.dumpedTargetYaml = yaml.dump(this.props.commandResult.command.target)
        }
    }

    buildSidePanelTitle(): React.ReactNode {
        return "CommandResult";
    }

    buildIcon(): [React.ReactNode, string] {
        return [<CloudSync fontSize={"large"}/>, "result"]
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

        return tabs
    }

    buildSummaryPage(): React.ReactNode {
        const props = [
            {name: "Initiator", value: this.props.commandResult.command?.initiator},
            {name: "Command", value: this.props.commandResult.command?.command},
        ]

        return <>
            <PropertiesTable properties={props}/>
        </>
    }

    buildTargetPage(): React.ReactNode {
        return <CodeViewer code={this.dumpedTargetYaml!} language={"yaml"}/>
    }
}
