import { NodeData } from "./NodeData";
import React from "react";
import { Delete, LinkOff } from "@mui/icons-material";
import { CommandResultProps } from "../CommandResultView";
import { PropertiesTable } from "../../PropertiesTable";
import { SidePanelTab } from "../SidePanel";

export class DeletedOrOrphanObjectsCollectionNode extends NodeData {
    deleted: boolean

    constructor(props: CommandResultProps, id: string, deleted: boolean) {
        super(props, id, false, true);
        this.deleted = deleted
    }

    buildSidePanelTitle(): React.ReactNode {
        if (this.deleted) {
            return `${this.diffStatus?.deletedObjects.length} objects deleted`
        } else {
            return `${this.diffStatus?.orphanObjects.length} objects orphaned`
        }
    }

    buildIcon(): [React.ReactNode, string] {
        if (this.deleted) {
            return [<Delete fontSize={"large"}/>, "deleted"]
        } else {
            return [<LinkOff fontSize={"large"}/>, "orphans"]
        }
    }

    buildSidePanelTabs(): SidePanelTab[] {
        const tabs = [
            {label: "Summary", content: this.buildSummaryPage()}
        ]

        return tabs
    }

    buildSummaryPage(): React.ReactNode {
        const props = []

        if (this.diffStatus?.deletedObjects.length) {
            props.push({ name: "Deleted", value: this.diffStatus?.deletedObjects.length })
        }
        if (this.diffStatus?.orphanObjects.length) {
            props.push({ name: "Orphaned", value: this.diffStatus?.orphanObjects.length })
        }

        return <>
            <PropertiesTable properties={props}/>
        </>
    }
}
