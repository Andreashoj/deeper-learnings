import QueryStats from "../components/ui/QueryStats.tsx";

function Dashboard() {
    return (
        <div className="flex flex-col overflow-hidden">
            <QueryStats method="GET" url="/api/cache/hit" />
            <QueryStats method="GET" url="/api/no-cache/hit" />
            <QueryStats method="GET" url="/api/no-cache/posts" />
            <QueryStats method="GET" url="/api/cache/posts" />
            <QueryStats method="GET" url="/api/cache/hit" />
            <QueryStats method="GET" url="/api/cache/hit" />
        </div>
    )
}

export default Dashboard