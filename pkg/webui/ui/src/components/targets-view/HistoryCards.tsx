import React, { Suspense, useEffect, useRef, useState } from "react";
import { Box, Tab, useTheme } from "@mui/material";
import { CommandResultSummary } from "../../models";
import { ProjectSummary, TargetSummary } from "../../project-summaries";
import { CardPaper, cardHeight, cardWidth } from "./Card";
import { CommandResultItemHeader } from "./CommandResultItem";
import { Loading } from "../Loading";
import { NodeData } from "../result-view/nodes/NodeData";
import { api, usePromise } from "../../api";
import { NodeBuilder } from "../result-view/nodes/NodeBuilder";
import { SidePanelProvider, useSidePanelTabs } from "../result-view/SidePanel";
import { TabContext, TabList, TabPanel } from "@mui/lab";

async function doGetRootNode(rs: CommandResultSummary) {
    const shortNames = api.getShortNames()
    const r = api.getResult(rs.id)
    const builder = new NodeBuilder({
        shortNames: await shortNames,
        summary: rs,
        commandResult: await r,
    })
    const [node] = builder.buildRoot()
    return node
}

export interface HistoryCardsProps {
    rs: CommandResultSummary,
    ts: TargetSummary,
    ps: ProjectSummary,
    initialCardRect: DOMRect
}

const paddingY = 25;
const paddingX = 120;

interface Rect {
    left: number,
    top: number,
    width: number | string,
    height: number | string
}

type TransitionStatus = 'not-started' | 'running' | 'finished'

function CardContent(props: { provider: SidePanelProvider }) {
    const { tabs, selectedTab, handleTabChange } = useSidePanelTabs(props.provider)

    if (!props.provider
        || !selectedTab
        || !tabs.find(x => x.label === selectedTab)
    ) {
        return null;
    }

    return <TabContext value={selectedTab}>
        <TabList onChange={handleTabChange}>
            {tabs.map((tab, i) => {
                return <Tab label={tab.label} value={tab.label} key={tab.label} />
            })}
        </TabList>
        <Box overflow='auto' p='30px'>
            {tabs.map(tab => {
                return <TabPanel value={tab.label} sx={{ padding: 0 }}>
                    {tab.content}
                </TabPanel>
            })}
        </Box>
    </TabContext>
}

export const HistoryCards = React.memo((props: HistoryCardsProps) => {
    const theme = useTheme();
    const containerElem = useRef<HTMLElement>();
    const [cardRect, setCardRect] = useState<Rect | undefined>();
    const [transitionStatus, setTransitionStatus] = useState<TransitionStatus>('not-started');
    const [promise, setPromise] = useState<Promise<NodeData>>(new Promise(() => undefined));

    const Content = () => {
        const node = usePromise(promise)
        return <CardContent provider={node} />;
    }

    useEffect(() => {
        if (props.rs === undefined) {
            return
        }
        setPromise(doGetRootNode(props.rs));
    }, [props.rs]);

    useEffect(() => {
        const rect = containerElem.current?.getBoundingClientRect();
        if (!rect) {
            setCardRect(undefined);
            return;
        }

        const initialRect = {
            left: props.initialCardRect.left - rect.left - paddingX,
            top: props.initialCardRect.top - rect.top - paddingY,
            width: cardWidth,
            height: cardHeight
        };

        setCardRect(initialRect);
    }, [props.initialCardRect]);

    useEffect(() => {
        if (!cardRect) {
            return;
        }

        const targetRect = {
            left: 0,
            top: 0,
            width: '100%',
            height: '100%'
        };

        if (cardRect.left === targetRect.left
            && cardRect.top === targetRect.top
            && cardRect.width === targetRect.width
            && cardRect.height === targetRect.height
        ) {
            return;
        }

        setTimeout(() => {
            setCardRect(targetRect);
            setTransitionStatus('running');
            setTimeout(() => {
                setTransitionStatus('finished');
            }, theme.transitions.duration.enteringScreen)
        }, 10);
    }, [cardRect, theme.transitions.duration.enteringScreen]);

    return <Box
        width='100%'
        height='100%'
        p={`${paddingY}px ${paddingX}px`}
        position='relative'
        ref={containerElem}
    >
        {cardRect && <CardPaper
            sx={{
                width: cardRect.width,
                height: cardRect.height,
                position: 'relative',
                translate: `${cardRect.left}px ${cardRect.top}px`,
                transition: theme.transitions.create(['translate', 'width', 'height'], {
                    easing: theme.transitions.easing.sharp,
                    duration: theme.transitions.duration.enteringScreen,
                }),
                padding: '20px 16px'
            }}
        >
            <Box
                display='flex'
                flexDirection='column'
                height='100%'
            >
                <CommandResultItemHeader rs={props.rs} />
                <Box width='100%' flex='1 1 auto' overflow='hidden'>
                    {transitionStatus === 'finished' &&
                        <Suspense fallback={<Loading />}>
                            <Content />
                        </Suspense>
                    }
                </Box>
            </Box>
        </CardPaper>}
    </Box>;
});
