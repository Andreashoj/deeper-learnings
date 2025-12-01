import type {ReactElement} from "react";

function Card({children}: { children: ReactElement[] }) {
    return (
        <div>
            {children}
        </div>
    )
}

export default Card

