import { useState } from 'react'
import { Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import {$api} from '../../lib/api-client'
import { Button } from '../../components/ui/button'
import { Input } from '../../components/ui/input'
import { 
  Select, 
  SelectContent, 
  SelectItem, 
  SelectTrigger, 
  SelectValue 
} from '../../components/ui/select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '../../components/ui/table'
import {
  Pagination,
  PaginationContent,
  PaginationItem,
  PaginationLink,
  PaginationNext,
  PaginationPrevious,
} from '../../components/ui/pagination'
import { Skeleton } from '../../components/ui/skeleton'
import { Badge } from '../../components/ui/badge'
import { formatDistanceToNow } from 'date-fns'
import { Plus, RefreshCw, Search } from 'lucide-react'

// Status badge variants
const statusVariants = {
  'running': 'bg-green-500/10 text-green-500 hover:bg-green-500/20',
  'pending': 'bg-yellow-500/10 text-yellow-500 hover:bg-yellow-500/20',
  'terminating': 'bg-red-500/10 text-red-500 hover:bg-red-500/20',
  'terminated': 'bg-gray-500/10 text-gray-500 hover:bg-gray-500/20',
  'error': 'bg-red-500/10 text-red-500 hover:bg-red-500/20',
  'default': 'bg-gray-500/10 text-gray-500 hover:bg-gray-500/20'
}

export default function Sessions() {
  const [filter, setFilter] = useState('')
  const [statusFilter, setStatusFilter] = useState('')
  const [page, setPage] = useState(1)
  const pageSize = 10

  // Fetch sessions with optional filtering
  const { data: sessions, isLoading, refetch } = useQuery({
    ...($api.get('/api/v1/sessions/', {
      query: {
        status: statusFilter || undefined,
        offset: (page - 1) * pageSize,
        limit: pageSize
      }
    })),
    queryKey: ['sessions', statusFilter, page],
  })

  // Filter sessions by search term (client-side filtering)
  const filteredSessions = !sessions ? [] : sessions.filter((session) => {
    if (!filter) return true
    const searchTerm = filter.toLowerCase()
    return (
      session.id.toLowerCase().includes(searchTerm) ||
      session.browser?.toLowerCase().includes(searchTerm) ||
      session.status?.toLowerCase().includes(searchTerm)
    )
  })

  // TODO: Implement proper pagination with API
  const totalPages = Math.ceil((filteredSessions?.length || 0) / pageSize)

  const handleRefresh = () => {
    refetch()
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <h1 className="text-3xl font-bold tracking-tight">Browser Sessions</h1>
        <div className="flex items-center gap-2">
          <Button onClick={handleRefresh} variant="outline" size="icon">
            <RefreshCw className="h-4 w-4" />
          </Button>
          <Button asChild>
            <Link to="/sessions/new">
              <Plus className="h-4 w-4 mr-2" />
              New Session
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
            placeholder="Search sessions..."
            className="pl-8"
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
          />
        </div>
        <Select value={statusFilter} onValueChange={setStatusFilter}>
          <SelectTrigger className="w-36 sm:w-44">
            <SelectValue placeholder="Status" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="">All Statuses</SelectItem>
            <SelectItem value="pending">Pending</SelectItem>
            <SelectItem value="running">Running</SelectItem>
            <SelectItem value="terminating">Terminating</SelectItem>
            <SelectItem value="terminated">Terminated</SelectItem>
            <SelectItem value="error">Error</SelectItem>
          </SelectContent>
        </Select>
      </div>

      {/* Sessions Table */}
      <div className="border rounded-md">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>ID</TableHead>
              <TableHead>Status</TableHead>
              <TableHead>Browser</TableHead>
              <TableHead className="hidden md:table-cell">Created</TableHead>
              <TableHead className="hidden md:table-cell">Expires</TableHead>
              <TableHead className="text-right">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              Array(5).fill(0).map((_, i) => (
                <TableRow key={i}>
                  <TableCell><Skeleton className="h-5 w-24" /></TableCell>
                  <TableCell><Skeleton className="h-5 w-20" /></TableCell>
                  <TableCell><Skeleton className="h-5 w-32" /></TableCell>
                  <TableCell className="hidden md:table-cell"><Skeleton className="h-5 w-24" /></TableCell>
                  <TableCell className="hidden md:table-cell"><Skeleton className="h-5 w-24" /></TableCell>
                  <TableCell className="text-right"><Skeleton className="h-9 w-20 ml-auto" /></TableCell>
                </TableRow>
              ))
            ) : filteredSessions.length === 0 ? (
              <TableRow>
                <TableCell colSpan={6} className="h-24 text-center">
                  No sessions found.
                </TableCell>
              </TableRow>
            ) : (
              filteredSessions.map((session) => (
                <TableRow key={session.id}>
                  <TableCell className="font-medium">
                    <Link to={`/sessions/${session.id}`} className="hover:underline">
                      {session.id.substring(0, 8)}
                    </Link>
                  </TableCell>
                  <TableCell>
                    <Badge 
                      variant="outline" 
                      className={statusVariants[session.status as keyof typeof statusVariants] || statusVariants.default}
                    >
                      {session.status}
                    </Badge>
                  </TableCell>
                  <TableCell>
                    {session.browser} {session.browser_version}
                  </TableCell>
                  <TableCell className="hidden md:table-cell">
                    {formatDistanceToNow(new Date(session.created_at), { addSuffix: true })}
                  </TableCell>
                  <TableCell className="hidden md:table-cell">
                    {session.expires_at ? 
                      formatDistanceToNow(new Date(session.expires_at), { addSuffix: true }) : 
                      'N/A'}
                  </TableCell>
                  <TableCell className="text-right">
                    <Button asChild variant="outline" size="sm">
                      <Link to={`/sessions/${session.id}`}>
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