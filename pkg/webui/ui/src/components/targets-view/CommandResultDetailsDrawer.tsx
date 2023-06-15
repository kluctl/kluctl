import { CommandResultSummary } from "../../models";
import { Api } from "../../api";
import { NodeBuilder } from "../result-view/nodes/NodeBuilder";
import React, { useContext, useEffect, useMemo, useRef, useState } from "react";
import { SidePanel } from "../result-view/SidePanel";
import { Box, Drawer, ThemeProvider, useTheme } from "@mui/material";
import { Loading, useLoadingHelper } from "../Loading";
import { dark } from "../theme";
import { Card, cardGap, cardHeight, cardWidth } from "./Card";
import { CommandResultItem } from "./CommandResultItem";
import { ProjectSummary, TargetSummary } from "../../project-summaries";
import { ApiContext } from "../App";
import { CommandResultNodeData } from "../result-view/nodes/CommandResultNode";

const sidePanelWidth = 720;

async function doGetRootNode(api: Api, rs: CommandResultSummary) {
    const shortNames = await api.getShortNames()
    const r = await api.getResult(rs.id)

    const builder = new NodeBuilder({
        shortNames: shortNames,
        summary: rs,
        commandResult: r,
    })
    const [node] = builder.buildRoot()
    return node
}

export const CommandResultDetailsDrawer = React.memo((props: {
    rs?: CommandResultSummary,
    ts?: TargetSummary,
    ps?: ProjectSummary,
    onClose: () => void,
    selectedCardRect?: DOMRect
}) => {
    const { ps, ts, selectedCardRect } = props;
    const api = useContext(ApiContext)
    const theme = useTheme();
    const [selectedCommandResult, setSelectedCommandResult] = useState<CommandResultSummary | undefined>();
    const [prevTargetSummary, setPrevTargetSummary] = useState<TargetSummary | undefined>(ts);
    const [cardsCoords, setCardsCoords] = useState<{ left: number, top: number }[]>([]);
    const [transitionRunning, setTransitionRunning] = useState(false);

    if (prevTargetSummary !== ts) {
        setPrevTargetSummary(ts);
        setSelectedCommandResult(ts?.commandResults?.[0]);
    }

    const [loading, loadingError, nodeData] = useLoadingHelper<CommandResultNodeData | undefined>(() => {
        if (selectedCommandResult === undefined) {
            return Promise.resolve(undefined)
        }
        return doGetRootNode(api, selectedCommandResult)
    }, [selectedCommandResult])

    const cardsContainerElem = useRef<HTMLElement>();

    useEffect(() => {
        const rect = cardsContainerElem.current?.getBoundingClientRect();
        if (!rect || !selectedCardRect || !ts?.commandResults) {
            setCardsCoords([]);
            return;
        }

        const initialCoords = ts.commandResults.map(() => ({
            left: selectedCardRect.left - rect.left,
            top: selectedCardRect.top - rect.top
        }));

        setCardsCoords(initialCoords);
        setTransitionRunning(true);
    }, [selectedCardRect, ts?.commandResults]);

    useEffect(() => {
        if (cardsCoords.length > 0) {
            const targetCoords = cardsCoords.map((_, i) => ({
                left: 0,
                top: i * (cardHeight + cardGap)
            }));

            if (cardsCoords.length === targetCoords.length
                && cardsCoords.every(({ left, top }, i) =>
                    targetCoords[i].left === left && targetCoords[i].top === top
                )
            ) {
                return;
            }

            setTimeout(() => {
                setCardsCoords(targetCoords);
                setTimeout(() => {
                    setTransitionRunning(false);
                }, theme.transitions.duration.enteringScreen)
            }, 10);
        }
    }, [cardsCoords, theme.transitions.duration.enteringScreen])

    const zIndex = theme.zIndex.modal + 1;

    const cards = useMemo(() => {
        if (!(ps && ts && ts.commandResults && ts.commandResults.length > 0 && cardsCoords.length > 0)) {
            return null;
        }

        return ts.commandResults.map((rs, i) => {
            return <Card
                key={rs.id}
                sx={{
                    position: 'absolute',
                    translate: `${cardsCoords[i].left}px ${cardsCoords[i].top}px`,
                    zIndex: zIndex,
                    transition: theme.transitions.create(['translate'], {
                        easing: theme.transitions.easing.sharp,
                        duration: theme.transitions.duration.enteringScreen,
                    })
                }}
            >
                <CommandResultItem
                    ps={ps}
                    ts={ts}
                    rs={rs}
                    onSelectCommandResult={setSelectedCommandResult}
                />
            </Card>
        })
    }, [ps, ts, cardsCoords, zIndex, theme.transitions])

    if (loadingError) {
        return <>Error</>
    }

    return <>
        {ps && ts && ts.commandResults && ts.commandResults.length > 0 &&
            <Box
                sx={{
                    position: 'fixed',
                    top: 0,
                    bottom: 0,
                    right: sidePanelWidth,
                    width: `calc(100% - ${sidePanelWidth}px)`,
                    overflowX: 'visible',
                    overflowY: transitionRunning ? 'visible' : 'auto',
                    display: 'flex',
                    flexDirection: 'column',
                    alignItems: 'center',
                    justifyContent: 'center',
                    padding: '25px 0',
                    zIndex: zIndex
                }}
                onClick={props.onClose}
            >
                <Box
                    onClick={(e) => e.stopPropagation()}
                    position='relative'
                    width={cardWidth}
                    height={cardHeight * ts.commandResults.length + cardGap * (ts.commandResults.length - 1)}
                    ref={cardsContainerElem}
                >
                    {cards}
                </Box>
            </Box>
        }
        <ThemeProvider theme={dark}>
            <Drawer
                sx={{ zIndex: 1300 }}
                anchor={"right"}
                open={props.rs !== undefined}
                onClose={props.onClose}
            >
                <Box width={sidePanelWidth} height={"100%"}>
                    {loading ? <Loading/> :
                        <SidePanel provider={nodeData} onClose={props.onClose} />
                    }
                </Box>
            </Drawer>
        </ThemeProvider>
    </>
});
