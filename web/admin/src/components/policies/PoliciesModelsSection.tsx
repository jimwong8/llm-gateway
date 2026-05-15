type Props = {
  models: string[]
}

export function PoliciesModelsSection({ models }: Props) {
  return (
    <div className="event-table">
      <table>
        <thead>
          <tr>
            <th>Model</th>
          </tr>
        </thead>
        <tbody>
          {models.map((model) => (
            <tr key={model}>
              <td>{model}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
