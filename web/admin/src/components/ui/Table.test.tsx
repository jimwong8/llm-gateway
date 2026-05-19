import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Table } from './Table'

const columns = [
  { key: 'name', header: '名称' },
  { key: 'status', header: '状态' },
]

const data = [
  { id: 1, name: '项目 A', status: '活跃' },
  { id: 2, name: '项目 B', status: '停用' },
]

describe('Table', () => {
  it('renders headers and data', () => {
    render(<Table columns={columns} data={data} keyExtractor={(d) => d.id} />)
    expect(screen.getByText('名称')).toBeInTheDocument()
    expect(screen.getByText('项目 A')).toBeInTheDocument()
    expect(screen.getByText('项目 B')).toBeInTheDocument()
  })

  it('renders empty state', () => {
    render(<Table columns={columns} data={[]} keyExtractor={(d) => d.id} />)
    expect(screen.getByText('暂无数据')).toBeInTheDocument()
  })

  it('renders custom empty text', () => {
    render(<Table columns={columns} data={[]} keyExtractor={(d) => d.id} emptyText="空" />)
    expect(screen.getByText('空')).toBeInTheDocument()
  })

  it('calls onRowClick when row clicked', async () => {
    const user = userEvent.setup()
    const onRowClick = vi.fn()
    render(<Table columns={columns} data={data} keyExtractor={(d) => d.id} onRowClick={onRowClick} />)
    await user.click(screen.getByText('项目 A'))
    expect(onRowClick).toHaveBeenCalledWith(data[0])
  })

  it('uses custom render function', () => {
    const cols = [
      { key: 'name', header: '名称', render: (item: any) => <strong>{item.name}</strong> },
    ]
    render(<Table columns={cols} data={data} keyExtractor={(d) => d.id} />)
    expect(screen.getByText('项目 A').tagName).toBe('STRONG')
  })

  it('merges custom className', () => {
    const { container } = render(
      <Table columns={columns} data={data} keyExtractor={(d) => d.id} className="custom" />,
    )
    expect(container.firstChild).toHaveClass('custom')
  })
})
