import React, { useEffect, useRef, useState } from "react";
import { Box, ClickAwayListener, IconButton, useTheme } from "@mui/material";
import { TriangleLeftLightIcon, TriangleRightLightIcon } from "../../icons/Icons";
import { Transition, TransitionStatus } from "react-transition-group";

const transitionTime = 200
const arrowButtonWidth = 80;
const arrowButtonHeight = 50;
export const expandedZIndex = 1300; // same as MUI modals

const ArrowButton = React.memo((props: {
    direction: 'left' | 'right',
    onButtonClick: () => void,
    onContainerClick: () => void,
    containerRect: DOMRect,
    hidden: boolean
}) => {
    const Icon = {
        left: TriangleLeftLightIcon,
        right: TriangleRightLightIcon
    }[props.direction];

    if (props.hidden) {
        return <></>
    }

    const top = props.containerRect.top + props.containerRect.height / 2 - arrowButtonHeight / 2

    let left = 10
    if (props.direction === "right") {
        left = props.containerRect.right + 10
    }


    return <IconButton
        sx={{
            position: "fixed",
            top: `${top}px`,
            left: `${left}px`,
            zIndex: expandedZIndex,
        }}
        onClick={(e) => {
            e.stopPropagation();
            props.onButtonClick()
        }}
    >
        <Icon/>
    </IconButton>
});

export interface ExpandedCardsViewProps<CardData> {
    cardWidth: number,
    cardHeight: number,
    expand: boolean,
    selected: string

    cardsData: CardData[],
    getKey: (cd: CardData) => string
    renderCard: (
        cardData: CardData,
        expanded: boolean,
        current: boolean
    ) => React.ReactElement,

    onExpand: () => void
    onClose: () => void
    onSelect: (cd: CardData) => void
}

