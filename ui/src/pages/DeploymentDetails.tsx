import React, { useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { ArrowLeft, Play, RefreshCw, Calendar, Clock, Package, Globe, Activity, FileText, AlertCircle } from 'lucide-react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { StatusBadge } from '@/components/dashboard/StatusBadge';
import { 
  useGetApiV1DeploymentsId, 
  useGetApiV1DeploymentsIdRuns, 
  useGetApiV1DeploymentsIdStats,
  usePostApiV1DeploymentsIdRuns 
} from '@/lib/api/deployments/deployments';
import type { 
  InternalDeploymentsCreateDeploymentRunRequest,
  InternalDeploymentsDeployment 
} from '@/lib/api/model';
import { toast } from 'sonner';

export default function DeploymentDetails() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [showRunDialog, setShowRunDialog] = useState(false);
  
  const { data: deployment, isLoading: deploymentLoading } = useGetApiV1DeploymentsId(id!);
  const { data: runsData, isLoading: runsLoading } = useGetApiV1DeploymentsIdRuns(id!);
  const { data: statsData } = useGetApiV1DeploymentsIdStats(id!);
  const createRun = usePostApiV1DeploymentsIdRuns();

  const [newRun, setNewRun] = useState<InternalDeploymentsCreateDeploymentRunRequest>({
    environment: {},
    config: {}
  });

  const handleCreateRun = async () => {
    try {
      await createRun.mutateAsync({ id: id!, data: newRun });
      toast.success('Deployment run started successfully');
      setShowRunDialog(false);
      setNewRun({ environment: {}, config: {} });
    } catch (error) {
      toast.error('Failed to start deployment run');
    }
  };

  if (deploymentLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="flex items-center space-x-2">
          <RefreshCw className="h-4 w-4 animate-spin text-neutral-400" />
          <span className="text-sm text-neutral-600">Loading deployment...</span>
        </div>
      </div>
    );
  }

  if (!deployment) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-center">
          <AlertCircle className="h-8 w-8 mx-auto text-neutral-400 mb-3" />
          <h3 className="text-sm font-semibold text-neutral-900 mb-1">Deployment not found</h3>
          <p className="text-xs text-neutral-600 mb-4">
            The deployment you're looking for doesn't exist or has been deleted.
          </p>
          <Button onClick={() => navigate('/deployments')} size="sm">
            Back to Deployments
          </Button>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
        <div className="flex items-center gap-3">
          <Button 
            variant="ghost" 
            size="sm" 
            onClick={() => navigate('/deployments')}
            className="text-neutral-600 hover:text-neutral-900"
          >
            <ArrowLeft className="h-4 w-4 mr-1" />
            Back
          </Button>
          <div>
            <h1 className="text-2xl font-semibold text-neutral-900 tracking-tight">
              {deployment.name}
            </h1>
            <p className="text-sm text-neutral-600 mt-1">
              {deployment.description || 'No description'}
            </p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <Dialog open={showRunDialog} onOpenChange={setShowRunDialog}>
            <DialogTrigger asChild>
              <Button 
                size="sm" 
                className="bg-neutral-900 hover:bg-neutral-800 text-white"
                disabled={deployment.status !== 'active'}
              >
                <Play className="h-3 w-3 mr-1.5" />
                Run Deployment
              </Button>
            </DialogTrigger>
            <RunDeploymentDialog
              deployment={deployment}
              newRun={newRun}
              setNewRun={setNewRun}
              onSubmit={handleCreateRun}
              isLoading={createRun.isPending}
            />
          </Dialog>
        </div>
      </div>

      {/* Stats Cards */}
      <div className="grid grid-cols-1 gap-4 md:grid-cols-4">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Status</CardTitle>
            <Package className="h-4 w-4 text-neutral-400" />
          </CardHeader>
          <CardContent>
            <StatusBadge status={deployment.status || 'unknown'} />
          </CardContent>
        </Card>
        
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Runs</CardTitle>
            <Activity className="h-4 w-4 text-neutral-400" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{deployment.total_runs || 0}</div>
          </CardContent>
        </Card>
        
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Success Rate</CardTitle>
            <Globe className="h-4 w-4 text-neutral-400" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {deployment.total_runs && deployment.total_runs > 0 
                ? Math.round(((deployment.successful_runs || 0) / deployment.total_runs) * 100)
                : 0}%
            </div>
          </CardContent>
        </Card>
        
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Last Run</CardTitle>
            <Clock className="h-4 w-4 text-neutral-400" />
          </CardHeader>
          <CardContent>
            <div className="text-sm">
              {deployment.last_run_at 
                ? new Date(deployment.last_run_at).toLocaleDateString()
                : 'Never'
              }
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Main Content Tabs */}
      <Tabs defaultValue="overview" className="space-y-4">
        <TabsList>
          <TabsTrigger value="overview">Overview</TabsTrigger>
          <TabsTrigger value="runs">Runs</TabsTrigger>
          <TabsTrigger value="config">Configuration</TabsTrigger>
        </TabsList>

        <TabsContent value="overview" className="space-y-4">
          <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
            {/* Deployment Info */}
            <Card>
              <CardHeader>
                <CardTitle className="text-lg">Deployment Information</CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                <div className="flex justify-between">
                  <span className="text-sm text-neutral-600">ID:</span>
                  <span className="text-sm font-mono">{deployment.id}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-sm text-neutral-600">Version:</span>
                  <span className="text-sm">{deployment.version}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-sm text-neutral-600">Runtime:</span>
                  <Badge variant="outline" className="text-xs">
                    {deployment.runtime}
                  </Badge>
                </div>
                <div className="flex justify-between">
                  <span className="text-sm text-neutral-600">Created:</span>
                  <span className="text-sm">
                    {deployment.created_at ? new Date(deployment.created_at).toLocaleString() : 'N/A'}
                  </span>
                </div>
                <div className="flex justify-between">
                  <span className="text-sm text-neutral-600">Updated:</span>
                  <span className="text-sm">
                    {deployment.updated_at ? new Date(deployment.updated_at).toLocaleString() : 'N/A'}
                  </span>
                </div>
              </CardContent>
            </Card>

            {/* Package Info */}
            <Card>
              <CardHeader>
                <CardTitle className="text-lg">Package Information</CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                <div>
                  <span className="text-sm text-neutral-600">Package URL:</span>
                  <div className="text-sm font-mono break-all mt-1">
                    {deployment.package_url}
                  </div>
                </div>
                <div>
                  <span className="text-sm text-neutral-600">Package Hash:</span>
                  <div className="text-sm font-mono break-all mt-1">
                    {deployment.package_hash}
                  </div>
                </div>
              </CardContent>
            </Card>
          </div>
        </TabsContent>

        <TabsContent value="runs" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle className="text-lg">Deployment Runs</CardTitle>
            </CardHeader>
            <CardContent>
              {runsLoading ? (
                <div className="flex items-center justify-center h-32">
                  <RefreshCw className="h-4 w-4 animate-spin text-neutral-400 mr-2" />
                  <span className="text-sm text-neutral-600">Loading runs...</span>
                </div>
              ) : runsData?.runs && runsData.runs.length > 0 ? (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Run ID</TableHead>
                      <TableHead>Status</TableHead>
                      <TableHead>Started</TableHead>
                      <TableHead>Duration</TableHead>
                      <TableHead>Session</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {runsData.runs.map((run) => (
                      <TableRow key={run.id}>
                        <TableCell className="font-mono text-sm">
                          {run.id?.substring(0, 8)}...
                        </TableCell>
                        <TableCell>
                          <StatusBadge status={run.status || 'unknown'} />
                        </TableCell>
                        <TableCell>
                          {run.started_at ? new Date(run.started_at).toLocaleString() : 'N/A'}
                        </TableCell>
                        <TableCell>
                          {run.duration_seconds ? `${run.duration_seconds}s` : '-'}
                        </TableCell>
                        <TableCell>
                          {run.session_id ? (
                            <Badge variant="outline" className="text-xs">
                              {run.session_id.substring(0, 8)}...
                            </Badge>
                          ) : (
                            <span className="text-neutral-400">-</span>
                          )}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              ) : (
                <div className="text-center py-8">
                  <Activity className="h-8 w-8 mx-auto text-neutral-400 mb-3" />
                  <h3 className="text-sm font-semibold text-neutral-900 mb-1">No runs yet</h3>
                  <p className="text-xs text-neutral-600 mb-4">
                    This deployment hasn't been run yet. Start your first run to see results here.
                  </p>
                  <Button 
                    size="sm" 
                    onClick={() => setShowRunDialog(true)}
                    disabled={deployment.status !== 'active'}
                  >
                    <Play className="h-3 w-3 mr-1.5" />
                    Run Deployment
                  </Button>
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="config" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle className="text-lg">Configuration</CardTitle>
            </CardHeader>
            <CardContent>
                             <pre className="bg-neutral-50 p-4 rounded-md text-sm overflow-auto">
                 {deployment.config ? JSON.stringify(deployment.config, null, 2) : 'No configuration'}
               </pre>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}

// Run Deployment Dialog Component
function RunDeploymentDialog({
  deployment,
  newRun,
  setNewRun,
  onSubmit,
  isLoading
}: {
  deployment: InternalDeploymentsDeployment;
  newRun: InternalDeploymentsCreateDeploymentRunRequest;
  setNewRun: (run: InternalDeploymentsCreateDeploymentRunRequest) => void;
  onSubmit: () => void;
  isLoading: boolean;
}) {
  return (
    <DialogContent className="max-w-2xl">
      <DialogHeader>
        <DialogTitle>Run Deployment</DialogTitle>
        <DialogDescription>
          Start a new run of {deployment.name} v{deployment.version}
        </DialogDescription>
      </DialogHeader>
      
      <div className="space-y-4">
        <div>
          <label className="text-sm font-medium text-neutral-700">Environment Variables (JSON)</label>
          <textarea
            value={JSON.stringify(newRun.environment, null, 2)}
            onChange={(e) => {
              try {
                const env = JSON.parse(e.target.value);
                setNewRun({ ...newRun, environment: env });
              } catch {
                // Invalid JSON, ignore
              }
            }}
            placeholder='{"KEY": "value"}'
            className="mt-1 w-full h-24 p-2 border border-neutral-200 rounded-md text-sm font-mono"
          />
        </div>
        
        <div>
          <label className="text-sm font-medium text-neutral-700">Configuration Override (JSON)</label>
          <textarea
            value={JSON.stringify(newRun.config, null, 2)}
            onChange={(e) => {
              try {
                const config = JSON.parse(e.target.value);
                setNewRun({ ...newRun, config });
              } catch {
                // Invalid JSON, ignore
              }
            }}
            placeholder='{"timeout_seconds": 600}'
            className="mt-1 w-full h-24 p-2 border border-neutral-200 rounded-md text-sm font-mono"
          />
        </div>
        
        <div className="flex justify-end gap-2 pt-4">
          <Button onClick={onSubmit} disabled={isLoading}>
            {isLoading ? 'Starting...' : 'Start Run'}
          </Button>
        </div>
      </div>
    </DialogContent>
  );
} 