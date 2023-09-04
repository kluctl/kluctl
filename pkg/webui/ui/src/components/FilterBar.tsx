import React, { useState } from "react";
import { Box, Chip } from "@mui/material";
import { AutoFixHigh, ErrorOutline, Grade, TableRows } from "@mui/icons-material";
import { OverridableComponent } from "@mui/material/OverridableComponent";
import Tooltip from "@mui/material/Tooltip";
import { useTheme } from "@mui/material/styles";

export interface ActiveFilters {
    showTableView: boolean
    onlyImportant: boolean
    onlyDrifted: boolean
    onlyWithErrorsOrWarnings: boolean
    filterStr: string
}

export function DoFilterSwitches(hasDrift: boolean, hasErrors: boolean, hasWarnings: boolean, activeFilters?: ActiveFilters) {
    const hasErrorsOrWarnings = hasErrors || hasWarnings
    if (activeFilters?.onlyImportant && !hasDrift && !hasErrorsOrWarnings) {
        return false
    }
    if (activeFilters?.onlyDrifted && !hasDrift) {
        return false
    }
    if (activeFilters?.onlyWithErrorsOrWarnings && !hasErrorsOrWarnings) {
        return false
    }
    return true
}

export function DoFilterText(text: (string|undefined)[] | null, activeFilters?: ActiveFilters) {
    if (activeFilters?.filterStr && text && text.length) {
        const l = activeFilters.filterStr.toLowerCase()
        const f = text.find(t => t && t.toLowerCase().includes(l))
        if (!f) {
            return false
        }
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

export const FilterBar = (props: { onFilterChange: (f: ActiveFilters) => void }) => {
    const theme = useTheme();

    const [activeFilters, setActiveFilters] = useState<ActiveFilters>({
        showTableView: false,
        onlyImportant: false,
        onlyDrifted: false,
        onlyWithErrorsOrWarnings: false,
        filterStr: "",
    })

    const doSetActiveFilters = (newActiveFilters: ActiveFilters) => {
        setActiveFilters(newActiveFilters)
        props.onFilterChange(newActiveFilters)
    }

    return (
        <Box display={"flex"} flexDirection={"column"} alignItems={"center"}>
            <Box
                display="flex"
                alignItems="center"
                gap="5px"
            >
                <FilterButton Icon={TableRows} tooltip={"Show table view"}
                              color={"secondary"}
                              active={activeFilters.showTableView}
                              handler={(active: boolean) => {
                                  doSetActiveFilters({ ...activeFilters, showTableView: active });
                              }}/>
                <FilterButton Icon={Grade} tooltip={"Only important (drifted or with errors/warnings)"}
                              color={"secondary"}
                              active={activeFilters.onlyImportant}
                              handler={(active: boolean) => {
                                  doSetActiveFilters({ ...activeFilters, onlyImportant: active });
                              }}/>
                <FilterButton Icon={AutoFixHigh} tooltip={"Only with drift"} color={"secondary"}
                              active={activeFilters.onlyDrifted}
                              handler={(active: boolean) => {
                                  doSetActiveFilters({ ...activeFilters, onlyDrifted: active });
                              }}/>
                <FilterButton Icon={ErrorOutline} tooltip={"Only with errors or warnings"} color={"error"}
                              active={activeFilters.onlyWithErrorsOrWarnings}
                              handler={(active: boolean) => {
                                  doSetActiveFilters({ ...activeFilters, onlyWithErrorsOrWarnings: active });
                              }}/>
                <Box
                    height='40px'
                    maxWidth='314px'
                    flexGrow={1}
                    borderRadius='10px'
                    display='flex'
                    justifyContent='space-between'
                    alignItems='center'
                    padding='0 9px 0 15px'
                    sx={{ background: theme.palette.background.default }}
                >
                    <input
                        type='text'
                        style={{
                            background: 'none',
                            border: 'none',
                            outline: 'none',
                            height: '20px',
                            lineHeight: '20px',
                            fontSize: '18px'
                        }}
                        placeholder='Filter'
                        onChange={e => {
                            doSetActiveFilters({ ...activeFilters, filterStr: e.target.value })
                        }}
                    />
                </Box>
            </Box>
        </Box>
    )
}
