function Input({ readonly, value }: { readonly: boolean, value: string}) {
    return (
        <input className="text-white" readOnly={readonly} value={value} />
    )
}

export default Input