export const ExpandableCard = function <CardData>(props: ExpandedCardsViewProps<CardData>) {
    const theme = useTheme();

    const dummyBoxRef = useRef<HTMLElement>(null)
    const nodeRef = useRef(null)

    const [initialRender, setInitialRender] = useState(true)
    const [expanding, setExpanding] = useState(false)
    const [expandingIndex, setExpandingIndex] = useState(0)
    const [fullyExpanded, setFullyExpanded] = useState(false)

    let selectedIndex = props.cardsData.findIndex((cd) => props.getKey(cd) === props.selected)
    if (selectedIndex === -1) {
        selectedIndex = 0
    }

    useEffect(() => {
        setInitialRender(false)
    }, [])
    useEffect(() => {
        if (!props.expand) {
            setFullyExpanded(false)
        }
    }, [props.expand])

    const handleClick = () => {
        if (props.expand) {
            return
        }
        props.onExpand()
    }
    const handleClickaway = () => {
        if (!props.expand) {
            return
        }
        setFullyExpanded(false)
        props.onClose()
    }
    const handleSelect = (newIndex: number) => {
        newIndex = Math.max(0, newIndex)
        newIndex = Math.min(props.cardsData.length - 1, newIndex)
        setExpandingIndex(newIndex)
        props.onSelect(props.cardsData[newIndex])
    }

    const sx: any = {}

    if (!props.expand) {
        sx.cursor = "pointer"
    }

    let expandedTransformX = 0
    let expandedTransformY = 0
    let expandedWidth = 0
    let expandedHeight = 0

    const containerRect = new DOMRect(arrowButtonWidth, 40, window.innerWidth - arrowButtonWidth * 2, window.innerHeight - 80)

    if (dummyBoxRef.current) {
        const r = dummyBoxRef.current.getBoundingClientRect()
        expandedTransformX += -r.left + containerRect.left
        expandedTransformY += -r.top + containerRect.top
        expandedWidth = containerRect.width
        expandedHeight = containerRect.height
    }

    const buildStyle = (state: TransitionStatus, i: number): any => {
        let r = dummyBoxRef.current?.getBoundingClientRect()!
        const selectOffsetX = -(selectedIndex - i) * (window.innerWidth)
        const transitionProps = ["transform", "width", "height", "min-width", "min-height"]
        if (state === "exited") {
            return {
                position: "relative",
                width: `${props.cardWidth}px`,
                height: `${props.cardHeight}px`,
                minWidth: `${props.cardWidth}px`,
                minHeight: `${props.cardHeight}px`,
                visibility: i === selectedIndex ? "visible" : "hidden",
            }
        } else if (state === "entering") {
            return {
                position: "fixed",
                width: `${expandedWidth}px`,
                height: `${expandedHeight}px`,
                minWidth: `${expandedWidth}px`,
                minHeight: `${expandedHeight}px`,
                top: r.top,
                left: r.left,
                transform: `translateX(${selectOffsetX + expandedTransformX}px) translateY(${expandedTransformY}px)`,
                transition: theme.transitions.create(transitionProps, {
                    duration: transitionTime,
                    easing: theme.transitions.easing.sharp
                }),
                "z-index": expandedZIndex,
                visibility: i === selectedIndex ? "visible" : "hidden",
            }
        } else if (state === "entered") {
            return {
                position: "fixed",
                width: `${expandedWidth}px`,
                height: `${expandedHeight}px`,
                minWidth: `${expandedWidth}px`,
                minHeight: `${expandedHeight}px`,
                top: r.top,
                left: r.left,
                transform: `translateX(${selectOffsetX + expandedTransformX}px) translateY(${expandedTransformY}px)`,
                transition: theme.transitions.create(transitionProps, {
                    duration: transitionTime,
                    easing: theme.transitions.easing.sharp
                }),
                "z-index": expandedZIndex,
            }
        } else if (state === "exiting") {
            return {
                position: "fixed",
                width: `${props.cardWidth}px`,
                height: `${props.cardHeight}px`,
                minWidth: `${props.cardWidth}px`,
                minHeight: `${props.cardHeight}px`,
                top: r.top,
                left: r.left,
                transition: theme.transitions.create(transitionProps, {
                    duration: transitionTime,
                    easing: theme.transitions.easing.sharp
                }),
                "z-index": expandedZIndex,
                visibility: i === expandingIndex ? "visible" : "hidden",
            }
        }
        return {}
    }

    return <ClickAwayListener onClickAway={handleClickaway}>
        <Box ref={dummyBoxRef} width={props.cardWidth} height={props.cardHeight} minWidth={props.cardWidth}
             minHeight={props.cardHeight}>
            <ArrowButton
                direction='left'
                onButtonClick={() => {
                    handleSelect(selectedIndex - 1)
                }}
                onContainerClick={handleClickaway}
                containerRect={containerRect}
                hidden={selectedIndex === 0 || !fullyExpanded}
            />
            {props.cardsData.map((cd, i) => {
                return <Transition
                    key={props.getKey(cd)}
                    in={!initialRender && props.expand}
                    nodeRef={nodeRef}
                    timeout={transitionTime}
                    onEnter={() => {
                        setExpanding(true)
                        setExpandingIndex(selectedIndex)
                    }}
                    onEntered={() => {
                        setFullyExpanded(true)
                    }}
                    onExited={() => {
                        setExpanding(false)
                    }}
                >
                    {state => {
                        const style = {
                            ...sx,
                            ...buildStyle(state, i)
                        }
                        return <Box ref={nodeRef} display={"flex"} sx={style} onClick={handleClick}>
                            {props.renderCard(cd, expanding, i === selectedIndex)}
                        </Box>
                    }}
                </Transition>
            })}
            <ArrowButton
                direction='right'
                onButtonClick={() => {
                    handleSelect(selectedIndex + 1)
                }}
                onContainerClick={handleClickaway}
                containerRect={containerRect}
                hidden={selectedIndex === props.cardsData.length - 1 || !fullyExpanded}
            />
        </Box>
    </ClickAwayListener>
};
