import { useState } from 'react'
import { Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import {$api} from '@/lib/api-client'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Pagination,
  PaginationContent,
  PaginationItem,
  PaginationLink,
  PaginationNext,
  PaginationPrevious,
} from '@/components/ui/pagination'
import { Skeleton } from '@/components/ui/skeleton'
import { Badge } from '@/components/ui/badge'
import { formatDistanceToNow } from 'date-fns'
import { Plus, RefreshCw, Search, Globe, Webhook } from 'lucide-react'

export default function Webhooks() {
  const [filter, setFilter] = useState('')
  const [page, setPage] = useState(1)
  const pageSize = 10

  // Fetch webhooks
  const { data: webhooks, isLoading, refetch } = useQuery({
    ...($api.get('/api/v1/webhooks/')),
    queryKey: ['webhooks', page],
  })

  // Filter webhooks by search term (client-side filtering)
  const filteredWebhooks = !webhooks ? [] : webhooks.filter((webhook) => {
    if (!filter) return true
    const searchTerm = filter.toLowerCase()
    return (
      webhook.id.toLowerCase().includes(searchTerm) ||
      webhook.name.toLowerCase().includes(searchTerm) ||
      webhook.session_id.toLowerCase().includes(searchTerm)
    )
  })

  // TODO: Implement proper pagination with API
  const totalPages = Math.ceil((filteredWebhooks?.length || 0) / pageSize)
  const paginatedWebhooks = filteredWebhooks.slice((page - 1) * pageSize, page * pageSize)

  const handleRefresh = () => {
    refetch()
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <h1 className="text-3xl font-bold tracking-tight">Webhooks</h1>
        <div className="flex items-center gap-2">
          <Button onClick={handleRefresh} variant="outline" size="icon">
            <RefreshCw className="h-4 w-4" />
          </Button>
          <Button asChild>
            <Link to="/webhooks/new">
              <Plus className="h-4 w-4 mr-2" />
              New Webhook
            </Link>
          </Button>
        </div>
      </div>

      {/* Filters */}
      <div className="flex flex-col sm:flex-row gap-4">
        <div className="relative flex-1">
          <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
          <Input
            type="search"
            placeholder="Search webhooks..."
            className="pl-8"
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
          />
        </div>
      </div>

      {/* Webhooks Table */}
      <div className="border rounded-md">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Status</TableHead>
              <TableHead className="hidden md:table-cell">Event Pattern</TableHead>
              <TableHead className="hidden md:table-cell">Session</TableHead>
              <TableHead className="text-right">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              Array(5).fill(0).map((_, i) => (
                <TableRow key={i}>
                  <TableCell><Skeleton className="h-5 w-32" /></TableCell>
                  <TableCell><Skeleton className="h-5 w-20" /></TableCell>
                  <TableCell className="hidden md:table-cell"><Skeleton className="h-5 w-32" /></TableCell>
                  <TableCell className="hidden md:table-cell"><Skeleton className="h-5 w-24" /></TableCell>
                  <TableCell className="text-right"><Skeleton className="h-9 w-20 ml-auto" /></TableCell>
                </TableRow>
              ))
            ) : paginatedWebhooks.length === 0 ? (
              <TableRow>
                <TableCell colSpan={5} className="h-24 text-center">
                  No webhooks found.
                </TableCell>
              </TableRow>
            ) : (
              paginatedWebhooks.map((webhook) => (
                <TableRow key={webhook.id}>
                  <TableCell className="font-medium">
                    <Link to={`/webhooks/${webhook.id}`} className="flex items-center gap-2 hover:underline">
                      <Webhook className="h-4 w-4 text-primary" />
                      {webhook.name}
                    </Link>
                  </TableCell>
                  <TableCell>
                    <Badge 
                      variant="outline" 
                      className={webhook.active ? 
                        "bg-green-500/10 text-green-500 hover:bg-green-500/20" : 
                        "bg-gray-500/10 text-gray-500 hover:bg-gray-500/20"}
                    >
                      {webhook.active ? 'Active' : 'Inactive'}
                    </Badge>
                  </TableCell>
                  <TableCell className="hidden md:table-cell">
                    <div className="flex items-center">
                      <code className="text-xs bg-secondary px-1 py-0.5 rounded">
                        {webhook.event_pattern?.method || 'Any'}
                      </code>
                    </div>
                  </TableCell>
                  <TableCell className="hidden md:table-cell">
                    <Link 
                      to={`/sessions/${webhook.session_id}`} 
                      className="text-sm text-muted-foreground hover:text-primary hover:underline"
                    >
                      {webhook.session_id.substring(0, 8)}
                    </Link>
                  </TableCell>
                  <TableCell className="text-right">
                    <Button asChild variant="outline" size="sm">
                      <Link to={`/webhooks/${webhook.id}`}>
                        View
                      </Link>
                    </Button>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>

      {/* Pagination */}
      {!isLoading && totalPages > 1 && (
        <Pagination>
          <PaginationContent>
            <PaginationItem>
              <PaginationPrevious 
                onClick={() => setPage(p => Math.max(1, p - 1))}
                className={page === 1 ? 'pointer-events-none opacity-50' : ''}
              />
            </PaginationItem>
            {Array.from({ length: totalPages }).map((_, i) => (
              <PaginationItem key={i}>
                <PaginationLink
                  onClick={() => setPage(i + 1)}
                  isActive={page === i + 1}
                >
                  {i + 1}
                </PaginationLink>
              </PaginationItem>
            ))}
            <PaginationItem>
              <PaginationNext 
                onClick={() => setPage(p => Math.min(totalPages, p + 1))}
                className={page === totalPages ? 'pointer-events-none opacity-50' : ''}
              />
            </PaginationItem>
          </PaginationContent>
        </Pagination>
      )}
    </div>
  )
} 