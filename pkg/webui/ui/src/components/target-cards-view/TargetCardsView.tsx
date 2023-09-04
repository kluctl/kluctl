import { CommandResultSummary } from "../../models";
import { Box, Typography } from "@mui/material";
import React, { useMemo } from "react";
import { useAppContext } from "../App";
import { ProjectCard } from "./ProjectCard";
import { TargetCard } from "./TargetCard";
import Divider from "@mui/material/Divider";
import { CardCol, cardGap, cardHeight, CardPaper, CardRow } from "../card/Card";
import { buildTargetKey, ProjectSummary, TargetSummary } from "../../project-summaries";
import { buildListKey } from "../../utils/listKey";
import { ExpandableCard } from "../card/ExpandableCard";
import { CommandResultCard } from "../command-result/CommandResultCard";
import { RelationHorizontalLine, RelationTree } from "./Relations";

const colWidth = 416;
const targetCardWidth = 300
const commandResultCardWidth = 247

function ColHeader({ children }: { children: React.ReactNode }) {
    return <Box
        minWidth={colWidth}
        width={colWidth}
        height='42px'
        display='flex'
        alignItems='center'
        sx={{
            borderLeft: '0.8px solid rgba(0,0,0,0.5)',
            paddingLeft: '15px',
            '&:first-of-type': {
                borderLeft: 'none',
                paddingLeft: 0
            }
        }}
    >
        <Typography variant='h2' textAlign='left'>{children}</Typography>
    </Box>
}

export interface TargetCardsViewProps {
    selectedProject?: ProjectSummary
    selectedTarget?: TargetSummary
    selectedResult?: CommandResultSummary
    selectedResultFull?: boolean
    onSelect: (ps: ProjectSummary, ts: TargetSummary, showResults: boolean, rs?: CommandResultSummary, full?: boolean) => void
    onCloseExpanded: () => void
}

export const TargetCardsView = (props: TargetCardsViewProps) => {
    const appContext = useAppContext();
    const projects = appContext.projects;

    const selectedTargetKey = useMemo(() => {
        if (!props.selectedProject || !props.selectedTarget) {
            return undefined
        }
        return buildTargetKey(props.selectedProject.project, props.selectedTarget.target, props.selectedTarget.kdInfo)
    }, [props.selectedProject, props.selectedTarget])

    return <Box minWidth={colWidth * 3} height={"100%"} p='0 40px'>
        <Box display={"flex"} alignItems={"center"} height='70px'>
            <ColHeader>Projects</ColHeader>
            <ColHeader>Targets</ColHeader>
            <ColHeader>Command Results</ColHeader>
        </Box>
        <Divider />
        {projects.map((ps, i) => {
            return <Box key={buildListKey(ps.project)}>
                <Box display={"flex"} alignItems={"center"} margin='40px 0'>
                    <Box display='flex' alignItems='center' width={colWidth} flex='0 0 auto'>
                        <ProjectCard ps={ps} />
                        <Box
                            flexGrow={1}
                            height={ps.targets.length * cardHeight + (ps.targets.length - 1) * cardGap}
                            display='flex'
                            justifyContent='center'
                            alignItems='center'
                        >
                            <RelationTree targetCount={ps.targets.length} />
                        </Box>
                    </Box>

                    <CardCol width={colWidth} flex='0 0 auto'>
                        {ps.targets.map((ts, i) => {
                            const key = buildTargetKey(ps.project, ts.target, ts.kdInfo)
                            return <Box key={key} display='flex'>
                                <ExpandableCard
                                    cardWidth={targetCardWidth}
                                    cardHeight={cardHeight}
                                    expand={!props.selectedResult && selectedTargetKey === key}
                                    onExpand={() => props.onSelect(ps, ts, false)}
                                    onClose={() => {
                                        props.onCloseExpanded()
                                    }}
                                    onSelect={cd => {
                                    }}
                                    cardsData={[ts]}
                                    getKey={cd => buildListKey([cd.target, cd.kdInfo])}
                                    selected={selectedTargetKey}
                                    renderCard={(cardData, expanded, current) => {
                                        return <TargetCard
                                            ps={ps}
                                            ts={ts}
                                            expanded={expanded}
                                            onClose={() => {
                                                props.onCloseExpanded()
                                            }}
                                        />
                                    }}/>
                                {ts.commandResults.length ? <RelationHorizontalLine/> : <></>}
                            </Box>
                        })}
                    </CardCol>

                    <CardCol width={colWidth}>
                        {ps.targets.map((ts, i) => {
                            const tsKey = buildTargetKey(ps.project, ts.target, ts.kdInfo)
                            return <CardRow key={tsKey} height={cardHeight}>
                                {ts.commandResults?.slice(0, 4).map((rs, i) => {
                                    return i === 0 ? <ExpandableCard
                                            key={rs.id}
                                            cardWidth={commandResultCardWidth}
                                            cardHeight={cardHeight}
                                            expand={selectedTargetKey === tsKey && !!props.selectedResult}
                                            onExpand={() => {
                                                props.onSelect(ps, ts, true, rs)
                                            }}
                                            onClose={() => {
                                                props.onCloseExpanded()
                                            }}
                                            onSelect={cd => {
                                                props.onSelect(ps, ts, true, cd)
                                            }}
                                            selected={props.selectedResult?.id}
                                            cardsData={ts.commandResults}
                                            getKey={cd => cd.id}
                                            renderCard={(cardData, expanded, current) => {
                                                return <CommandResultCard
                                                    current={current}
                                                    ps={ps}
                                                    ts={ts}
                                                    rs={cardData}
                                                    showSummary={!props.selectedResultFull}
                                                    expanded={expanded}
                                                    loadData={expanded && current}
                                                    onClose={() => {
                                                        props.onCloseExpanded()
                                                    }}
                                                    onSwitchFullCommandResult={() => {
                                                        props.onSelect(ps, ts, true, cardData, !props.selectedResultFull)
                                                    }}
                                                />
                                            }}/>
                                        : <CardPaper
                                            key={rs.id}
                                            sx={{
                                                width: commandResultCardWidth,
                                                height: cardHeight,
                                                translate: `${-i * (commandResultCardWidth + cardGap / 2)}px`,
                                                zIndex: -i,
                                            }}
                                        />
                                })}
                            </CardRow>
                        })}
                    </CardCol>
                </Box>
                <Divider />
            </Box>
        })}
    </Box>
}
