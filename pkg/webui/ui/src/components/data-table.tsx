import type { ReactNode } from "react"
import { flexRender, type Table as TableModel } from "@tanstack/react-table"
import {
  ArrowDown,
  ArrowUp,
  ArrowUpDown,
  ChevronLeft,
  ChevronRight,
} from "lucide-react"
import { Button } from "@/components/ui/button"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { cn } from "@/lib/utils"

// shadcn Data Table: shadcn Table primitives, with TanStack providing the row models.
export function DataTable<T>({
  table,
  empty,
  onOpen,
  active,
  label,
}: {
  table: TableModel<T>
  empty: ReactNode
  onOpen?: (row: T) => void
  active?: string
  label: string
}) {
  const rows = table.getRowModel().rows
  return (
    <div className="space-y-3">
      <div className="rounded-md border bg-background">
        <Table aria-label={label}>
          <TableHeader className="bg-muted/65">
            {table.getHeaderGroups().map((group) => (
              <TableRow key={group.id}>
                {group.headers.map((header) => {
                  const direction = header.column.getIsSorted()
                  const text = flexRender(
                    header.column.columnDef.header,
                    header.getContext()
                  )
                  const Icon =
                    direction === "asc"
                      ? ArrowUp
                      : direction === "desc"
                        ? ArrowDown
                        : ArrowUpDown
                  return (
                    <TableHead
                      key={header.id}
                      aria-sort={
                        direction === "asc"
                          ? "ascending"
                          : direction === "desc"
                            ? "descending"
                            : undefined
                      }
                    >
                      {header.isPlaceholder ? null : header.column.getCanSort() ? (
                        <Button
                          variant="ghost"
                          size="sm"
                          className="-ml-2.5"
                          onClick={header.column.getToggleSortingHandler()}
                        >
                          {text}
                          <Icon className="opacity-60" />
                        </Button>
                      ) : (
                        text
                      )}
                    </TableHead>
                  )
                })}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody>
            {rows.length ? (
              rows.map((row) => (
                <TableRow
                  key={row.id}
                  data-state={row.id === active ? "selected" : undefined}
                  className={cn(onOpen && "cursor-pointer")}
                  onClick={
                    onOpen
                      ? (e) => {
                          if (
                            !(e.target as HTMLElement).closest(
                              "a, button, input, [role=button]"
                            ) &&
                            !window.getSelection()?.toString()
                          )
                            onOpen(row.original)
                        }
                      : undefined
                  }
                >
                  {row.getVisibleCells().map((cell) => (
                    <TableCell key={cell.id}>
                      {flexRender(
                        cell.column.columnDef.cell,
                        cell.getContext()
                      )}
                    </TableCell>
                  ))}
                </TableRow>
              ))
            ) : (
              <TableRow>
                <TableCell
                  colSpan={table.getVisibleLeafColumns().length}
                  className="h-28 text-center text-muted-foreground"
                >
                  {empty}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>
    </div>
  )
}

export function Pagination<T>({
  table,
  noun,
}: {
  table: TableModel<T>
  noun: string
}) {
  const count = table.getPrePaginationRowModel().rows.length
  const { pageIndex, pageSize } = table.getState().pagination
  const start = pageIndex * pageSize + 1
  return (
    <div className="flex flex-wrap items-center justify-between gap-2 text-sm text-muted-foreground">
      <span aria-live="polite">
        {count
          ? `${start}–${Math.min(start + pageSize - 1, count)} of ${count}`
          : "0"}{" "}
        {noun}
      </span>
      <div className="flex items-center gap-2">
        <Button
          variant="outline"
          size="sm"
          aria-label="Previous page"
          onClick={() => table.previousPage()}
          disabled={!table.getCanPreviousPage()}
        >
          <ChevronLeft />
        </Button>
        <Button
          variant="outline"
          size="sm"
          aria-label="Next page"
          onClick={() => table.nextPage()}
          disabled={!table.getCanNextPage()}
        >
          <ChevronRight />
        </Button>
      </div>
    </div>
  )
}
