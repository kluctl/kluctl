import { ReactComponent as KluctlTextSvg } from './kluctl-text.svg';
import { ReactComponent as GitIconSvg } from './git.svg';
import { ReactComponent as KluctlLogoSvg } from './kluctl-logo.svg';
import { ReactComponent as SearchIconSvg } from './search.svg';
import { ReactComponent as TargetsIconSvg } from './targets.svg';
import { ReactComponent as TargetIconSvg } from './target.svg';
import { ReactComponent as RelationHLineSvg } from './relation-hline.svg';
import { ReactComponent as ProjectIconSvg } from './project.svg';
import { ReactComponent as CpuIconSvg } from './cpu.svg';
import { ReactComponent as FingerScanIconSvg } from './finger-scan.svg';
import { ReactComponent as TreeViewIconSvg } from './tree-view.svg';
import { ReactComponent as CloseIconSvg } from './close.svg';
import { ReactComponent as CloseLightIconSvg } from './close-light.svg';
import { ReactComponent as DeployIconSvg } from './deploy.svg';
import { ReactComponent as PruneIconSvg } from './prune.svg';
import { ReactComponent as DiffIconSvg } from './diff.svg';
import { ReactComponent as WarningIconSvg } from './warning.svg';
import { ReactComponent as ErrorIconSvg } from './error.svg';
import { ReactComponent as TrashIconSvg } from './trash.svg';
import { ReactComponent as OrphanIconSvg } from './orphan.svg';
import { ReactComponent as AddedIconSvg } from './added.svg';
import { ReactComponent as ChangedIconSvg } from './changed.svg';
import { ReactComponent as CheckboxIconSvg } from './checkbox.svg';
import { ReactComponent as CheckboxCheckedIconSvg } from './checkbox-checked.svg';
import { ReactComponent as CheckboxDisabledIconSvg } from './checkbox-disabled.svg';
import { ReactComponent as ArrowLeftIconSvg } from './arrow-left.svg';
import { ReactComponent as WarningSignIconSvg } from './warning-sign.svg';
import { ReactComponent as ChangesIconSvg } from './changes.svg';
import { ReactComponent as StarIconSvg } from './star.svg';
import { ReactComponent as TriangleDownIconSvg } from './triangle-down.svg';
import { ReactComponent as TriangleLeftLightIconSvg } from './triangle-left-light.svg';
import { ReactComponent as TriangleRightLightIconSvg } from './triangle-right-light.svg';
import { ReactComponent as TriangleRightIconSvg } from './triangle-right.svg';
import { ReactComponent as BracketsCurlyIconSvg } from './brackets-curly.svg';
import { ReactComponent as BracketsSquareIconSvg } from './brackets-square.svg';
import { ReactComponent as FileIconSvg } from './file.svg';
import { ReactComponent as ResultIconSvg } from './result.svg';
import { ReactComponent as IncludeIconSvg } from './include.svg';
import { ReactComponent as LogoutIconSvg } from './logout.svg';

export const KluctlText = () => {
    return <KluctlTextSvg width="115px" height="33px" />
}

export const GitIcon = () => {
    return <GitIconSvg width={"35px"} height={"35px"} />
}

export const KluctlLogo = () => {
    return <KluctlLogoSvg width="50px" height="50px" />
}

export const SearchIcon = () => {
    return <SearchIconSvg width="27px" height="27px" />
}

export const TargetsIcon = () => {
    return <TargetsIconSvg width="48px" height="48px" />
}

export const TargetIcon = () => {
    return <TargetIconSvg width="45px" height="45px" />
}
export const RelationHLine = () => {
    return <RelationHLineSvg width="169px" height="12px" />
}

export const ProjectIcon = () => {
    return <ProjectIconSvg width="45px" height="45px" />
}

export const DeployIcon = () => {
    return <DeployIconSvg width="45px" height="45px" />
}

export const PruneIcon = () => {
    return <PruneIconSvg width="45px" height="45px" />
}

export const DiffIcon = () => {
    return <DiffIconSvg width="45px" height="45px" />
}

export const CpuIcon = () => {
    return <CpuIconSvg width="24px" height="24px" />
}

export const FingerScanIcon = () => {
    return <FingerScanIconSvg width="24px" height="24px" />
}

