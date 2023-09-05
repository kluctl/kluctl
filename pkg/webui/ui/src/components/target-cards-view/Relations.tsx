import React from "react";
import { Box, useTheme } from "@mui/material";
import { cardGap, cardHeight } from "../card/Card";

const curveRadius = 12;
const circleRadius = 5;
const strokeWidth = 2;

const Circle = React.memo((props: React.SVGProps<SVGCircleElement>) => {
    const theme = useTheme();
    return <circle
        r={circleRadius}
        fill={theme.palette.background.default}
        stroke={theme.palette.secondary.main}
        strokeWidth={strokeWidth}
        {...props}
    />
})

export const RelationHorizontalLine = React.memo(() => {
    const theme = useTheme();
    return <Box flexGrow={1} display='flex' justifyContent='center' alignItems='center' px='9px'>
        <svg
            xmlns='http://www.w3.org/2000/svg'
            fill='none'
            height={`${2 * circleRadius + strokeWidth}px`}
            width='100%'
        >
            <svg
                height='100%'
                width='100%'
                viewBox={`0 0 100 ${2 * circleRadius + strokeWidth}`}
                fill='none'
                preserveAspectRatio='none'
            >
                <path
                    d={`
                  M ${circleRadius + strokeWidth / 2} ${circleRadius + strokeWidth / 2}
                  H ${100 - circleRadius - strokeWidth / 2}
                `}
                    stroke={theme.palette.secondary.main}
                    strokeWidth={strokeWidth}
                />
            </svg>
            <svg
                fill='none'
                height='100%'
                width='100%'
            >
                <Circle cx={circleRadius + strokeWidth / 2} cy='50%' />
            </svg>
            <svg
                fill='none'
                height='100%'
                width='100%'
            >
                <Circle cx={`calc(100% - ${circleRadius + strokeWidth / 2}px)`} cy='50%' />
            </svg>
        </svg>
    </Box>;
});

export const RelationTree = React.memo(({ targetCount }: { targetCount: number }): JSX.Element | null => {
    const theme = useTheme();
    const height = targetCount * cardHeight + (targetCount - 1) * cardGap
    const width = 100;

    if (targetCount <= 0) {
        return null;
    }

    const targetsCenterYs = Array(targetCount).fill(0).map((_, i) =>
        cardHeight / 2 + i * (cardHeight + cardGap)
    );
    const rootCenterY = height / 2;

    return <svg
        width={width}
        height={height}
        viewBox={`0 0 ${width} ${height}`}
        fill='none'
    >
        {targetsCenterYs.map((cy, i) => {
            let d: React.SVGAttributes<SVGPathElement>['d'];
            if (targetCount % 2 === 1 && i === Math.floor(targetCount / 2)) {
                // target is in the middle.
                d = `
                        M ${circleRadius},${rootCenterY}
                        h ${width - circleRadius}
                    `;
            } else if (i < targetCount / 2) {
                // target is higher than root.
                d = `
                        M ${circleRadius},${rootCenterY}
                        h ${width / 2 - curveRadius - circleRadius}
                        a ${curveRadius} ${curveRadius} 90 0 0 ${curveRadius} -${curveRadius}
                        v ${cy - rootCenterY + curveRadius * 2}
                        a ${curveRadius} ${curveRadius} 90 0 1 ${curveRadius} -${curveRadius}
                        h ${width / 2 - curveRadius - circleRadius}
                    `;
            } else {
                // target is lower than root.
                d = `
                    M ${circleRadius},${rootCenterY}
                    h ${width / 2 - curveRadius - circleRadius}
                    a ${curveRadius} ${curveRadius} 90 0 1 ${curveRadius} ${curveRadius}
                    v ${cy - rootCenterY - curveRadius * 2}
                    a ${curveRadius} ${curveRadius} 90 0 0 ${curveRadius} ${curveRadius}
                    h ${width / 2 - curveRadius - circleRadius}
                `;
            }

            return [
                <path
                    key={`path-${i}`}
                    d={d}
                    stroke={theme.palette.secondary.main}
                    strokeWidth={strokeWidth}
                    strokeLinecap='round'
                    strokeLinejoin='round'
                />,
                <Circle
                    key={`circle-${i}`}
                    cx={width - circleRadius - strokeWidth / 2}
                    cy={cy}
                />
            ]
        })}
        <Circle
            key='circle-root'
            cx={circleRadius + strokeWidth / 2}
            cy={rootCenterY}
        />
    </svg>
});