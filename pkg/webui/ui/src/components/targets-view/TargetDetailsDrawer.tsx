import { TargetSummary } from "../../models";
import { Box, Drawer } from "@mui/material";
import { SidePanel, SidePanelProvider, SidePanelTab } from "../result-view/SidePanel";
import React from "react";
import { PropertiesTable } from "../PropertiesTable";
import { DiffStatus } from "../result-view/nodes/NodeData";
import { ChangesTable } from "../result-view/ChangesTable";
import { ErrorsTable } from "../ErrorsTable";

class MyProvider implements SidePanelProvider {
    private ts?: TargetSummary;
    private diffStatus: DiffStatus;

    constructor(ts?: TargetSummary) {
        this.ts = ts
        this.diffStatus = new DiffStatus()

        this.ts?.lastValidateResult?.drift?.forEach(co => {
            this.diffStatus.addChangedObject(co)
        })
    }

    buildSidePanelTabs(): SidePanelTab[] {
        if (!this.ts) {
            return []
        }

        const tabs = [
            {label: "Summary", content: this.buildSummaryTab()}
        ]

        if (this.ts.target)

        if (this.diffStatus.changedObjects.length) {
            tabs.push({
                label: "Drift",
                content: <ChangesTable diffStatus={this.diffStatus}/>
            })
        }
        if (this.ts.lastValidateResult?.errors?.length) {
            tabs.push({
                label: "Errors",
                content: <ErrorsTable errors={this.ts.lastValidateResult.errors}/>
            })
        }
        if (this.ts.lastValidateResult?.warnings?.length) {
            tabs.push({
                label: "Warnings",
                content: <ErrorsTable errors={this.ts.lastValidateResult.warnings}/>
            })
        }

        return tabs
    }

    buildSummaryTab(): React.ReactNode {
        const props = [
            {name: "Target Name", value: this.getTargetName()},
            {name: "Discriminator", value: this.ts?.target.discriminator},
        ]

        if (this.ts?.lastValidateResult) {
            props.push({name: "Ready", value: this.ts.lastValidateResult.ready + ""})
        }
        if (this.ts?.lastValidateResult?.errors?.length) {
            props.push({name: "Errors", value: this.ts.lastValidateResult.errors.length + ""})
        }
        if (this.ts?.lastValidateResult?.warnings?.length) {
            props.push({name: "Warnings", value: this.ts.lastValidateResult.warnings.length + ""})
        }
        if (this.ts?.lastValidateResult?.drift?.length) {
            props.push({name: "Drifted Objects", value: this.ts.lastValidateResult.drift.length + ""})
        }

        return <>
            <PropertiesTable properties={props}/>
        </>
    }

    getTargetName() {
        if (!this.ts) {
            return ""
        }

        let name = "<no-name>"
        if (this.ts.target.targetName) {
            name = this.ts.target.targetName
        }
        return name
    }

    buildSidePanelTitle(): React.ReactNode {
        return this.getTargetName()
    }
}

export const TargetDetailsDrawer = (props: { ts?: TargetSummary, onClose: () => void }) => {
    return <Drawer
        sx={{ zIndex: 1300 }}
        anchor={"right"}
        open={props.ts !== undefined}
        onClose={() => props.onClose()}
    >
        <Box width={"800px"} height={"100%"}>
            <SidePanel provider={new MyProvider(props.ts)}/>
        </Box>
    </Drawer>
}