export const MessageQuestionIcon = (props: { color: string }) => {
    return <svg width="24" height="24" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
        <path
            d="M17 2.42999H7C4 2.42999 2 4.42999 2 7.42999V13.43C2 16.43 4 18.43 7 18.43V20.56C7 21.36 7.89 21.84 8.55 21.39L13 18.43H17C20 18.43 22 16.43 22 13.43V7.42999C22 4.42999 20 2.42999 17 2.42999ZM12 14.6C11.58 14.6 11.25 14.26 11.25 13.85C11.25 13.44 11.58 13.1 12 13.1C12.42 13.1 12.75 13.44 12.75 13.85C12.75 14.26 12.42 14.6 12 14.6ZM13.26 10.45C12.87 10.71 12.75 10.88 12.75 11.16V11.37C12.75 11.78 12.41 12.12 12 12.12C11.59 12.12 11.25 11.78 11.25 11.37V11.16C11.25 9.99999 12.1 9.42999 12.42 9.20999C12.79 8.95999 12.91 8.78999 12.91 8.52999C12.91 8.02999 12.5 7.61999 12 7.61999C11.5 7.61999 11.09 8.02999 11.09 8.52999C11.09 8.93999 10.75 9.27999 10.34 9.27999C9.93 9.27999 9.59 8.93999 9.59 8.52999C9.59 7.19999 10.67 6.11999 12 6.11999C13.33 6.11999 14.41 7.19999 14.41 8.52999C14.41 9.66999 13.57 10.24 13.26 10.45Z"
            fill={props.color}
        />
    </svg>

}

export const TreeViewIcon = () => {
    return <TreeViewIconSvg width="26px" height="26px" />
}

export const CloseIcon = () => {
    return <CloseIconSvg width="24px" height="24px" />
}

export const CloseLightIcon = () => {
    return <CloseLightIconSvg width="24px" height="24px" />
}

export const WarningIcon = () => {
    return <WarningIconSvg width="24px" height="24px" />
}

export const ErrorIcon = () => {
    return <ErrorIconSvg width="24px" height="24px" />
}

export const TrashIcon = () => {
    return <TrashIconSvg width="24px" height="24px" />
}

export const OrphanIcon = () => {
    return <OrphanIconSvg width="24px" height="24px" />
}

export const AddedIcon = () => {
    return <AddedIconSvg width="24px" height="24px" />
}

export const ChangedIcon = () => {
    return <ChangedIconSvg width="24px" height="24px" />
}

export const CheckboxIcon = () => {
    return <CheckboxIconSvg width="24px" height="24px" />
}

export const CheckboxCheckedIcon = () => {
    return <CheckboxCheckedIconSvg width="24px" height="24px" />
}

export const CheckboxDisabledIcon = () => {
    return <CheckboxDisabledIconSvg width="24px" height="24px" />
}

export const ArrowLeftIcon = () => {
    return <ArrowLeftIconSvg width="40px" height="40px" />
}

export const WarningSignIcon = () => {
    return <WarningSignIconSvg width="21px" height="21px" />
}

export const ChangesIcon = () => {
    return <ChangesIconSvg width="21px" height="21px" />
}

export const StarIcon = () => {
    return <StarIconSvg width="21px" height="21px" />
}

export const TriangleDownIcon = () => {
    return <TriangleDownIconSvg width="50px" height="50px" />
}

export const TriangleLeftLightIcon = () => {
    return <TriangleLeftLightIconSvg width="50px" height="50px" />
}

export const TriangleRightLightIcon = () => {
    return <TriangleRightLightIconSvg width="50px" height="50px" />
}

export const TriangleRightIcon = () => {
    return <TriangleRightIconSvg width="50px" height="50px" />
}
export const BracketsCurlyIcon = () => {
    return <BracketsCurlyIconSvg width="22px" height="18px" />
}

export const BracketsSquareIcon = () => {
    return <BracketsSquareIconSvg width="22px" height="18px" />
}

export const FileIcon = () => {
    return <FileIconSvg width="40px" height="40px" />
}

export const ResultIcon = () => {
    return <ResultIconSvg width="30px" height="30px" />
}

export const IncludeIcon = () => {
    return <IncludeIconSvg width="30px" height="30px" />
}

export const LogoutIcon = () => {
    return <LogoutIconSvg width="40px" height="40px" />
}
