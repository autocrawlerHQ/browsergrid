import React from 'react';
import { RefreshCw, Activity, Clock } from 'lucide-react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { useGetApiV1MonitoringServers } from '@/lib/api/monitoring/monitoring';
import type { AsynqServerInfo } from '@/lib/api/model';
import { toast } from 'sonner';

export default function Workers() {
  const { data: servers, isLoading, refetch } = useGetApiV1MonitoringServers();

  const formatUptime = (startTime: string) => {
    const start = new Date(startTime);
    const now = new Date();
    const diffMs = now.getTime() - start.getTime();
    const diffHours = Math.floor(diffMs / (1000 * 60 * 60));
    const diffMinutes = Math.floor((diffMs % (1000 * 60 * 60)) / (1000 * 60));
    
    if (diffHours > 0) {
      return `${diffHours}h ${diffMinutes}m`;
    }
    return `${diffMinutes}m`;
  };

  const getServerStatus = (server: AsynqServerInfo) => {
    const activeWorkers = server.activeWorkers?.length || 0;
    const concurrency = server.concurrency || 0;
    
    if (activeWorkers === 0) {
      return { status: 'idle', color: 'secondary' };
    } else if (activeWorkers >= concurrency) {
      return { status: 'busy', color: 'destructive' };
    } else {
      return { status: 'active', color: 'default' };
    }
  };

  const getTotalActiveWorkers = () => {
    if (!Array.isArray(servers)) return 0;
    return servers.reduce((total, server) => total + (server.activeWorkers?.length || 0), 0);
  };

  const getTotalCapacity = () => {
    if (!Array.isArray(servers)) return 0;
    return servers.reduce((total, server) => total + (server.concurrency || 0), 0);
  };

  if (isLoading) {
    return <div className="flex items-center justify-center h-64">Loading workers...</div>;
  }

  // Ensure servers is an array
  const serversArray = Array.isArray(servers) ? servers : [];

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-3xl font-bold">Workers</h1>
          <p className="text-muted-foreground">
            Monitor your Asynq worker servers and active tasks
          </p>
        </div>
        <Button onClick={() => refetch()} variant="outline" size="sm">
          <RefreshCw className="h-4 w-4 mr-2" />
          Refresh
        </Button>
      </div>

      {/* Summary Cards */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Servers</CardTitle>
            <Activity className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{serversArray.length}</div>
            <p className="text-xs text-muted-foreground">
              Active worker servers
            </p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Active Workers</CardTitle>
            <Clock className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{getTotalActiveWorkers()}</div>
            <p className="text-xs text-muted-foreground">
              Currently processing tasks
            </p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Capacity</CardTitle>
            <Activity className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{getTotalCapacity()}</div>
            <p className="text-xs text-muted-foreground">
              Maximum concurrent tasks
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Servers Table */}
      <Card>
        <CardHeader>
          <CardTitle>Worker Servers</CardTitle>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Server ID</TableHead>
                <TableHead>Host</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Active/Capacity</TableHead>
                <TableHead>Queues</TableHead>
                <TableHead>Started</TableHead>
                <TableHead>PID</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {serversArray.map((server) => {
                const serverStatus = getServerStatus(server);
                const activeWorkers = server.activeWorkers?.length || 0;
                const capacity = server.concurrency || 0;
                const queues = server.queues ? Object.keys(server.queues) : [];
                
                return (
                  <TableRow key={server.id}>
                    <TableCell>
                      <div className="font-mono text-sm">{server.id?.slice(0, 8)}</div>
                    </TableCell>
                    <TableCell>
                      <div className="font-medium">{server.host}</div>
                    </TableCell>
                    <TableCell>
                      <Badge variant={serverStatus.color as any}>
                        {serverStatus.status}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center gap-2">
                        <span>{activeWorkers}/{capacity}</span>
                        <div className="w-16 bg-gray-200 rounded-full h-2 dark:bg-gray-700">
                          <div 
                            className="bg-blue-600 h-2 rounded-full" 
                            style={{ width: `${capacity > 0 ? (activeWorkers / capacity) * 100 : 0}%` }}
                          ></div>
                        </div>
                      </div>
                    </TableCell>
                    <TableCell>
                      <div className="flex flex-wrap gap-1">
                        {queues.map((queue) => (
                          <Badge key={queue} variant="outline" className="text-xs">
                            {queue}
                          </Badge>
                        ))}
                      </div>
                    </TableCell>
                    <TableCell>
                      {server.started ? formatUptime(server.started) : 'N/A'}
                    </TableCell>
                    <TableCell>
                      <div className="font-mono text-sm">{server.pid}</div>
                    </TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      {/* Active Workers Details */}
      {serversArray.some(server => server.activeWorkers && server.activeWorkers.length > 0) && (
        <Card>
          <CardHeader>
            <CardTitle>Active Workers</CardTitle>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Server</TableHead>
                  <TableHead>Task Type</TableHead>
                  <TableHead>Queue</TableHead>
                  <TableHead>Task ID</TableHead>
                  <TableHead>Started</TableHead>
                  <TableHead>Deadline</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {serversArray.map((server) => 
                  server.activeWorkers?.map((worker, index) => (
                    <TableRow key={`${server.id}-${index}`}>
                      <TableCell>
                        <div className="font-mono text-sm">{server.id?.slice(0, 8)}</div>
                      </TableCell>
                      <TableCell>
                        <Badge variant="outline">{worker.taskType}</Badge>
                      </TableCell>
                      <TableCell>
                        <Badge variant="secondary">{worker.queue}</Badge>
                      </TableCell>
                      <TableCell>
                        <div className="font-mono text-sm">{worker.taskID?.slice(0, 8)}</div>
                      </TableCell>
                      <TableCell>
                        {worker.started ? new Date(worker.started).toLocaleTimeString() : 'N/A'}
                      </TableCell>
                      <TableCell>
                        {worker.deadline ? new Date(worker.deadline).toLocaleTimeString() : 'N/A'}
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      )}
    </div>
  );
} 