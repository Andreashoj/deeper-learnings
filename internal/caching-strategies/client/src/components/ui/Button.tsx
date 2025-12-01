import type {ReactNode} from "react";

function Button({children, onClick}: { children: ReactNode, onClick?: () =>void}) {
    return (
        <button onClick={onClick} className="px-4 py-2 bg-green-400 rounded-lg text-white cursor-pointer">
            { children }
        </button>
    )
}

export default Button