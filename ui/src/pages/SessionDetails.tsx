import React from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { ArrowLeft, ExternalLink, Settings, Activity, RefreshCw } from 'lucide-react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Separator } from '@/components/ui/separator';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Switch } from '@/components/ui/switch';
import { StatusBadge } from '@/components/dashboard/StatusBadge';
import { LiveVNCFrame } from '@/components/LiveVNCFrame';
import { useGetApiV1SessionsIdEvents } from '@/lib/api/events/events';
import { processVncUrl } from '@/lib/utils';
import type { Session, SessionEvent } from '@/lib/api/model';
import { useGetApiV1SessionsId } from '@/lib/api/sessions/sessions';

export default function SessionDetails() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  
  // For now, we'll need to get the session data from the sessions list
  // In a real implementation, you'd have a dedicated API endpoint for getting a single session
  const { data: eventsData, isLoading: eventsLoading } = useGetApiV1SessionsIdEvents(
    id || '', 
    { limit: 50, offset: 0 }
  );

  const { data: session, isLoading: sessionLoading } = useGetApiV1SessionsId(id || ''); 


  if (!id) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-center">
          <p className="text-sm text-neutral-600">Session ID not found</p>
          <Button onClick={() => navigate('/sessions')} variant="outline" className="mt-2">
            Back to Sessions
          </Button>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <Button 
            onClick={() => navigate('/sessions')} 
            variant="ghost" 
            size="sm"
            className="h-8 w-8 p-0"
          >
            <ArrowLeft className="h-4 w-4" />
          </Button>
          <div>
            <h1 className="text-2xl font-semibold text-neutral-900 tracking-tight">
              Session Details
            </h1>
            <p className="text-sm text-neutral-600 mt-1">
              {session.id?.substring(0, 8)}... • {session.browser} {session.version}
            </p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          {session.live_url && (
            <Button size="sm" asChild className="bg-neutral-900 hover:bg-neutral-800 text-white text-xs h-8">
              <a href={processVncUrl(session.live_url)} target="_blank" rel="noopener noreferrer">
                <ExternalLink className="h-3 w-3 mr-1.5" />
                Open Live Session
              </a>
            </Button>
          )}
          <Button size="sm" variant="outline" className="border-neutral-200 hover:bg-neutral-50 text-xs h-8">
            <Settings className="h-3 w-3 mr-1.5" />
            Configure
          </Button>
        </div>
      </div>

      {/* Session Details Content */}
      <Tabs defaultValue="overview" className="w-full">
        <TabsList className="grid w-full grid-cols-5 bg-neutral-50 border border-neutral-200 h-9">
          <TabsTrigger value="overview" className="data-[state=active]:bg-white data-[state=active]:border-neutral-200 text-xs">Overview</TabsTrigger>
          <TabsTrigger value="live" className="data-[state=active]:bg-white data-[state=active]:border-neutral-200 text-xs">Live View</TabsTrigger>
          <TabsTrigger value="events" className="data-[state=active]:bg-white data-[state=active]:border-neutral-200 text-xs">
            Events
            {eventsData?.events && (
              <Badge variant="secondary" className="ml-1.5 text-xs bg-neutral-100 text-neutral-600 px-1 py-0">
                {eventsData.events.length}
              </Badge>
            )}
          </TabsTrigger>
          <TabsTrigger value="metrics" className="data-[state=active]:bg-white data-[state=active]:border-neutral-200 text-xs">Metrics</TabsTrigger>
          <TabsTrigger value="settings" className="data-[state=active]:bg-white data-[state=active]:border-neutral-200 text-xs">Settings</TabsTrigger>
        </TabsList>
        
        <TabsContent value="overview" className="space-y-4 mt-4">
          <div className="grid gap-4 md:grid-cols-2">
            <Card className="border-neutral-200">
              <CardHeader className="pb-2">
                <CardTitle className="text-sm font-medium text-neutral-900">Session Information</CardTitle>
              </CardHeader>
              <CardContent className="space-y-2">
                <div className="flex justify-between py-1">
                  <span className="text-xs text-neutral-600">ID</span>
                  <span className="font-mono text-xs text-neutral-900">{session.id}</span>
                </div>
                <div className="flex justify-between py-1">
                  <span className="text-xs text-neutral-600">Status</span>
                  <StatusBadge status={session.status || 'unknown'} />
                </div>
                <div className="flex justify-between py-1">
                  <span className="text-xs text-neutral-600">Created</span>
                  <span className="text-xs text-neutral-900">{session.created_at ? new Date(session.created_at).toLocaleString() : 'N/A'}</span>
                </div>
                <div className="flex justify-between py-1">
                  <span className="text-xs text-neutral-600">Provider</span>
                  <span className="text-xs text-neutral-900">{session.provider}</span>
                </div>
                {session.expires_at && (
                  <div className="flex justify-between py-1">
                    <span className="text-xs text-neutral-600">Expires</span>
                    <span className="text-xs text-neutral-900">{new Date(session.expires_at).toLocaleString()}</span>
                  </div>
                )}
              </CardContent>
            </Card>
            
            <Card className="border-neutral-200">
              <CardHeader className="pb-2">
                <CardTitle className="text-sm font-medium text-neutral-900">Browser Configuration</CardTitle>
              </CardHeader>
              <CardContent className="space-y-2">
                <div className="flex justify-between py-1">
                  <span className="text-xs text-neutral-600">Browser</span>
                  <span className="text-xs text-neutral-900">{session.browser} {session.version}</span>
                </div>
                <div className="flex justify-between py-1">
                  <span className="text-xs text-neutral-600">OS</span>
                  <span className="text-xs text-neutral-900">{session.operating_system}</span>
                </div>
                <div className="flex justify-between py-1">
                  <span className="text-xs text-neutral-600">Mode</span>
                  <span className="text-xs text-neutral-900">{session.headless ? 'Headless' : 'GUI'}</span>
                </div>
                <div className="flex justify-between py-1">
                  <span className="text-xs text-neutral-600">Screen</span>
                  <span className="text-xs text-neutral-900">{session.screen?.width}×{session.screen?.height}</span>
                </div>
                {session.ws_endpoint && (
                  <div className="flex justify-between py-1">
                    <span className="text-xs text-neutral-600">WebSocket</span>
                    <span className="text-xs font-mono bg-neutral-100 px-1.5 py-0.5 rounded text-neutral-700">
                      {session.ws_endpoint.length > 30 ? 
                        `${session.ws_endpoint.substring(0, 30)}...` : 
                        session.ws_endpoint
                      }
                    </span>
                  </div>
                )}
              </CardContent>
            </Card>
          </div>
          
          <div className="grid gap-4 md:grid-cols-2">
            {session.live_url && (
              <Card className="border-neutral-200">
                <CardHeader className="pb-2">
                  <CardTitle className="text-sm font-medium text-neutral-900">Quick Actions</CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="flex gap-2">
                    <Button size="sm" asChild className="bg-neutral-900 hover:bg-neutral-800 text-white text-xs h-7">
                      <a href={processVncUrl(session.live_url)} target="_blank" rel="noopener noreferrer">
                        <ExternalLink className="h-3 w-3 mr-1.5" />
                        Open Live Session
                      </a>
                    </Button>
                    <Button size="sm" variant="outline" className="border-neutral-200 hover:bg-neutral-50 text-xs h-7">
                      <Settings className="h-3 w-3 mr-1.5" />
                      Configure
                    </Button>
                  </div>
                </CardContent>
              </Card>
            )}
            
            {(session.claimed_by || session.worker_id) && (
              <Card className="border-neutral-200">
                <CardHeader className="pb-2">
                  <CardTitle className="text-sm font-medium text-neutral-900">Assignment Details</CardTitle>
                </CardHeader>
                <CardContent className="space-y-2">
                  {session.claimed_by && (
                    <div className="flex justify-between py-1">
                      <span className="text-xs text-neutral-600">Claimed By</span>
                      <span className="text-xs text-neutral-900">{session.claimed_by}</span>
                    </div>
                  )}
                  {session.claimed_at && (
                    <div className="flex justify-between py-1">
                      <span className="text-xs text-neutral-600">Claimed At</span>
                      <span className="text-xs text-neutral-900">{new Date(session.claimed_at).toLocaleString()}</span>
                    </div>
                  )}
                  {session.worker_id && (
                    <div className="flex justify-between py-1">
                      <span className="text-xs text-neutral-600">Worker</span>
                      <span className="font-mono text-xs text-neutral-900">{session.worker_id.substring(0, 8)}...</span>
                    </div>
                  )}
                </CardContent>
              </Card>
            )}
          </div>
        </TabsContent>
        
        <TabsContent value="live" className="mt-4">
          <Card className="border-neutral-200">
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium text-neutral-900">Live Browser View</CardTitle>
              <p className="text-xs text-neutral-600">
                Real-time view of the browser session. Click and interact directly with the browser.
              </p>
            </CardHeader>
            <CardContent className="p-0">
              <LiveVNCFrame 
                sessionId={session.id || ''} 
                liveUrl={processVncUrl(session.live_url)}
                className="h-[600px]"
              />
            </CardContent>
          </Card>
        </TabsContent>
        
        <TabsContent value="events" className="mt-4">
          <SessionEventsTimeline 
            sessionId={session.id || ''} 
            events={eventsData?.events || []}
            isLoading={eventsLoading}
          />
        </TabsContent>
        
        <TabsContent value="metrics" className="mt-4">
          <SessionMetricsView sessionId={session.id || ''} />
        </TabsContent>
        
        <TabsContent value="settings" className="mt-4">
          <Card className="border-neutral-200">
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium text-neutral-900">Session Settings</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="space-y-4">
                <div className="flex items-center justify-between">
                  <div>
                    <p className="text-xs font-medium text-neutral-900">Webhooks</p>
                    <p className="text-xs text-neutral-600">Receive notifications for session events</p>
                  </div>
                  <Switch checked={session.webhooks_enabled} />
                </div>
                <Separator className="bg-neutral-200" />
                <div className="space-y-2">
                  <p className="text-xs font-medium text-neutral-900">Resource Limits</p>
                  <div className="grid gap-2 text-xs">
                    <div className="flex justify-between py-1">
                      <span className="text-neutral-600">CPU</span>
                      <span className="text-neutral-900">{session.resource_limits?.cpu || 'N/A'}</span>
                    </div>
                    <div className="flex justify-between py-1">
                      <span className="text-neutral-600">Memory</span>
                      <span className="text-neutral-900">{session.resource_limits?.memory || 'N/A'}</span>
                    </div>
                    <div className="flex justify-between py-1">
                      <span className="text-neutral-600">Timeout</span>
                      <span className="text-neutral-900">{session.resource_limits?.timeout_minutes ? `${session.resource_limits.timeout_minutes}m` : 'N/A'}</span>
                    </div>
                  </div>
                </div>
                <Separator className="bg-neutral-200" />
                <div className="space-y-2">
                  <p className="text-xs font-medium text-neutral-900">Container Details</p>
                  <div className="grid gap-2 text-xs">
                    {session.container_id && (
                      <div className="flex justify-between py-1">
                        <span className="text-neutral-600">Container ID</span>
                        <span className="font-mono text-neutral-900">{session.container_id.substring(0, 12)}...</span>
                      </div>
                    )}
                    {session.container_network && (
                      <div className="flex justify-between py-1">
                        <span className="text-neutral-600">Network</span>
                        <span className="text-neutral-900">{session.container_network}</span>
                      </div>
                    )}
                  </div>
                </div>
              </div>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}

