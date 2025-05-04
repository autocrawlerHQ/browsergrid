import React, { useState } from 'react';
import { 
  ChevronLeft, 
  Layers, 
  Code, 
  Server, 
  Cpu, 
  Globe, 
  BarChart3, 
  Plus, 
  PlayCircle, 
  Settings, 
  Terminal, 
  Webhook, 
  ExternalLink,
  Clock, 
  LayoutGrid,
  RefreshCw,
  Shield,
  Zap,
  MonitorPlay,
  Trash,
  Edit,
  X,
  Lock,
  AlertTriangle,
  Pickaxe,
  
} from 'lucide-react';
import { Link } from 'react-router-dom';
import { 
  Card, 
  CardContent, 
  CardFooter, 
  CardHeader 
} from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { $api } from '@/lib/api-client';
import { useQuery } from '@tanstack/react-query';
import { components } from '@/lib/api';

export default function BrowsergridDashboard() {
  const [activeTab, setActiveTab] = useState('overview');

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

  const browserSessions = [
    { 
      id: 'sess_12345', 
      browser: 'Chrome', 
      version: '118.0.5993.117', 
      status: 'Running', 
      clients: 3,
      startedAt: '2025-04-17T09:15:30Z',
      resource: { cpu: 1, memory: '2G' }
    },
    { 
      id: 'sess_23456', 
      browser: 'Firefox', 
      version: '119.0', 
      status: 'Running', 
      clients: 1,
      startedAt: '2025-04-17T10:25:45Z',
      resource: { cpu: 1, memory: '2G' }
    },
    { 
      id: 'sess_34567', 
      browser: 'WebKit', 
      version: 'latest', 
      status: 'Terminated', 
      clients: 0,
      startedAt: '2025-04-16T14:30:00Z',
      resource: { cpu: 1, memory: '1G' }
    },
    { 
      id: 'sess_45678', 
      browser: 'Chromium', 
      version: 'latest', 
      status: 'Pending', 
      clients: 0,
      startedAt: '2025-04-17T11:45:10Z',
      resource: { cpu: 2, memory: '4G' }
    }
  ];

  const pools = [
    { name: 'CI Testing', browser: 'Chrome', workers: 5, sessions: 2, status: 'Active' },
    { name: 'Development', browser: 'Firefox', workers: 3, sessions: 1, status: 'Active' },
    { name: 'E2E Tests', browser: 'Multiple', workers: 8, sessions: 0, status: 'Idle' }
  ];

  const workers = [
    { id: 'worker_123', pool: 'CI Testing', status: 'Online', sessions: 2, uptime: '12h 34m' },
    { id: 'worker_456', pool: 'Development', status: 'Online', sessions: 1, uptime: '5h 12m' },
    { id: 'worker_789', pool: 'E2E Tests', status: 'Online', sessions: 0, uptime: '8h 45m' }
  ];

  const webhooks = [
    { name: 'CI Pipeline', event: 'session.created', url: 'https://ci.example.com/hooks/browsergrid', status: 'Active' },
    { name: 'Error Logger', event: 'browser.error', url: 'https://logs.example.com/api/errors', status: 'Active' }
  ];

  const StatusBadge = ({ status }) => {
    const variants = {
      Running: "bg-green-500/10 text-green-500 hover:bg-green-500/20",
      Online: "bg-green-500/10 text-green-500 hover:bg-green-500/20",
      Active: "bg-green-500/10 text-green-500 hover:bg-green-500/20",
      Idle: "bg-blue-500/10 text-blue-500 hover:bg-blue-500/20",
      Terminated: "bg-gray-500/10 text-gray-500 hover:bg-gray-500/20",
      Pending: "bg-yellow-500/10 text-yellow-500 hover:bg-yellow-500/20"
    };
    
    return (
      <Badge variant="secondary" className={`px-1.5 py-0.5 text-xs font-normal ${variants[status] || ""}`}>
        {status}
      </Badge>
    );
  };

  const timeAgo = (dateString) => {
    const date = new Date(dateString);
    const now = new Date();
    const diffMs = now - date;
    const diffMins = Math.floor(diffMs / 60000);
    
    if (diffMins < 60) return `${diffMins}m ago`;
    if (diffMins < 1440) return `${Math.floor(diffMins / 60)}h ago`;
    return `${Math.floor(diffMins / 1440)}d ago`;
  };

  return (
    <div className="bg-background flex flex-grow flex-col">
      <div className="flex w-full flex-grow flex-col">
        <div className="before:bg-secondary/40 before:border-muted relative flex h-full w-full flex-col items-center justify-center p-8 before:absolute before:top-[293px] before:left-0 before:z-0 before:h-[calc(100%-293px)] before:w-full before:border-t before:border-b before:dark:bg-black">
          <div className="z-10 flex min-h-screen h-full w-full max-w-7xl flex-col items-stretch justify-start">
            <div className="mb-6 flex items-start justify-between">
              <div>
                <Link to="/" className="text-primary/70 mb-4 flex items-center gap-2 text-sm hover:underline">
                  <ChevronLeft className="h-3 w-3" /> Back to Home
                </Link>
                <h1 className="mb-2 text-4xl font-bold">Browsergrid</h1>
                <p className="text-primary/70 mb-6 text-sm">
                  Browser infrastructure for automation, testing, and development
                </p>
                
                <div className="mb-6 flex space-x-2">
                  <Button className="bg-blue-600 text-white hover:bg-blue-700">
                    <PlayCircle className="mr-2 h-4 w-4" />
                    Launch Browser
                  </Button>
                  <Button variant="outline">
                    <Plus className="mr-2 h-4 w-4" />
                    New Work Pool
                  </Button>
                  <Button variant="outline">
                    <Settings className="mr-2 h-4 w-4" />
                    Settings
                  </Button>
                </div>
              </div>

            </div>
            
            {/* Tabs */}
            <div className="mb-6">
              <div className="flex space-x-6">
                {[
                  { id: 'overview', label: 'Overview', icon: LayoutGrid },
                  { id: 'sessions', label: 'Browser Sessions', icon: Globe },
                  { id: 'pools', label: 'Work Pools', icon: Layers },
                  { id: 'workers', label: 'Workers', icon: Pickaxe },
                  { id: 'webhooks', label: 'Webhooks', icon: Webhook }
                ].map((tab) => (
                  <button
                    key={tab.id}
                    onClick={() => setActiveTab(tab.id)}
                    className={`flex items-center gap-2 border-b-2 px-1 py-3 text-sm font-medium transition-all ${
                      activeTab === tab.id 
                        ? "border-blue-600 text-blue-600 dark:border-blue-400 dark:text-blue-400" 
                        : "border-transparent text-gray-500 hover:border-gray-300 hover:text-gray-700 dark:text-gray-400 dark:hover:border-gray-600 dark:hover:text-gray-300"
                    }`}
                  >
                    <tab.icon className="h-4 w-4" />
                    {tab.label}
                  </button>
                ))}
              </div>
            </div>
            
            {/* Main content */}
            {activeTab === 'overview' && (
              <div className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-3">
                <Card>
                  <CardHeader className="pb-2">
                    <div className="flex items-center justify-between">
                      <h3 className="text-sm font-medium">Active Sessions</h3>
                      <Globe className="h-4 w-4 text-blue-500" />
                    </div>
                  </CardHeader>
                  <CardContent className="pb-2">
                    <div className="flex items-baseline justify-between">
                      <span className="text-3xl font-bold">3</span>
                      <Badge variant="outline" className="text-xs">
                        <RefreshCw className="mr-1 h-3 w-3" />
                        Updated just now
                      </Badge>
                    </div>
                  </CardContent>
                  <CardFooter className="pt-0">
                    <div className="flex w-full justify-between text-xs text-gray-500">
                      <span>2 Chrome, 1 Firefox</span>
                      <Link to="/" className="text-blue-500 hover:underline" onClick={() => setActiveTab('sessions')}>View all</Link>
                    </div>
                  </CardFooter>
                </Card>
                
                <Card>
                  <CardHeader className="pb-2">
                    <div className="flex items-center justify-between">
                      <h3 className="text-sm font-medium">Work Pools</h3>
                      <Layers className="h-4 w-4 text-purple-500" />
                    </div>
                  </CardHeader>
                  <CardContent className="pb-2">
                    <div className="flex items-baseline justify-between">
                      <span className="text-3xl font-bold">3</span>
                      <div className="flex gap-2">
                        <Badge variant="secondary" className="bg-green-50 text-green-600 dark:bg-green-900/20 dark:text-green-400">2 Active</Badge>
                        <Badge variant="secondary" className="bg-blue-50 text-blue-600 dark:bg-blue-900/20 dark:text-blue-400">1 Idle</Badge>
                      </div>
                    </div>
                  </CardContent>
                  <CardFooter className="pt-0">
                    <div className="flex w-full justify-between text-xs text-gray-500">
                      <span>16 Workers total</span>
                      <Link to="/" className="text-blue-500 hover:underline" onClick={() => setActiveTab('pools')}>Manage pools</Link>
                    </div>
                  </CardFooter>
                </Card>
                
                <Card>
                  <CardHeader className="pb-2">
                    <div className="flex items-center justify-between">
                      <h3 className="text-sm font-medium">System Resources</h3>
                      <Cpu className="h-4 w-4 text-emerald-500" />
                    </div>
                  </CardHeader>
                  <CardContent className="pb-2">
                    <div className="space-y-2">
                      <div className="flex items-center justify-between">
                        <span className="text-xs font-medium">CPU Usage</span>
                        <span className="text-xs">32%</span>
                      </div>
                      <div className="h-2 w-full overflow-hidden rounded-full bg-gray-100 dark:bg-gray-800">
                        <div className="h-full w-[32%] rounded-full bg-emerald-500"></div>
                      </div>
                      
                      <div className="flex items-center justify-between">
                        <span className="text-xs font-medium">Memory Usage</span>
                        <span className="text-xs">47%</span>
                      </div>
                      <div className="h-2 w-full overflow-hidden rounded-full bg-gray-100 dark:bg-gray-800">
                        <div className="h-full w-[47%] rounded-full bg-blue-500"></div>
                      </div>
                      
                      <div className="flex items-center justify-between">
                        <span className="text-xs font-medium">Disk Usage</span>
                        <span className="text-xs">18%</span>
                      </div>
                      <div className="h-2 w-full overflow-hidden rounded-full bg-gray-100 dark:bg-gray-800">
                        <div className="h-full w-[18%] rounded-full bg-purple-500"></div>
                      </div>
                    </div>
                  </CardContent>
                </Card>
                
                <Card>
                  <CardHeader className="pb-2">
                    <div className="flex items-center justify-between">
                      <h3 className="text-sm font-medium">Browser Usage</h3>
                      <BarChart3 className="h-4 w-4 text-blue-500" />
                    </div>
                  </CardHeader>
                  <CardContent>
                    <div className="space-y-4">
                      <div className="space-y-2">
                        <div className="flex items-center justify-between">
                          <span className="flex items-center text-xs">
                            <span className="mr-2 h-2 w-2 rounded-full bg-blue-500"></span>
                            Chrome
                          </span>
                          <span className="text-xs">67%</span>
                        </div>
                        <div className="h-2 w-full overflow-hidden rounded-full bg-gray-100 dark:bg-gray-800">
                          <div className="h-full w-[67%] rounded-full bg-blue-500"></div>
                        </div>
                      </div>
                      
                      <div className="space-y-2">
                        <div className="flex items-center justify-between">
                          <span className="flex items-center text-xs">
                            <span className="mr-2 h-2 w-2 rounded-full bg-orange-500"></span>
                            Firefox
                          </span>
                          <span className="text-xs">23%</span>
                        </div>
                        <div className="h-2 w-full overflow-hidden rounded-full bg-gray-100 dark:bg-gray-800">
                          <div className="h-full w-[23%] rounded-full bg-orange-500"></div>
                        </div>
                      </div>
                      
                      <div className="space-y-2">
                        <div className="flex items-center justify-between">
                          <span className="flex items-center text-xs">
                            <span className="mr-2 h-2 w-2 rounded-full bg-purple-500"></span>
                            WebKit
                          </span>
                          <span className="text-xs">10%</span>
                        </div>
                        <div className="h-2 w-full overflow-hidden rounded-full bg-gray-100 dark:bg-gray-800">
                          <div className="h-full w-[10%] rounded-full bg-purple-500"></div>
                        </div>
                      </div>
                    </div>
                  </CardContent>
                </Card>
                
                <Card className="md:col-span-2">
                  <CardHeader className="pb-2">
                    <div className="flex items-center justify-between">
                      <h3 className="text-sm font-medium">Recent Activity</h3>
                      <Clock className="h-4 w-4 text-gray-500" />
                    </div>
                  </CardHeader>
                  <CardContent>
                    <div className="space-y-4">
                      {[
                        { action: "Browser session created", details: "Chrome 118.0.5993.117", time: "2 minutes ago", icon: Globe, color: "text-blue-500" },
                        { action: "Worker joined pool", details: "worker_456 joined Development pool", time: "5 hours ago", icon: Server, color: "text-green-500" },
                        { action: "Webhook triggered", details: "CI Pipeline on session.created", time: "6 hours ago", icon: Webhook, color: "text-purple-500" },
                        { action: "Browser session terminated", details: "WebKit session_34567", time: "1 day ago", icon: Globe, color: "text-red-500" }
                      ].map((activity, idx) => (
                        <div key={idx} className="flex items-start gap-3">
                          <div className={`flex h-8 w-8 items-center justify-center rounded-full bg-gray-100 ${activity.color} dark:bg-gray-800`}>
                            <activity.icon className="h-4 w-4" />
                          </div>
                          <div className="flex-1">
                            <p className="text-sm font-medium">{activity.action}</p>
                            <p className="text-xs text-gray-500">{activity.details}</p>
                          </div>
                          <span className="text-xs text-gray-500">{activity.time}</span>
                        </div>
                      ))}
                    </div>
                  </CardContent>
                </Card>
                
                <Card>
                  <CardHeader className="pb-2">
                    <div className="flex items-center justify-between">
                      <h3 className="text-sm font-medium">Quick Actions</h3>
                      <Zap className="h-4 w-4 text-amber-500" />
                    </div>
                  </CardHeader>
                  <CardContent>
                    <div className="grid grid-cols-2 gap-2">
                      <Button variant="outline" size="sm" className="h-auto justify-start py-3">
                        <PlayCircle className="mr-2 h-4 w-4" />
                        <div className="flex flex-col items-start">
                          <span className="text-xs font-medium">Launch Chrome</span>
                          <span className="text-[10px] text-gray-500">Latest version</span>
                        </div>
                      </Button>
                      
                      <Button variant="outline" size="sm" className="h-auto justify-start py-3">
                        <MonitorPlay className="mr-2 h-4 w-4" />
                        <div className="flex flex-col items-start">
                          <span className="text-xs font-medium">Create Session</span>
                          <span className="text-[10px] text-gray-500">With screenshot</span>
                        </div>
                      </Button>
                      
                      <Button variant="outline" size="sm" className="h-auto justify-start py-3">
                        <Terminal className="mr-2 h-4 w-4" />
                        <div className="flex flex-col items-start">
                          <span className="text-xs font-medium">Run Script</span>
                          <span className="text-[10px] text-gray-500">Execute automation</span>
                        </div>
                      </Button>
                      
                      <Button variant="outline" size="sm" className="h-auto justify-start py-3">
                        <RefreshCw className="mr-2 h-4 w-4" />
                        <div className="flex flex-col items-start">
                          <span className="text-xs font-medium">Restart Worker</span>
                          <span className="text-[10px] text-gray-500">Reset connections</span>
                        </div>
                      </Button>
                    </div>
                  </CardContent>
                </Card>
              </div>
            )}
            
            {activeTab === 'sessions' && (
              <div className="space-y-6">
                <div className="flex items-center justify-between">
                  <h2 className="text-xl font-bold">Browser Sessions</h2>
                  <Button className="bg-blue-600 text-white hover:bg-blue-700">
                    <Plus className="mr-2 h-4 w-4" />
                    New Session
                  </Button>
                </div>
                
                <div className="rounded-lg border">
                  <div className="grid grid-cols-7 gap-4 border-b bg-gray-50 p-4 text-xs font-medium text-gray-500 dark:bg-gray-800/50 dark:text-gray-400">
                    <div>ID</div>
                    <div>Browser</div>
                    <div>Status</div>
                    <div>Clients</div>
                    <div>Resources</div>
                    <div>Started</div>
                    <div className="text-right">Actions</div>
                  </div>
                  
                  {browserSessions.map((session, idx) => (
                    <div key={idx} className={`grid grid-cols-7 gap-4 p-4 text-sm ${idx < browserSessions.length - 1 ? 'border-b' : ''}`}>
                      <div className="font-mono text-xs">{session.id}</div>
                      <div className="flex items-center">
                        <span>{session.browser}</span>
                        <Badge variant="outline" className="ml-2 text-[10px]">{session.version}</Badge>
                      </div>
                      <div><StatusBadge status={session.status} /></div>
                      <div>{session.clients}</div>
                      <div className="text-xs text-gray-500">
                        {session.resource.cpu} CPU, {session.resource.memory} RAM
                      </div>
                      <div className="text-xs text-gray-500">{timeAgo(session.startedAt)}</div>
                      <div className="flex justify-end space-x-2">
                        <Button variant="ghost" size="sm" disabled={session.status !== 'Running'}>
                          <ExternalLink className="h-3 w-3" />
                        </Button>
                        <Button variant="ghost" size="sm" disabled={session.status !== 'Running'}>
                          <Terminal className="h-3 w-3" />
                        </Button>
                        <Button variant="ghost" size="sm" className="text-red-500 hover:bg-red-50 hover:text-red-600" disabled={session.status === 'Terminated'}>
                          <Trash className="h-3 w-3" />
                        </Button>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )}
            
            {activeTab === 'pools' && (
              <div className="space-y-6">
                <div className="flex items-center justify-between">
                  <h2 className="text-xl font-bold">Work Pools</h2>
                  <Button className="bg-blue-600 text-white hover:bg-blue-700">
                    <Plus className="mr-2 h-4 w-4" />
                    New Pool
                  </Button>
                </div>
                
                <div className="grid grid-cols-1 gap-4 lg:grid-cols-3">
                  {pools.map((pool, idx) => (
                    <Card key={idx} className="group relative overflow-hidden transition-all duration-200 hover:shadow-md">
                      <div className="absolute inset-0 bg-gradient-to-r from-blue-500/0 via-blue-500/5 to-blue-500/0 opacity-0 transition-opacity duration-200 group-hover:opacity-100"></div>
                      <CardHeader className="pb-2">
                        <div className="flex items-center justify-between">
                          <h3 className="text-base font-medium">{pool.name}</h3>
                          <StatusBadge status={pool.status} />
                        </div>
                      </CardHeader>
                      <CardContent className="pb-3">
                        <div className="mb-3 mt-1 space-y-2">
                          <div className="flex items-center justify-between">
                            <span className="text-xs text-gray-500">Primary Browser</span>
                            <span className="text-xs font-medium">{pool.browser}</span>
                          </div>
                          <div className="flex items-center justify-between">
                            <span className="text-xs text-gray-500">Workers</span>
                            <span className="text-xs font-medium">{pool.workers}</span>
                          </div>
                          <div className="flex items-center justify-between">
                            <span className="text-xs text-gray-500">Active Sessions</span>
                            <span className="text-xs font-medium">{pool.sessions}</span>
                          </div>
                        </div>
                      </CardContent>
                      <CardFooter className="flex justify-between pt-0">
                        <Button variant="outline" size="sm">
                          <Server className="mr-2 h-3 w-3" />
                          Manage
                        </Button>
                        <Button variant="outline" size="sm">
                          <PlayCircle className="mr-2 h-3 w-3" />
                          Launch Browser
                        </Button>
                      </CardFooter>
                    </Card>
                  ))}
                </div>
              </div>
            )}
            
            {activeTab === 'workers' && (
              <div className="space-y-6">
                <div className="flex items-center justify-between">
                  <h2 className="text-xl font-bold">Workers</h2>
                  <Button className="bg-blue-600 text-white hover:bg-blue-700">
                    <Plus className="mr-2 h-4 w-4" />
                    New Worker
                  </Button>
                </div>
                
                <div className="rounded-lg border">
                  <div className="grid grid-cols-5 gap-4 border-b bg-gray-50 p-4 text-xs font-medium text-gray-500 dark:bg-gray-800/50 dark:text-gray-400">
                    <div>ID</div>
                    <div>Pool</div>
                    <div>Status</div>
                    <div>Sessions</div>
                    <div className="text-right">Actions</div>
                  </div>
                  
                  {workers.map((worker, idx) => (
                    <div key={idx} className={`grid grid-cols-5 gap-4 p-4 text-sm ${idx < workers.length - 1 ? 'border-b' : ''}`}>
                      <div className="font-mono text-xs">{worker.id}</div>
                      <div>{worker.pool}</div>
                      <div>
                        <StatusBadge status={worker.status} />
                        <span className="ml-2 text-xs text-gray-500">Uptime: {worker.uptime}</span>
                      </div>
                      <div>{worker.sessions} active</div>
                      <div className="flex justify-end space-x-2">
                        <Button variant="ghost" size="sm">
                          <Terminal className="h-3 w-3" />
                        </Button>
                        <Button variant="ghost" size="sm">
                          <RefreshCw className="h-3 w-3" />
                        </Button>
                        <Button variant="ghost" size="sm" className="text-red-500 hover:bg-red-50 hover:text-red-600">
                          <X className="h-3 w-3" />
                        </Button>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )}
            
            {activeTab === 'webhooks' && (
              <div className="space-y-6">
                <div className="flex items-center justify-between">
                  <h2 className="text-xl font-bold">Webhooks</h2>
                  <Button className="bg-blue-600 text-white hover:bg-blue-700">
                    <Plus className="mr-2 h-4 w-4" />
                    New Webhook
                  </Button>
                </div>
                
                <div className="rounded-lg border">
                  <div className="grid grid-cols-4 gap-4 border-b bg-gray-50 p-4 text-xs font-medium text-gray-500 dark:bg-gray-800/50 dark:text-gray-400">
                    <div>Name</div>
                    <div>Event</div>
                    <div>URL</div>
                    <div className="text-right">Actions</div>
                  </div>
                  
                  {webhooks.map((webhook, idx) => (
                    <div key={idx} className={`grid grid-cols-4 gap-4 p-4 text-sm ${idx < webhooks.length - 1 ? 'border-b' : ''}`}>
                      <div>{webhook.name}</div>
                      <div>
                        <Badge variant="outline" className="font-mono text-xs">{webhook.event}</Badge>
                      </div>
                      <div className="truncate font-mono text-xs">{webhook.url}</div>
                      <div className="flex justify-end space-x-2">
                        <Button variant="ghost" size="sm">
                          <Edit className="h-3 w-3" />
                        </Button>
                        <Button variant="ghost" size="sm">
                          <RefreshCw className="h-3 w-3" />
                        </Button>
                        <Button variant="ghost" size="sm" className="text-red-500 hover:bg-red-50 hover:text-red-600">
                          <Trash className="h-3 w-3" />
                        </Button>
                      </div>
                    </div>
                  ))}
                </div>
                
                <Card className="bg-blue-50 border-blue-100 dark:bg-blue-900/20 dark:border-blue-800/40">
                  <CardContent className="flex items-start gap-4 p-4">
                    <div className="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-full bg-blue-100 text-blue-600 dark:bg-blue-800 dark:text-blue-300">
                      <Shield className="h-4 w-4" />
                    </div>
                    <div>
                      <h3 className="mb-1 text-sm font-medium text-blue-800 dark:text-blue-300">Webhook Security</h3>
                      <p className="text-xs text-blue-600 dark:text-blue-400">
                        Browsergrid signs all webhook payloads with an HMAC-SHA256 signature. Verify this signature
                        to ensure the webhook was sent by Browsergrid.
                      </p>
                      <Button variant="link" className="mt-1 h-auto p-0 text-xs text-blue-600 hover:text-blue-800 dark:text-blue-400 dark:hover:text-blue-300">
                        Learn more about webhook security
                      </Button>
                    </div>
                  </CardContent>
                </Card>
                
                {/* CDP Authentication Section */}
                <div className="space-y-4">
                  <h3 className="text-lg font-medium">CDP Authentication</h3>
                  
                  <Card>
                    <CardHeader className="pb-2">
                      <div className="flex items-center justify-between">
                        <h3 className="text-sm font-medium">Authentication Settings</h3>
                        <Lock className="h-4 w-4 text-blue-500" />
                      </div>
                    </CardHeader>
                    <CardContent>
                      <div className="space-y-4">
                        <div className="flex items-start gap-4">
                          <div className="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-full bg-amber-100 text-amber-600 dark:bg-amber-900/50 dark:text-amber-400">
                            <AlertTriangle className="h-4 w-4" />
                          </div>
                          <div className="flex-1">
                            <p className="text-sm text-amber-800 dark:text-amber-200">
                              CDP access requires authentication. Configure your auth method below.
                            </p>
                          </div>
                        </div>
                        
                        <div className="grid grid-cols-1 gap-3 lg:grid-cols-2">
                          <Card className="border-border/50 bg-background/50 hover:border-blue-500/30 group cursor-pointer transition-all duration-200">
                            <CardContent className="flex items-start gap-3 p-4">
                              <div className="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-full bg-gray-100 text-gray-500 dark:bg-gray-800">
                                <span className="text-xs font-bold">1</span>
                              </div>
                              <div>
                                <h4 className="text-sm font-medium group-hover:text-blue-600 transition-colors">API Key Authentication</h4>
                                <p className="text-xs text-gray-500 mt-1">
                                  Use API keys to authenticate CDP connections. Best for scripted clients.
                                </p>
                              </div>
                            </CardContent>
                          </Card>
                          
                          <Card className="border-border/50 bg-background/50 hover:border-blue-500/30 group cursor-pointer transition-all duration-200">
                            <CardContent className="flex items-start gap-3 p-4">
                              <div className="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-full bg-gray-100 text-gray-500 dark:bg-gray-800">
                                <span className="text-xs font-bold">2</span>
                              </div>
                              <div>
                                <h4 className="text-sm font-medium group-hover:text-blue-600 transition-colors">JWT Authentication</h4>
                                <p className="text-xs text-gray-500 mt-1">
                                  Use JWTs for temporary access tokens. Best for OAuth integrations.
                                </p>
                              </div>
                            </CardContent>
                          </Card>
                        </div>
                      </div>
                    </CardContent>
                  </Card>
                </div>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}