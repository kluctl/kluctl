import { NodeData } from "./NodeData";
import React from "react";
import { Delete, LinkOff } from "@mui/icons-material";
import { PropertiesTable } from "../../PropertiesTable";
import { CardTab } from "../../card/CardTabs";
import { CommandResult } from "../../../models";

export class DeletedOrOrphanObjectsCollectionNode extends NodeData {
    deleted: boolean

    constructor(commandResult: CommandResult, id: string, deleted: boolean) {
        super(commandResult, id, false, true);
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

    buildSidePanelTabs(): CardTab[] {
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
