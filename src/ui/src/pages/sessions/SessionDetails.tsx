import { useState } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { formatDistanceToNow } from 'date-fns'
import { toast } from 'sonner'
import { $api } from '@/lib/api-client'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Separator } from '@/components/ui/separator'
import { 
  Card, 
  CardContent, 
  CardDescription, 
  CardFooter, 
  CardHeader, 
  CardTitle 
} from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Skeleton } from '@/components/ui/skeleton'
import { 
  Clock, 
  ArrowLeft, 
  RefreshCcw, 
  Power, 
  Cpu, 
  MemoryStick, 
  Network, 
  Activity
} from 'lucide-react'

// Status badge variants
const statusVariants = {
  'running': 'bg-green-500/10 text-green-500 hover:bg-green-500/20',
  'pending': 'bg-yellow-500/10 text-yellow-500 hover:bg-yellow-500/20',
  'terminating': 'bg-red-500/10 text-red-500 hover:bg-red-500/20',
  'terminated': 'bg-gray-500/10 text-gray-500 hover:bg-gray-500/20',
  'error': 'bg-red-500/10 text-red-500 hover:bg-red-500/20',
  'default': 'bg-gray-500/10 text-gray-500 hover:bg-gray-500/20'
}

export default function SessionDetails() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [showTerminateDialog, setShowTerminateDialog] = useState(false)
  const [activeTab, setActiveTab] = useState('overview')

  // Fetch session details
  const { data: session, isLoading: isLoadingSession, refetch: refetchSession } = useQuery({
    ...($api.get('/api/v1/sessions/{session_id}', {
      params: { path: { session_id: id! } },
    })),
    queryKey: ['session', id],
    enabled: !!id,
  })

  // Fetch session metrics
  const { data: metrics, isLoading: isLoadingMetrics } = useQuery({
    ...($api.get('/api/v1/metrics/session/{session_id}', {
      params: { path: { session_id: id! } },
    })),
    queryKey: ['session-metrics', id],
    enabled: !!id,
  })

  // Fetch session events
  const { data: events, isLoading: isLoadingEvents } = useQuery({
    ...($api.get('/api/v1/events/session/{session_id}', {
      params: { path: { session_id: id! } },
    })),
    queryKey: ['session-events', id],
    enabled: !!id,
  })

  // Session refresh mutation
  const refreshMutation = useMutation({
    mutationFn: () => 
      $api.post('/api/v1/sessions/{session_id}/refresh', {
        params: { path: { session_id: id! } },
      })(),
    onSuccess: () => {
      toast.success('Session refreshed successfully')
      refetchSession()
    },
    onError: (error) => {
      toast.error('Failed to refresh session', {
        description: error.message || 'Please try again',
      })
    },
  })

  // Session terminate mutation
  const terminateMutation = useMutation({
    mutationFn: (force: boolean = false) =>
      $api.delete('/api/v1/sessions/{session_id}', {
        params: { path: { session_id: id! } },
        query: { force }
      })(),
    onSuccess: () => {
      toast.success('Session terminated successfully')
      queryClient.invalidateQueries({ queryKey: ['sessions'] })
      navigate('/sessions')
    },
    onError: (error) => {
      toast.error('Failed to terminate session', {
        description: error.message || 'Please try again',
      })
    },
  })

  const handleRefresh = () => {
    refreshMutation.mutate()
  }

  const handleTerminate = (force: boolean = false) => {
    terminateMutation.mutate(force)
    setShowTerminateDialog(false)
  }

  const isRunning = session?.status === 'running'
  const isPending = session?.status === 'pending'
  const isTerminated = session?.status === 'terminated'

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div className="flex items-center gap-4">
          <Button
            variant="outline"
            size="icon"
            onClick={() => navigate('/sessions')}
            className="h-8 w-8"
          >
            <ArrowLeft className="h-4 w-4" />
          </Button>
          <div>
            <div className="flex items-center gap-2">
              <h1 className="text-2xl sm:text-3xl font-bold tracking-tight truncate">
                Session {id?.substring(0, 8)}
              </h1>
              {session?.status && (
                <Badge 
                  variant="outline" 
                  className={statusVariants[session.status as keyof typeof statusVariants] || statusVariants.default}
                >
                  {session.status}
                </Badge>
              )}
            </div>
            {session?.created_at && (
              <p className="text-muted-foreground">
                Created {formatDistanceToNow(new Date(session.created_at), { addSuffix: true })}
              </p>
            )}
          </div>
        </div>
        <div className="flex items-center gap-2">
          <Button 
            variant="outline" 
            size="sm"
            disabled={!isRunning || refreshMutation.isPending}
            onClick={handleRefresh}
          >
            <RefreshCcw className="h-4 w-4 mr-2" />
            Refresh
          </Button>
          
          <Dialog open={showTerminateDialog} onOpenChange={setShowTerminateDialog}>
            <DialogTrigger asChild>
              <Button 
                variant="destructive" 
                size="sm"
                disabled={isTerminated || terminateMutation.isPending}
              >
                <Power className="h-4 w-4 mr-2" />
                Terminate
              </Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Terminate Session</DialogTitle>
                <DialogDescription>
                  Are you sure you want to terminate this browser session? 
                  This action cannot be undone.
                </DialogDescription>
              </DialogHeader>
              <DialogFooter>
                <Button 
                  variant="outline" 
                  onClick={() => setShowTerminateDialog(false)}
                >
                  Cancel
                </Button>
                <Button 
                  variant="destructive" 
                  onClick={() => handleTerminate(false)}
                  disabled={terminateMutation.isPending}
                >
                  Terminate
                </Button>
                <Button 
                  variant="destructive" 
                  onClick={() => handleTerminate(true)}
                  disabled={terminateMutation.isPending}
                >
                  Force Terminate
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </div>
      </div>

      {/* Session Details Content */}
      <Tabs value={activeTab} onValueChange={setActiveTab}>
        <TabsList className="mb-4">
          <TabsTrigger value="overview">Overview</TabsTrigger>
          <TabsTrigger value="metrics">Metrics</TabsTrigger>
          <TabsTrigger value="events">Events</TabsTrigger>
          <TabsTrigger value="logs">Logs</TabsTrigger>
        </TabsList>

        {/* Overview Tab */}
        <TabsContent value="overview" className="space-y-6">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            {/* Connection Details */}
            <Card>
              <CardHeader className="pb-2">
                <CardTitle>Connection Details</CardTitle>
                <CardDescription>
                  Information needed to connect to the browser
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                {isLoadingSession ? (
                  <div className="space-y-4">
                    <Skeleton className="h-4 w-full" />
                    <Skeleton className="h-4 w-full" />
                    <Skeleton className="h-4 w-3/4" />
                  </div>
                ) : (
                  <>
                    <div className="grid grid-cols-3 gap-4">
                      <div className="space-y-1">
                        <p className="text-sm font-medium">Status</p>
                        <p className="text-sm">{session?.status || 'Unknown'}</p>
                      </div>
                      <div className="space-y-1">
                        <p className="text-sm font-medium">Browser</p>
                        <p className="text-sm">{session?.browser} {session?.browser_version}</p>
                      </div>
                      <div className="space-y-1">
                        <p className="text-sm font-medium">OS</p>
                        <p className="text-sm">{session?.operating_system || 'Unknown'}</p>
                      </div>
                    </div>

                    <Separator />

                    {session?.status === 'running' && session?.connection_details && (
                      <>
                        <div className="space-y-1">
                          <p className="text-sm font-medium">WebSocket Endpoint</p>
                          <p className="text-sm font-mono bg-secondary p-2 rounded-md overflow-x-auto">
                            {session.connection_details.websocket_url || 'Not available'}
                          </p>
                        </div>

                        <div className="space-y-1">
                          <p className="text-sm font-medium">Browser URL</p>
                          <p className="text-sm font-mono bg-secondary p-2 rounded-md overflow-x-auto">
                            {session.connection_details.browser_url || 'Not available'}
                          </p>
                        </div>
                      </>
                    )}

                    {isPending && (
                      <div className="flex items-center justify-center p-4">
                        <p className="text-sm text-muted-foreground">
                          Session is being provisioned...
                        </p>
                      </div>
                    )}
                  </>
                )}
              </CardContent>
            </Card>

            {/* Configuration */}
            <Card>
              <CardHeader className="pb-2">
                <CardTitle>Configuration</CardTitle>
                <CardDescription>
                  Browser session configuration
                </CardDescription>
              </CardHeader>
              <CardContent>
                {isLoadingSession ? (
                  <div className="space-y-4">
                    <Skeleton className="h-4 w-full" />
                    <Skeleton className="h-4 w-full" />
                    <Skeleton className="h-4 w-3/4" />
                  </div>
                ) : (
                  <>
                    <div className="space-y-4">
                      <div className="grid grid-cols-2 gap-4">
                        <div className="space-y-1">
                          <p className="text-sm font-medium">Screen Resolution</p>
                          <p className="text-sm">
                            {session?.config?.screen?.width || '1280'}x{session?.config?.screen?.height || '720'}
                          </p>
                        </div>
                        <div className="space-y-1">
                          <p className="text-sm font-medium">DPI</p>
                          <p className="text-sm">{session?.config?.screen?.dpi || '96'}</p>
                        </div>
                      </div>

                      <Separator />

                      <div className="grid grid-cols-2 gap-4">
                        <div className="space-y-1">
                          <p className="text-sm font-medium">Headless</p>
                          <p className="text-sm">{session?.config?.headless ? 'Yes' : 'No'}</p>
                        </div>
                        <div className="space-y-1">
                          <p className="text-sm font-medium">Timeout</p>
                          <p className="text-sm">
                            {session?.config?.resource_limits?.timeout_minutes || '30'} minutes
                          </p>
                        </div>
                      </div>

                      <Separator />

                      <div className="grid grid-cols-2 gap-4">
                        <div className="space-y-1">
                          <p className="text-sm font-medium">Record Network</p>
                          <p className="text-sm">{session?.config?.record_network ? 'Yes' : 'No'}</p>
                        </div>
                        <div className="space-y-1">
                          <p className="text-sm font-medium">Record Console</p>
                          <p className="text-sm">{session?.config?.record_console ? 'Yes' : 'No'}</p>
                        </div>
                      </div>
                    </div>
                  </>
                )}
              </CardContent>
            </Card>
          </div>

          {/* Resource Usage */}
          <Card>
            <CardHeader>
              <CardTitle>Resource Usage</CardTitle>
              <CardDescription>
                Current resource usage for this session
              </CardDescription>
            </CardHeader>
            <CardContent>
              {isLoadingMetrics || !metrics || metrics.length === 0 ? (
                <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
                  <div className="flex flex-col space-y-2 p-4 border rounded-lg">
                    <div className="flex items-center text-muted-foreground">
                      <Cpu className="mr-2 h-4 w-4" />
                      <span>CPU Usage</span>
                    </div>
                    <Skeleton className="h-6 w-16" />
                  </div>
                  
                  <div className="flex flex-col space-y-2 p-4 border rounded-lg">
                    <div className="flex items-center text-muted-foreground">
                      <MemoryStick className="mr-2 h-4 w-4" />
                      <span>Memory Usage</span>
                    </div>
                    <Skeleton className="h-6 w-16" />
                  </div>
                  
                  <div className="flex flex-col space-y-2 p-4 border rounded-lg">
                    <div className="flex items-center text-muted-foreground">
                      <Network className="mr-2 h-4 w-4" />
                      <span>Network I/O</span>
                    </div>
                    <Skeleton className="h-6 w-16" />
                  </div>
                </div>
              ) : (
                <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
                  <div className="flex flex-col space-y-2 p-4 border rounded-lg">
                    <div className="flex items-center text-muted-foreground">
                      <Cpu className="mr-2 h-4 w-4" />
                      <span>CPU Usage</span>
                    </div>
                    <div className="text-2xl font-bold">
                      {metrics[0].cpu_percent}%
                    </div>
                  </div>
                  
                  <div className="flex flex-col space-y-2 p-4 border rounded-lg">
                    <div className="flex items-center text-muted-foreground">
                      <MemoryStick className="mr-2 h-4 w-4" />
                      <span>Memory Usage</span>
                    </div>
                    <div className="text-2xl font-bold">
                      {(metrics[0].memory_mb / 1024).toFixed(1)} GB
                    </div>
                  </div>
                  
                  <div className="flex flex-col space-y-2 p-4 border rounded-lg">
                    <div className="flex items-center text-muted-foreground">
                      <Network className="mr-2 h-4 w-4" />
                      <span>Network I/O</span>
                    </div>
                    <div className="text-2xl font-bold">
                      {(metrics[0].network_rx_bytes / 1024 / 1024).toFixed(1)} MB
                    </div>
                  </div>
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        {/* Metrics Tab */}
        <TabsContent value="metrics">
          <Card>
            <CardHeader>
              <CardTitle>Resource Metrics</CardTitle>
              <CardDescription>
                Historical resource usage for this session
              </CardDescription>
            </CardHeader>
            <CardContent className="h-96">
              {isLoadingMetrics ? (
                <div className="h-full flex items-center justify-center">
                  <p className="text-muted-foreground">Loading metrics...</p>
                </div>
              ) : metrics && metrics.length > 0 ? (
                <div className="space-y-4">
                  <p className="text-muted-foreground">
                    Metrics are available. In a real implementation, this would display charts of CPU, memory, and network usage over time.
                  </p>

                  <div className="border rounded-md p-4">
                    <p className="font-medium mb-2">Raw Metrics Data:</p>
                    <ScrollArea className="h-64">
                      <pre className="text-xs font-mono">
                        {JSON.stringify(metrics, null, 2)}
                      </pre>
                    </ScrollArea>
                  </div>
                </div>
              ) : (
                <div className="h-full flex items-center justify-center">
                  <p className="text-muted-foreground">No metrics available for this session.</p>
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        {/* Events Tab */}
        <TabsContent value="events">
          <Card>
            <CardHeader>
              <CardTitle>Session Events</CardTitle>
              <CardDescription>
                Timeline of events for this browser session
              </CardDescription>
            </CardHeader>
            <CardContent>
              {isLoadingEvents ? (
                <div className="space-y-4">
                  {Array(3).fill(0).map((_, i) => (
                    <div key={i} className="flex items-center gap-4">
                      <Skeleton className="h-10 w-10 rounded-full" />
                      <div className="space-y-2">
                        <Skeleton className="h-4 w-[250px]" />
                        <Skeleton className="h-4 w-[200px]" />
                      </div>
                    </div>
                  ))}
                </div>
              ) : events && events.length > 0 ? (
                <div className="space-y-6">
                  {events.map((event) => (
                    <div key={event.id} className="flex gap-4">
                      <div className="mt-1">
                        <div className="flex h-8 w-8 items-center justify-center rounded-full bg-primary/10">
                          <Activity className="h-4 w-4 text-primary" />
                        </div>
                      </div>
                      <div className="space-y-1">
                        <div className="flex items-center gap-2">
                          <p className="font-medium">
                            {event.event_type}
                          </p>
                          <p className="text-sm text-muted-foreground">
                            {formatDistanceToNow(new Date(event.timestamp), { addSuffix: true })}
                          </p>
                        </div>
                        <p className="text-sm text-muted-foreground">
                          {event.description || 'No description provided'}
                        </p>
                        {event.details && (
                          <pre className="mt-2 rounded-md bg-secondary p-2 text-xs font-mono overflow-x-auto">
                            {JSON.stringify(event.details, null, 2)}
                          </pre>
                        )}
                      </div>
                    </div>
                  ))}
                </div>
              ) : (
                <div className="flex flex-col items-center justify-center py-8">
                  <p className="text-muted-foreground">No events recorded for this session.</p>
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        {/* Logs Tab */}
        <TabsContent value="logs">
          <Card>
            <CardHeader>
              <CardTitle>Browser Logs</CardTitle>
              <CardDescription>
                Console and network logs from the browser session
              </CardDescription>
            </CardHeader>
            <CardContent>
              <div className="flex flex-col items-center justify-center py-8">
                <p className="text-muted-foreground">
                  Browser logs would be displayed here. This feature is not implemented in this demo.
                </p>
              </div>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  )
} 