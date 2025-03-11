import { Link } from 'react-router-dom'

import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Skeleton } from '@/components/ui/skeleton'
import { Activity, Server, GitBranch, Clock, Box } from 'lucide-react'

import { useQuery } from '@tanstack/react-query'
import {$api} from '@/lib/api-client'
import { components } from '@/lib/api'

export default function Dashboard() {
  // Fetch system metrics overview
  const { data: systemMetrics, isLoading: isLoadingMetrics } = useQuery<components['schemas']['SessionMetrics']>({
    ...($api.GET('/api/v1/workerpools/metrics/system/overview', {})),
    queryKey: ['system-metrics-overview'],
  })
  
  // Fetch sessions
  const { data: sessions, isLoading: isLoadingSessions } = useQuery<components['schemas']['Session'][]>({
    ...($api.GET('/api/v1/sessions/', {})),
    queryKey: ['sessions'],
  })

  // Fetch work pools
  const { data: workPools, isLoading: isLoadingWorkPools } = useQuery<components['schemas']['WorkPool'][]>({
    ...($api.GET('/api/v1/workerpools/pools', {})),
    queryKey: ['workpools'],
  })

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-3xl font-bold tracking-tight">Dashboard</h1>
        <Button asChild>
          <Link to="/sessions/new">New Session</Link>
        </Button>
      </div>

      {/* Stats Cards */}
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatsCard 
          title="Active Sessions"
          value={isLoadingSessions ? null : sessions?.length ?? 0}
          description="Currently running browser sessions"
          icon={<Activity className="h-5 w-5 text-blue-600" />}
          linkTo="/sessions"
        />
        <StatsCard 
          title="Work Pools"
          value={isLoadingWorkPools ? null : workPools?.length ?? 0}
          description="Available compute resources"
          icon={<Server className="h-5 w-5 text-green-600" />}
          linkTo="/workpools"
        />
        <StatsCard 
          title="Workers"
          value={isLoadingWorkPools ? null : 
            workPools?.reduce((acc, pool) => acc + (pool.workers.length || 0), 0) ?? 0}
          description="Connected worker nodes"
          icon={<Box className="h-5 w-5 text-purple-600" />}
          linkTo="/workers"
        />
        <StatsCard 
          title="System Uptime"
          value={isLoadingMetrics ? null : "96.8%"}
          description="Last 30 days"
          icon={<Clock className="h-5 w-5 text-orange-600" />}
          linkTo="#"
          showLink={false}
        />
      </div>

      {/* Tabs Content */}
      <Tabs defaultValue="overview" className="mt-6">
        <TabsList className="mb-4">
          <TabsTrigger value="overview">Overview</TabsTrigger>
          <TabsTrigger value="sessions">Sessions</TabsTrigger>
          <TabsTrigger value="resources">Resources</TabsTrigger>
        </TabsList>
        
        <TabsContent value="overview" className="space-y-4">
          <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="text-sm font-medium">CPU Usage</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">
                  {isLoadingMetrics ? (
                    <Skeleton className="h-8 w-24" />
                  ) : (
                    "47%"
                  )}
                </div>
                <p className="text-xs text-muted-foreground">
                  +2.5% from last hour
                </p>
                <div className="mt-4 h-3 w-full rounded-full bg-secondary">
                  <div className="h-3 w-[47%] rounded-full bg-primary"></div>
                </div>
              </CardContent>
            </Card>
            
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="text-sm font-medium">Memory Usage</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">
                  {isLoadingMetrics ? (
                    <Skeleton className="h-8 w-24" />
                  ) : (
                    "6.2 GB"
                  )}
                </div>
                <p className="text-xs text-muted-foreground">
                  +512 MB from last hour
                </p>
                <div className="mt-4 h-3 w-full rounded-full bg-secondary">
                  <div className="h-3 w-[65%] rounded-full bg-primary"></div>
                </div>
              </CardContent>
            </Card>
            
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="text-sm font-medium">Network I/O</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">
                  {isLoadingMetrics ? (
                    <Skeleton className="h-8 w-24" />
                  ) : (
                    "3.4 MB/s"
                  )}
                </div>
                <p className="text-xs text-muted-foreground">
                  -0.3 MB/s from last hour
                </p>
                <div className="mt-4 h-3 w-full rounded-full bg-secondary">
                  <div className="h-3 w-[34%] rounded-full bg-primary"></div>
                </div>
              </CardContent>
            </Card>
          </div>

          <Card>
            <CardHeader>
              <CardTitle>Recent Activity</CardTitle>
              <CardDescription>
                Recent session events and system activities
              </CardDescription>
            </CardHeader>
            <CardContent>
              {isLoadingSessions || !sessions ? (
                <div className="space-y-4">
                  {Array(5).fill(0).map((_, i) => (
                    <div key={i} className="flex items-center gap-4">
                      <Skeleton className="h-12 w-12 rounded-full" />
                      <div className="space-y-2">
                        <Skeleton className="h-4 w-[250px]" />
                        <Skeleton className="h-4 w-[200px]" />
                      </div>
                    </div>
                  ))}
                </div>
              ) : sessions.length === 0 ? (
                <p className="text-muted-foreground">No recent activity</p>
              ) : (
                <div className="space-y-4">
                  {sessions.slice(0, 5).map((session) => (
                    <div key={session.id} className="flex flex-col space-y-1">
                      <div className="flex items-center gap-2">
                        <span className="font-medium">{session.id.substring(0, 8)}</span>
                        <span className="text-sm text-muted-foreground">
                          {new Date(session.created_at).toLocaleTimeString()}
                        </span>
                      </div>
                      <p className="text-sm">
                        {session.status} - {session.browser} {session.version}
                      </p>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
            <CardFooter>
              <Button asChild variant="outline" size="sm" className="ml-auto">
                <Link to="/sessions">View All Sessions</Link>
              </Button>
            </CardFooter>
          </Card>
        </TabsContent>
        
        <TabsContent value="sessions">
          <Card>
            <CardHeader>
              <CardTitle>Session Overview</CardTitle>
              <CardDescription>
                Status and details of all browser sessions
              </CardDescription>
            </CardHeader>
            <CardContent>
              {/* Session content will be here */}
              <p className="text-muted-foreground">Detailed session information is available on the Sessions page.</p>
            </CardContent>
            <CardFooter>
              <Button asChild variant="outline" size="sm" className="ml-auto">
                <Link to="/sessions">View All Sessions</Link>
              </Button>
            </CardFooter>
          </Card>
        </TabsContent>
        
        <TabsContent value="resources">
          <Card>
            <CardHeader>
              <CardTitle>Resource Allocation</CardTitle>
              <CardDescription>
                System resource allocation and availability
              </CardDescription>
            </CardHeader>
            <CardContent>
              {/* Resource content will be here */}
              <p className="text-muted-foreground">Detailed resource information is available on the Work Pools page.</p>
            </CardContent>
            <CardFooter>
              <Button asChild variant="outline" size="sm" className="ml-auto">
                <Link to="/workpools">View Work Pools</Link>
              </Button>
            </CardFooter>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  )
}

function StatsCard({ 
  title, 
  value, 
  description, 
  icon, 
  linkTo = "#",
  showLink = true
}: { 
  title: string
  value: number | string | null
  description: string
  icon: React.ReactNode
  linkTo?: string
  showLink?: boolean
}) {
  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between pb-2">
        <CardTitle className="text-sm font-medium">{title}</CardTitle>
        {icon}
      </CardHeader>
      <CardContent>
        <div className="text-2xl font-bold">
          {value === null ? <Skeleton className="h-8 w-16" /> : value}
        </div>
        <p className="text-xs text-muted-foreground">{description}</p>
      </CardContent>
      {showLink && (
        <CardFooter className="p-2">
          <Link
            to={linkTo}
            className="text-xs text-muted-foreground hover:text-primary w-full text-right"
          >
            View details
          </Link>
        </CardFooter>
      )}
    </Card>
  )
} 