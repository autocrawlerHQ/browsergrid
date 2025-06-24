import React from 'react';
import { RefreshCw, Play, Pause, Trash } from 'lucide-react';
import { Card } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { useGetApiV1Workers, useDeleteApiV1WorkersId, usePostApiV1WorkersIdPause } from '@/lib/api/workers/workers';
import { toast } from 'sonner';

export default function Workers() {
  const { data: workers, isLoading, refetch } = useGetApiV1Workers();
  const deleteWorker = useDeleteApiV1WorkersId();
  const pauseWorker = usePostApiV1WorkersIdPause();

  const handleDeleteWorker = async (id: string) => {
    try {
      await deleteWorker.mutateAsync({ id });
      toast.success('Worker deleted successfully');
      refetch();
    } catch (error) {
      toast.error('Failed to delete worker');
    }
  };

  const handleToggleWorker = async (id: string, paused: boolean) => {
    try {
      await pauseWorker.mutateAsync({ id, data: { paused: !paused } });
      toast.success(`Worker ${!paused ? 'paused' : 'resumed'} successfully`);
      refetch();
    } catch (error) {
      toast.error('Failed to toggle worker');
    }
  };

  if (isLoading) {
    return <div className="flex items-center justify-center h-64">Loading workers...</div>;
  }

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-3xl font-bold">Workers</h1>
          <p className="text-muted-foreground">
            Monitor and manage your browser workers
          </p>
        </div>
        <Button onClick={() => refetch()} variant="outline" size="sm">
          <RefreshCw className="h-4 w-4 mr-2" />
          Refresh
        </Button>
      </div>

      <Card>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Status</TableHead>
              <TableHead>Provider</TableHead>
              <TableHead>Active/Max</TableHead>
              <TableHead>Last Heartbeat</TableHead>
              <TableHead>Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {workers?.workers?.map((worker) => (
              <TableRow key={worker.id}>
                <TableCell>
                  <div>
                    <div className="font-medium">{worker.name}</div>
                    <div className="text-sm text-muted-foreground">{worker.hostname}</div>
                  </div>
                </TableCell>
                <TableCell>
                  <div className="flex items-center gap-2">
                    {worker.paused ? (
                      <Badge variant="secondary">Paused</Badge>
                    ) : (
                      <Badge variant="default">Online</Badge>
                    )}
                  </div>
                </TableCell>
                <TableCell>
                  <Badge variant="outline">{worker.provider}</Badge>
                </TableCell>
                <TableCell>
                  <div className="flex items-center gap-2">
                    <span>{worker.active}/{worker.max_slots}</span>
                    <div className="w-16 bg-gray-200 rounded-full h-2 dark:bg-gray-700">
                      <div 
                        className="bg-blue-600 h-2 rounded-full" 
                        style={{ width: `${((worker.active || 0) / (worker.max_slots || 1)) * 100}%` }}
                      ></div>
                    </div>
                  </div>
                </TableCell>
                <TableCell>
                  {worker.last_beat ? new Date(worker.last_beat).toLocaleString() : 'N/A'}
                </TableCell>
                <TableCell>
                  <div className="flex items-center gap-2">
                    <Button 
                      size="sm" 
                      variant="outline"
                      onClick={() => handleToggleWorker(worker.id!, worker.paused || false)}
                    >
                      {worker.paused ? <Play className="h-3 w-3" /> : <Pause className="h-3 w-3" />}
                    </Button>
                    <Button 
                      size="sm" 
                      variant="outline"
                      onClick={() => handleDeleteWorker(worker.id!)}
                    >
                      <Trash className="h-3 w-3" />
                    </Button>
                  </div>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </Card>
    </div>
  );
} 