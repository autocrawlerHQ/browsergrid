import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { RefreshCw, Clock, Calendar, Play, AlertTriangle } from 'lucide-react';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';

// Assume this is the generated API hook
import { useGetApiV1MonitoringScheduler } from '@/lib/api/monitoring/monitoring';

interface SchedulerEntry {
  id: string;
  spec: string;
  task?: {
    type?: string;
    payload?: any;
  };
  next?: string;
  prev?: string;
  opts?: any[];
}

export default function Schedulers() {
  const { data: entries, isLoading, error, refetch } = useGetApiV1MonitoringScheduler();
  
  const formatSchedule = (spec: string): string => {
    // Convert cron expression to human readable format
    if (spec === '* * * * *') return 'Every minute';
    if (spec === '*/5 * * * *') return 'Every 5 minutes';
    if (spec === '0 * * * *') return 'Every hour';
    if (spec === '0 0 * * *') return 'Daily at midnight';
    if (spec === '0 9 * * 1-5') return 'Weekdays at 9:00 AM';
    return spec;
  };
  
  const formatTime = (timeStr?: string): string => {
    if (!timeStr) return 'Never';
    const date = new Date(timeStr);
    const now = new Date();
    const diffMs = Math.abs(now.getTime() - date.getTime());
    const diffHours = diffMs / (1000 * 60 * 60);
    
    if (diffHours < 1) {
      const diffMinutes = Math.floor(diffMs / (1000 * 60));
      return `${diffMinutes}m ago`;
    } else if (diffHours < 24) {
      return date.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit' });
    }
    return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' });
  };
  
  const getNextRunStatus = (nextTime?: string) => {
    if (!nextTime) return { text: 'Not scheduled', variant: 'secondary' as const };
    
    const next = new Date(nextTime);
    const now = new Date();
    const diffMinutes = (next.getTime() - now.getTime()) / (1000 * 60);
    
    if (diffMinutes < 0) {
      return { text: 'Overdue', variant: 'destructive' as const };
    } else if (diffMinutes < 5) {
      return { text: 'Soon', variant: 'default' as const };
    }
    return { text: 'Scheduled', variant: 'secondary' as const };
  };

  const LoadingSkeleton = () => (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <div className="h-6 w-32 bg-muted animate-pulse rounded" />
        </CardHeader>
        <CardContent>
          <div className="space-y-3">
            {[1, 2, 3].map((i) => (
              <div key={i} className="h-12 w-full bg-muted animate-pulse rounded" />
            ))}
          </div>
        </CardContent>
      </Card>
    </div>
  );

  if (isLoading) {
    return <LoadingSkeleton />;
  }

  if (error) {
    return (
      <div className="space-y-6">
        <div className="flex justify-between items-center">
          <div>
            <h1 className="text-3xl font-bold">Schedulers</h1>
            <p className="text-muted-foreground">
              Manage scheduled and periodic tasks
            </p>
          </div>
          <Button onClick={() => refetch()} variant="outline" size="sm">
            <RefreshCw className="h-4 w-4 mr-2" />
            Retry
          </Button>
        </div>
        <Alert variant="destructive">
          <AlertDescription>
            Failed to load scheduler data. Please check your connection and try again.
          </AlertDescription>
        </Alert>
      </div>
    );
  }

  const schedulerEntries = entries?.entries || [];
  const totalEntries = entries?.total || 0;

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-3xl font-bold">Schedulers</h1>
          <p className="text-muted-foreground">
            Manage scheduled and periodic tasks
          </p>
        </div>
        <Button onClick={() => refetch()} variant="outline" size="sm" disabled={isLoading}>
          <RefreshCw className={`h-4 w-4 mr-2 ${isLoading ? 'animate-spin' : ''}`} />
          Refresh
        </Button>
      </div>

      {/* Summary Cards */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Schedules</CardTitle>
            <Calendar className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{totalEntries}</div>
            <p className="text-xs text-muted-foreground">
              Active scheduler entries
            </p>
          </CardContent>
        </Card>
        
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Next Run</CardTitle>
            <Clock className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {schedulerEntries.length > 0 ? formatTime(schedulerEntries[0]?.next) : 'N/A'}
            </div>
            <p className="text-xs text-muted-foreground">
              Upcoming task execution
            </p>
          </CardContent>
        </Card>
        
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Running Now</CardTitle>
            <Play className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {schedulerEntries.filter((e: SchedulerEntry) => {
                const status = getNextRunStatus(e.next);
                return status.text === 'Overdue';
              }).length}
            </div>
            <p className="text-xs text-muted-foreground">
              Tasks currently executing
            </p>
          </CardContent>
        </Card>
        
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Overdue</CardTitle>
            <AlertTriangle className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {schedulerEntries.filter((e: SchedulerEntry) => {
                const status = getNextRunStatus(e.next);
                return status.variant === 'destructive';
              }).length}
            </div>
            <p className="text-xs text-muted-foreground">
              Tasks past schedule
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Scheduler Table */}
      <Card>
        <CardHeader>
          <CardTitle>Scheduled Tasks</CardTitle>
        </CardHeader>
        <CardContent>
          {schedulerEntries.length === 0 ? (
            <div className="text-center py-8 text-muted-foreground">
              No scheduled tasks found
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full">
                <thead>
                  <tr className="border-b">
                    <th className="text-left p-2 font-medium">Task ID</th>
                    <th className="text-left p-2 font-medium">Task Type</th>
                    <th className="text-left p-2 font-medium">Schedule</th>
                    <th className="text-left p-2 font-medium">Last Run</th>
                    <th className="text-left p-2 font-medium">Next Run</th>
                    <th className="text-left p-2 font-medium">Status</th>
                  </tr>
                </thead>
                <tbody>
                  {schedulerEntries.map((entry: SchedulerEntry) => {
                    const nextStatus = getNextRunStatus(entry.next);
                    
                    return (
                      <tr key={entry.id} className="border-b hover:bg-muted/50">
                        <td className="p-2">
                          <div className="font-mono text-sm">
                            {entry.id.length > 8 ? entry.id.slice(0, 8) + '...' : entry.id}
                          </div>
                        </td>
                        <td className="p-2">
                          <Badge variant="outline">
                            {entry.task?.type || 'Unknown'}
                          </Badge>
                        </td>
                        <td className="p-2">
                          <div>
                            <div className="font-medium">{formatSchedule(entry.spec)}</div>
                            <div className="text-xs text-muted-foreground font-mono">{entry.spec}</div>
                          </div>
                        </td>
                        <td className="p-2">
                          <div className="text-sm">{formatTime(entry.prev)}</div>
                        </td>
                        <td className="p-2">
                          <div className="text-sm">{formatTime(entry.next)}</div>
                        </td>
                        <td className="p-2">
                          <Badge variant={nextStatus.variant}>
                            {nextStatus.text}
                          </Badge>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}