// Session Events Timeline Component
function SessionEventsTimeline({ 
  sessionId, 
  events, 
  isLoading 
}: { 
  sessionId: string;
  events: SessionEvent[];
  isLoading: boolean;
}) {
  if (isLoading) {
    return (
      <Card className="border-neutral-200">
        <CardContent className="p-6">
          <div className="flex items-center justify-center py-8">
            <RefreshCw className="h-4 w-4 animate-spin mr-2 text-neutral-400" />
            <span className="text-sm text-neutral-600">Loading events...</span>
          </div>
        </CardContent>
      </Card>
    );
  }

  if (events.length === 0) {
    return (
      <Card className="border-neutral-200">
        <CardHeader className="pb-2">
          <CardTitle className="text-sm font-medium text-neutral-900">Session Events</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="text-center py-8 text-neutral-500">
            <Activity className="h-6 w-6 mx-auto mb-2 text-neutral-400" />
            <p className="text-sm font-medium">No events recorded yet</p>
            <p className="text-xs">Events will appear here as the session progresses</p>
          </div>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card className="border-neutral-200">
      <CardHeader className="pb-2">
        <CardTitle className="text-sm font-medium text-neutral-900">Session Events Timeline</CardTitle>
      </CardHeader>
      <CardContent>
        <ScrollArea className="h-80">
          <div className="space-y-3">
            {events.map((event, index) => (
              <div key={event.id} className="flex gap-3">
                <div className="flex flex-col items-center">
                  <div className="w-2 h-2 rounded-full bg-neutral-900 flex-shrink-0" />
                  {index < events.length - 1 && (
                    <div className="w-px h-4 bg-neutral-200 mt-1" />
                  )}
                </div>
                <div className="flex-1 space-y-1 pb-3">
                  <div className="flex items-center justify-between">
                    <p className="text-xs font-medium text-neutral-900">{formatEventType(event.event || '')}</p>
                    <Badge variant="outline" className="text-xs border-neutral-200 text-neutral-600 px-1.5 py-0">
                      {event.timestamp ? new Date(event.timestamp).toLocaleTimeString() : 'N/A'}
                    </Badge>
                  </div>
                  {event.data && Object.keys(event.data).length > 0 && (
                    <div className="text-xs text-neutral-600 bg-neutral-50 p-2 rounded border border-neutral-200">
                      <pre className="whitespace-pre-wrap">
                        {JSON.stringify(event.data, null, 2)}
                      </pre>
                    </div>
                  )}
                  <p className="text-xs text-neutral-500">
                    {event.timestamp ? new Date(event.timestamp).toLocaleDateString() : 'N/A'}
                  </p>
                </div>
              </div>
            ))}
          </div>
        </ScrollArea>
      </CardContent>
    </Card>
  );
}

// Session Metrics View Component
function SessionMetricsView({ sessionId }: { sessionId: string }) {
  return (
    <div className="space-y-4">
      <Card className="border-neutral-200">
        <CardHeader className="pb-2">
          <CardTitle className="text-sm font-medium text-neutral-900">Performance Overview</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="text-center py-8 text-neutral-500">
            <Activity className="h-6 w-6 mx-auto mb-2 text-neutral-400" />
            <p className="text-sm font-medium">Performance metrics will be displayed here</p>
            <p className="text-xs">CPU, memory, and network usage charts</p>
          </div>
        </CardContent>
      </Card>
      
      <div className="grid gap-4 md:grid-cols-3">
        <Card className="border-neutral-200">
          <CardHeader className="pb-1">
            <CardTitle className="text-xs font-medium text-neutral-900">CPU Usage</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-xl font-semibold text-neutral-900">--</div>
            <p className="text-xs text-neutral-500">Average over last hour</p>
          </CardContent>
        </Card>
        
        <Card className="border-neutral-200">
          <CardHeader className="pb-1">
            <CardTitle className="text-xs font-medium text-neutral-900">Memory Usage</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-xl font-semibold text-neutral-900">--</div>
            <p className="text-xs text-neutral-500">Current allocation</p>
          </CardContent>
        </Card>
        
        <Card className="border-neutral-200">
          <CardHeader className="pb-1">
            <CardTitle className="text-xs font-medium text-neutral-900">Network I/O</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-xl font-semibold text-neutral-900">--</div>
            <p className="text-xs text-neutral-500">Total bytes transferred</p>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}

// Helper function to format event types
function formatEventType(eventType: string): string {
  return eventType
    .split('_')
    .map(word => word.charAt(0).toUpperCase() + word.slice(1))
    .join(' ');
} 