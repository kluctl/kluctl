import React, { useState } from "react";
import { Box, Chip } from "@mui/material";
import { AutoFixHigh, ErrorOutline, Grade } from "@mui/icons-material";
import { OverridableComponent } from "@mui/material/OverridableComponent";
import Tooltip from "@mui/material/Tooltip";
import { NodeData } from "./nodes/NodeData";

export interface ActiveFilters {
    onlyImportant: boolean
    onlyChanged: boolean
    onlyWithErrorsOrWarnings: boolean
}

export function FilterNode(node: NodeData, activeFilters?: ActiveFilters) {
    const hasChanges = node.diffStatus?.hasDiffs()
    const hasErrorsOrWarnings = node.healthStatus?.errors.length || node.healthStatus?.warnings.length
    if (activeFilters?.onlyImportant && !hasChanges && !hasErrorsOrWarnings) {
        return false
    }
    if (activeFilters?.onlyChanged && !hasChanges) {
        return false
    }
    if (activeFilters?.onlyWithErrorsOrWarnings && !hasErrorsOrWarnings) {
        return false
    }
    return true
}

const FilterButton = (props: { Icon: OverridableComponent<any>, tooltip: string, color: any, active: boolean, handler: (active: boolean) => void }) => {
    const handleClick = () => {
        props.handler(!props.active)
    }

    const Icon = props.Icon
    const chipColor = props.active ? props.color : "default";
    return <Tooltip title={props.tooltip}>
        <Chip
            variant="filled"
            color={chipColor}
            label={
                <Icon
                    color={props.active ? undefined : props.color}
                    htmlColor={props.active ? "white" : undefined}
                />
            }
            onClick={handleClick}
            sx={{
                "& .MuiChip-label": {
                    display: "flex",
                    justifyContent: "center",
                    alignItems: "center"
                }
            }}
        />
    </Tooltip>
}

export const NodeStatusFilter = (props: { onFilterChange: (f: ActiveFilters) => void }) => {
    const [activeFilters, setActiveFilters] = useState<ActiveFilters>({
        onlyImportant: false,
        onlyChanged: false,
        onlyWithErrorsOrWarnings: false,
    })

    const doSetActiveFilters = (newActiveFilters: ActiveFilters) => {
        setActiveFilters(newActiveFilters)
        props.onFilterChange(newActiveFilters)
    }

    return (
        <Box display={"flex"} flexDirection={"column"} alignItems={"center"}>
            Filters
            <Box
                display="flex"
                alignItems="center"
                gap="5px"
            >
                <FilterButton Icon={Grade} tooltip={"Only important (changed or with errors/warnings)"}
                              color={"secondary"}
                              active={activeFilters.onlyImportant}
                              handler={(active: boolean) => {
                                  doSetActiveFilters({ ...activeFilters, onlyImportant: active });
                              }}/>
                <FilterButton Icon={AutoFixHigh} tooltip={"Only with changed"} color={"secondary"}
                              active={activeFilters.onlyChanged}
                              handler={(active: boolean) => {
                                  doSetActiveFilters({ ...activeFilters, onlyChanged: active });
                              }}/>
                <FilterButton Icon={ErrorOutline} tooltip={"Only with errors or warnings"} color={"error"}
                              active={activeFilters.onlyWithErrorsOrWarnings}
                              handler={(active: boolean) => {
                                  doSetActiveFilters({ ...activeFilters, onlyWithErrorsOrWarnings: active });
                              }}/>
            </Box>
        </Box>
    )
}
