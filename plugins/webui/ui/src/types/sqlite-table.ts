export type SqliteTableItem = {
  name: string
  type: string
  rowCount: number
  columnCount: number
  primaryKeys: string[]
  primaryCount: number
}

export type SqliteTableDetail = {
  name: string
  type: string
  columns: string[]
  primaryKeys: string[]
  rows: Record<string, unknown>[]
  rowCount: number
  page: number
  perPage: number
}
