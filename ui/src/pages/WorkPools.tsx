import React, { useState } from 'react';
import { Plus, RefreshCw, Settings, TrendingUp, Trash } from 'lucide-react';
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import { useGetApiV1Workpools, usePostApiV1Workpools, useDeleteApiV1WorkpoolsId, usePostApiV1WorkpoolsIdScale } from '@/lib/api/workpools/workpools';
import type { WorkPool, ProviderType } from '@/lib/api/model';
import { toast } from 'sonner';

export default function WorkPools() {
  const { data: workpools, isLoading, refetch } = useGetApiV1Workpools();
  const createWorkPool = usePostApiV1Workpools();
  const deleteWorkPool = useDeleteApiV1WorkpoolsId();
  const scaleWorkPool = usePostApiV1WorkpoolsIdScale();
  const [showCreateDialog, setShowCreateDialog] = useState(false);
  const [newWorkPool, setNewWorkPool] = useState<Partial<WorkPool>>({
    name: '',
    description: '',
    provider: 'docker' as ProviderType,
    min_size: 0,
    max_concurrency: 10,
    auto_scale: true,
    paused: false,
  });

  const handleCreateWorkPool = async () => {
    try {
      await createWorkPool.mutateAsync({ data: newWorkPool as WorkPool });
      toast.success('Work pool created successfully');
      setShowCreateDialog(false);
      refetch();
    } catch (error) {
      toast.error('Failed to create work pool');
    }
  };

  const handleDeleteWorkPool = async (id: string) => {
    try {
      await deleteWorkPool.mutateAsync({ id });
      toast.success('Work pool deleted successfully');
      refetch();
    } catch (error) {
      toast.error('Failed to delete work pool');
    }
  };

  if (isLoading) {
    return <div className="flex items-center justify-center h-64">Loading work pools...</div>;
  }

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-3xl font-bold">Work Pools</h1>
          <p className="text-muted-foreground">
            Manage your browser work pool configurations
          </p>
        </div>
        <div className="flex gap-2">
          <Button onClick={() => refetch()} variant="outline" size="sm">
            <RefreshCw className="h-4 w-4 mr-2" />
            Refresh
          </Button>
          <Dialog open={showCreateDialog} onOpenChange={setShowCreateDialog}>
            <DialogTrigger asChild>
              <Button>
                <Plus className="h-4 w-4 mr-2" />
                New Work Pool
              </Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Create New Work Pool</DialogTitle>
                <DialogDescription>
                  Configure your work pool settings
                </DialogDescription>
              </DialogHeader>
              <div className="grid gap-4 py-4">
                <div className="grid grid-cols-4 items-center gap-4">
                  <Label className="text-right">Name</Label>
                  <Input 
                    className="col-span-3"
                    value={newWorkPool.name} 
                    onChange={(e) => setNewWorkPool({...newWorkPool, name: e.target.value})}
                  />
                </div>
                <div className="grid grid-cols-4 items-center gap-4">
                  <Label className="text-right">Description</Label>
                  <Textarea 
                    className="col-span-3"
                    value={newWorkPool.description} 
                    onChange={(e) => setNewWorkPool({...newWorkPool, description: e.target.value})}
                  />
                </div>
                <div className="grid grid-cols-4 items-center gap-4">
                  <Label className="text-right">Provider</Label>
                  <Select value={newWorkPool.provider} onValueChange={(value) => setNewWorkPool({...newWorkPool, provider: value as ProviderType})}>
                    <SelectTrigger className="col-span-3">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="docker">Docker</SelectItem>
                      <SelectItem value="azure_aci">Azure ACI</SelectItem>
                      <SelectItem value="local">Local</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div className="grid grid-cols-4 items-center gap-4">
                  <Label className="text-right">Max Concurrency</Label>
                  <Input 
                    type="number"
                    className="col-span-3"
                    value={newWorkPool.max_concurrency} 
                    onChange={(e) => setNewWorkPool({...newWorkPool, max_concurrency: parseInt(e.target.value)})}
                  />
                </div>
                <div className="grid grid-cols-4 items-center gap-4">
                  <Label className="text-right">Auto Scale</Label>
                  <Switch 
                    checked={newWorkPool.auto_scale} 
                    onCheckedChange={(checked) => setNewWorkPool({...newWorkPool, auto_scale: checked})}
                  />
                </div>
              </div>
              <DialogFooter>
                <Button type="submit" onClick={handleCreateWorkPool}>Create Work Pool</Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </div>
      </div>

      <div className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
        {workpools?.pools?.map((pool) => (
          <Card key={pool.id}>
            <CardHeader>
              <div className="flex items-center justify-between">
                <CardTitle className="text-lg">{pool.name}</CardTitle>
                <div className="flex items-center gap-2">
                  {pool.paused ? (
                    <Badge variant="secondary">Paused</Badge>
                  ) : (
                    <Badge variant="default">Active</Badge>
                  )}
                  <Button 
                    size="sm" 
                    variant="outline"
                    onClick={() => handleDeleteWorkPool(pool.id!)}
                  >
                    <Trash className="h-3 w-3" />
                  </Button>
                </div>
              </div>
              <CardDescription>{pool.description}</CardDescription>
            </CardHeader>
            <CardContent>
              <div className="space-y-2">
                <div className="flex justify-between text-sm">
                  <span>Provider:</span>
                  <Badge variant="outline">{pool.provider}</Badge>
                </div>
                <div className="flex justify-between text-sm">
                  <span>Min Size:</span>
                  <span>{pool.min_size}</span>
                </div>
                <div className="flex justify-between text-sm">
                  <span>Max Concurrency:</span>
                  <span>{pool.max_concurrency}</span>
                </div>
                <div className="flex justify-between text-sm">
                  <span>Auto Scale:</span>
                  <span>{pool.auto_scale ? 'Yes' : 'No'}</span>
                </div>
              </div>
            </CardContent>
            <CardFooter>
              <div className="flex gap-2 w-full">
                <Button size="sm" variant="outline" className="flex-1">
                  <Settings className="h-3 w-3 mr-1" />
                  Configure
                </Button>
                <Button size="sm" variant="outline" className="flex-1">
                  <TrendingUp className="h-3 w-3 mr-1" />
                  Scale
                </Button>
              </div>
            </CardFooter>
          </Card>
        ))}
      </div>
    </div>
  );
} 