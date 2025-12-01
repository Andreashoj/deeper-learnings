import Button from "./Button.tsx";
import {useMutation} from "@tanstack/react-query";
import {useState} from "react";

export interface QueryStatsProps {
    method: string;
    url: string;
    headers?: Record<string, string>;
    body?: any;
}

function QueryStats({ method, url, body, headers }: QueryStatsProps ) {
    const [duration, setDuration] = useState<string | null>(null)
    const {mutate} = useMutation({
        mutationFn: async () => {
            const start = performance.now()
            const res = await fetch("http://localhost:8080" + url, {
                method: method,
                headers: headers,
                body: body
            })

            const end = performance.now()
            setDuration(((end - start) / 1000).toFixed(4))
            return res.json()
        },
    })

    return (
        <>
            <div className="bg-gray-800 text-white rounded-lg p-4 mb-2 text-left">
                <QueryStatsDetails method={method} url={url} headers={headers} body={body} duration={duration}/>

                <div className="bg-gray-700 rounded p-4 mt-2 font-serif">
                    Headers:
                    { "  {  }" }
                    <span className="my-4 block"/>

                    Payload:
                    { "  {  }" }
                </div>
            </div>
            <div className="flex justify-end mb-8">
                <Button onClick={mutate}>
                    submit
                </Button>
            </div>
        </>
    )
}

function QueryStatsDetails({method, url, duration}: QueryStatsProps & { duration: string | null }) {
    return (
        <ul className="w-full">
            <li className="flex">
                <span className="w-20">
                    Method:
                </span>
                { method}
            </li>
            <li className="flex">
                <span className="w-20">
                    Request:
                </span>
                { url }
            </li>
            <li className="flex">
                <span className="w-20">
                    Duration:
                </span>
                { duration }{ duration ? "s" : null}
            </li>
        </ul>
    )
}

export default QueryStats