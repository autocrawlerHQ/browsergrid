import React, { useState, useMemo } from 'react';
import { Plus, RefreshCw, Package, ExternalLink, MoreVertical, Search, Filter, Eye, Play, Trash2, Edit } from 'lucide-react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { StatusBadge } from '@/components/dashboard/StatusBadge';
import { useGetApiV1Deployments, usePostApiV1Deployments, useDeleteApiV1DeploymentsId, usePatchApiV1DeploymentsId } from '@/lib/api/deployments/deployments';
import type { 
  InternalDeploymentsDeployment, 
  InternalDeploymentsCreateDeploymentRequest,
  InternalDeploymentsUpdateDeploymentRequest,
  InternalDeploymentsDeploymentConfig,
  Runtime,
  DeploymentStatus 
} from '@/lib/api/model';
import { toast } from 'sonner';
import { useNavigate } from 'react-router-dom';

export default function Deployments() {
  const { data: deploymentsData, isLoading, refetch } = useGetApiV1Deployments();
  const createDeployment = usePostApiV1Deployments();
  const deleteDeployment = useDeleteApiV1DeploymentsId();
  const updateDeployment = usePatchApiV1DeploymentsId();
  const navigate = useNavigate();
  
  // State management
  const [showCreateDialog, setShowCreateDialog] = useState(false);
  const [showEditDialog, setShowEditDialog] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const [statusFilter, setStatusFilter] = useState<DeploymentStatus | 'all'>('all');
  const [runtimeFilter, setRuntimeFilter] = useState<Runtime | 'all'>('all');
  const [editingDeployment, setEditingDeployment] = useState<InternalDeploymentsDeployment | null>(null);
  
  const [newDeployment, setNewDeployment] = useState<InternalDeploymentsCreateDeploymentRequest>({
    name: '',
    description: '',
    version: '',
    runtime: 'node' as Runtime,
    package_url: '',
    package_hash: '',
    config: {
      concurrency: 1,
      max_retries: 3,
      timeout_seconds: 300,
      environment: {},
      browser_requests: []
    } as InternalDeploymentsDeploymentConfig
  });

  // Filter and search deployments
  const filteredDeployments = useMemo(() => {
    if (!deploymentsData?.deployments) return [];
    
    return deploymentsData.deployments.filter(deployment => {
      const matchesSearch = !searchQuery || 
        deployment.name?.toLowerCase().includes(searchQuery.toLowerCase()) ||
        deployment.description?.toLowerCase().includes(searchQuery.toLowerCase()) ||
        deployment.version?.toLowerCase().includes(searchQuery.toLowerCase());
      
      const matchesStatus = statusFilter === 'all' || deployment.status === statusFilter;
      const matchesRuntime = runtimeFilter === 'all' || deployment.runtime === runtimeFilter;
      
      return matchesSearch && matchesStatus && matchesRuntime;
    });
  }, [deploymentsData?.deployments, searchQuery, statusFilter, runtimeFilter]);

  const handleCreateDeployment = async () => {
    try {
      await createDeployment.mutateAsync({ data: newDeployment });
      toast.success('Deployment created successfully');
      setShowCreateDialog(false);
      refetch();
      // Reset form
      setNewDeployment({
        name: '',
        description: '',
        version: '',
        runtime: 'node' as Runtime,
        package_url: '',
        package_hash: '',
        config: {
          concurrency: 1,
          max_retries: 3,
          timeout_seconds: 300,
          environment: {},
          browser_requests: []
        }
      });
    } catch (error) {
      toast.error('Failed to create deployment');
    }
  };

  const handleUpdateDeployment = async () => {
    if (!editingDeployment?.id) return;
    
    try {
      const updateData: InternalDeploymentsUpdateDeploymentRequest = {
        description: editingDeployment.description,
        config: editingDeployment.config as any,
        status: editingDeployment.status
      };
      
      await updateDeployment.mutateAsync({ id: editingDeployment.id, data: updateData });
      toast.success('Deployment updated successfully');
      setShowEditDialog(false);
      setEditingDeployment(null);
      refetch();
    } catch (error) {
      toast.error('Failed to update deployment');
    }
  };

  const handleDeleteDeployment = async (id: string) => {
    try {
      await deleteDeployment.mutateAsync({ id });
      toast.success('Deployment deleted successfully');
      refetch();
    } catch (error) {
      toast.error('Failed to delete deployment');
    }
  };

  const handleViewDetails = (deployment: InternalDeploymentsDeployment) => {
    navigate(`/deployments/${deployment.id}`);
  };

  const handleRunDeployment = async (deployment: InternalDeploymentsDeployment) => {
    // TODO: Implement deployment run functionality
    toast.info('Deployment run functionality coming soon');
  };

  // Stats calculation
  const stats = useMemo(() => {
    if (!deploymentsData?.deployments) return { total: 0, active: 0, inactive: 0, failed: 0 };
    
    const deployments = deploymentsData.deployments;
    return {
      total: deployments.length,
      active: deployments.filter(d => d.status === 'active').length,
      inactive: deployments.filter(d => d.status === 'inactive').length,
      failed: deployments.filter(d => d.status === 'failed').length,
    };
  }, [deploymentsData?.deployments]);

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="flex items-center space-x-2">
          <RefreshCw className="h-4 w-4 animate-spin text-neutral-400" />
          <span className="text-sm text-neutral-600">Loading deployments...</span>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-neutral-900 tracking-tight">
            Deployments
          </h1>
          <p className="text-sm text-neutral-600 mt-1">
            Manage and monitor your automation deployment packages
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button onClick={() => refetch()} variant="outline" size="sm" disabled={isLoading} className="text-xs h-8">
            <RefreshCw className={`h-3 w-3 mr-1.5 ${isLoading ? 'animate-spin' : ''}`} />
            Refresh
          </Button>
          <Dialog open={showCreateDialog} onOpenChange={setShowCreateDialog}>
            <DialogTrigger asChild>
              <Button size="sm" className="bg-neutral-900 hover:bg-neutral-800 text-white text-xs h-8">
                <Plus className="h-3 w-3 mr-1.5" />
                New Deployment
              </Button>
            </DialogTrigger>
            <DeploymentCreateDialog 
              deployment={newDeployment} 
              setDeployment={setNewDeployment}
              onSubmit={handleCreateDeployment}
              isLoading={createDeployment.isPending}
            />
          </Dialog>
        </div>
      </div>

      {/* Stats */}
      <div className="flex items-center gap-6 text-sm">
        <div className="flex items-center gap-2">
          <span className="text-neutral-600">Total:</span>
          <span className="font-semibold text-neutral-900">{stats.total}</span>
        </div>
        <div className="flex items-center gap-2">
          <span className="text-neutral-600">Active:</span>
          <span className="font-semibold text-neutral-900">{stats.active}</span>
        </div>
        <div className="flex items-center gap-2">
          <span className="text-neutral-600">Inactive:</span>
          <span className="font-semibold text-neutral-900">{stats.inactive}</span>
        </div>
        <div className="flex items-center gap-2">
          <span className="text-neutral-600">Failed:</span>
          <span className="font-semibold text-neutral-900">{stats.failed}</span>
        </div>
      </div>

      {/* Filters and Table */}
      <Card className="border-neutral-200/60">
        <CardHeader className="border-b border-neutral-100 bg-neutral-50/30 py-3">
          <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
            <div className="flex items-center gap-3">
              <div className="relative">
                <Search className="absolute left-2.5 top-1/2 h-3 w-3 -translate-y-1/2 text-neutral-400" />
                <Input
                  placeholder="Search deployments..."
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  className="pl-8 h-8 w-64 text-xs border-neutral-200"
                />
              </div>
              <div className="flex items-center gap-2">
                <Filter className="h-3 w-3 text-neutral-400" />
                <Select value={statusFilter} onValueChange={(value) => setStatusFilter(value as DeploymentStatus | 'all')}>
                  <SelectTrigger className="w-28 h-8 text-xs border-neutral-200">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="all">All Status</SelectItem>
                    <SelectItem value="active">Active</SelectItem>
                    <SelectItem value="inactive">Inactive</SelectItem>
                    <SelectItem value="deploying">Deploying</SelectItem>
                    <SelectItem value="failed">Failed</SelectItem>
                    <SelectItem value="deprecated">Deprecated</SelectItem>
                  </SelectContent>
                </Select>
                <Select value={runtimeFilter} onValueChange={(value) => setRuntimeFilter(value as Runtime | 'all')}>
                  <SelectTrigger className="w-28 h-8 text-xs border-neutral-200">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="all">All Runtimes</SelectItem>
                    <SelectItem value="node">Node.js</SelectItem>
                    <SelectItem value="python">Python</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>
          </div>
        </CardHeader>
        <CardContent className="p-0">
          {filteredDeployments.length === 0 ? (
            <EmptyState 
              searchQuery={searchQuery}
              onCreateNew={() => setShowCreateDialog(true)}
            />
          ) : (
            <DeploymentsTable 
              deployments={filteredDeployments}
              onViewDetails={handleViewDetails}
              onEdit={(deployment) => {
                setEditingDeployment(deployment);
                setShowEditDialog(true);
              }}
              onDelete={handleDeleteDeployment}
              onRun={handleRunDeployment}
            />
          )}
        </CardContent>
      </Card>

      {/* Edit Dialog */}
      <Dialog open={showEditDialog} onOpenChange={setShowEditDialog}>
        <DialogContent className="max-w-2xl">
          <DialogHeader>
            <DialogTitle>Edit Deployment</DialogTitle>
            <DialogDescription>
              Update deployment configuration and settings
            </DialogDescription>
          </DialogHeader>
          {editingDeployment && (
            <DeploymentEditForm
              deployment={editingDeployment}
              setDeployment={setEditingDeployment}
              onSubmit={handleUpdateDeployment}
              isLoading={updateDeployment.isPending}
            />
          )}
        </DialogContent>
      </Dialog>
    </div>
  );
}

// Deployment Create Dialog Component
function DeploymentCreateDialog({ 
  deployment, 
  setDeployment, 
  onSubmit, 
  isLoading 
}: {
  deployment: InternalDeploymentsCreateDeploymentRequest;
  setDeployment: (deployment: InternalDeploymentsCreateDeploymentRequest) => void;
  onSubmit: () => void;
  isLoading: boolean;
}) {
  return (
    <DialogContent className="max-w-3xl max-h-[90vh] border-neutral-200">
      <DialogHeader className="border-b border-neutral-100 pb-3">
        <DialogTitle className="text-lg font-semibold">Create New Deployment</DialogTitle>
        <DialogDescription className="text-sm">
          Configure your deployment package settings
        </DialogDescription>
      </DialogHeader>
      <DeploymentForm
        deployment={deployment}
        onDeploymentChange={setDeployment}
        onSubmit={onSubmit}
        onCancel={() => {}}
        isLoading={isLoading}
      />
    </DialogContent>
  );
}

// Deployments Table Component
function DeploymentsTable({ 
  deployments, 
  onViewDetails,
  onEdit,
  onDelete,
  onRun
}: { 
  deployments: InternalDeploymentsDeployment[];
  onViewDetails: (deployment: InternalDeploymentsDeployment) => void;
  onEdit: (deployment: InternalDeploymentsDeployment) => void;
  onDelete: (id: string) => void;
  onRun: (deployment: InternalDeploymentsDeployment) => void;
}) {
  return (
    <Table>
      <TableHeader>
        <TableRow className="border-neutral-100">
          <TableHead className="font-medium text-neutral-700 text-xs h-10">Deployment</TableHead>
          <TableHead className="font-medium text-neutral-700 text-xs h-10">Runtime</TableHead>
          <TableHead className="font-medium text-neutral-700 text-xs h-10">Status</TableHead>
          <TableHead className="font-medium text-neutral-700 text-xs h-10">Runs</TableHead>
          <TableHead className="font-medium text-neutral-700 text-xs h-10">Created</TableHead>
          <TableHead className="text-right font-medium text-neutral-700 text-xs h-10">Actions</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {deployments.map((deployment) => (
          <TableRow key={deployment.id} className="border-neutral-100 hover:bg-neutral-50/50 transition-colors duration-150">
            <TableCell className="py-3">
              <div className="space-y-0.5">
                <div className="font-medium text-neutral-900 text-sm">
                  {deployment.name}
                </div>
                <div className="text-xs text-neutral-500">
                  {deployment.description || 'No description'}
                </div>
                <div className="text-xs text-neutral-400">
                  v{deployment.version}
                </div>
              </div>
            </TableCell>
            <TableCell className="py-3">
              <Badge variant="outline" className="text-xs border-neutral-200 text-neutral-600 px-1.5 py-0">
                {deployment.runtime}
              </Badge>
            </TableCell>
            <TableCell className="py-3">
              <StatusBadge status={deployment.status || 'unknown'} />
            </TableCell>
            <TableCell className="py-3">
              <div className="space-y-0.5">
                <div className="text-xs text-neutral-900">
                  {deployment.total_runs || 0} total
                </div>
                <div className="text-xs text-neutral-500">
                  {deployment.successful_runs || 0} successful
                </div>
              </div>
            </TableCell>
            <TableCell className="py-3">
              <div className="space-y-0.5">
                <div className="text-xs text-neutral-900">
                  {deployment.created_at ? new Date(deployment.created_at).toLocaleDateString() : 'N/A'}
                </div>
                <div className="text-xs text-neutral-500">
                  {deployment.created_at ? new Date(deployment.created_at).toLocaleTimeString() : ''}
                </div>
              </div>
            </TableCell>
            <TableCell className="text-right py-3">
              <div className="flex items-center justify-end gap-1">
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => onViewDetails(deployment)}
                  className="h-7 w-7 p-0 hover:bg-neutral-100"
                >
                  <Eye className="h-3 w-3" />
                </Button>
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => onRun(deployment)}
                  className="h-7 w-7 p-0 hover:bg-neutral-100"
                  disabled={deployment.status !== 'active'}
                >
                  <Play className="h-3 w-3" />
                </Button>
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => onEdit(deployment)}
                  className="h-7 w-7 p-0 hover:bg-neutral-100"
                >
                  <Edit className="h-3 w-3" />
                </Button>
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => onDelete(deployment.id!)}
                  className="h-7 w-7 p-0 hover:bg-neutral-100"
                >
                  <Trash2 className="h-3 w-3" />
                </Button>
              </div>
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}

// Empty State Component
function EmptyState({ 
  searchQuery, 
  onCreateNew 
}: { 
  searchQuery: string;
  onCreateNew: () => void;
}) {
  return (
    <div className="text-center py-12">
      <Package className="h-8 w-8 mx-auto text-neutral-400 mb-3" />
      {searchQuery ? (
        <>
          <h3 className="text-sm font-semibold text-neutral-900 mb-1">No deployments found</h3>
          <p className="text-xs text-neutral-600 mb-4">
            No deployments match your search criteria. Try adjusting your filters.
          </p>
        </>
      ) : (
        <>
          <h3 className="text-sm font-semibold text-neutral-900 mb-1">No deployments yet</h3>
          <p className="text-xs text-neutral-600 mb-4">
            Get started by creating your first deployment package.
          </p>
        </>
      )}
      <Button onClick={onCreateNew} size="sm" className="bg-neutral-900 hover:bg-neutral-800 text-white text-xs h-8">
        <Plus className="h-3 w-3 mr-1.5" />
        Create New Deployment
      </Button>
    </div>
  );
}

// Deployment Form Component
function DeploymentForm({
  deployment,
  onDeploymentChange,
  onSubmit,
  onCancel,
  isLoading
}: {
  deployment: InternalDeploymentsCreateDeploymentRequest;
  onDeploymentChange: (deployment: InternalDeploymentsCreateDeploymentRequest) => void;
  onSubmit: () => void;
  onCancel: () => void;
  isLoading: boolean;
}) {
  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 gap-4">
        <div>
          <label className="text-sm font-medium text-neutral-700">Name</label>
          <Input
            value={deployment.name}
            onChange={(e) => onDeploymentChange({ ...deployment, name: e.target.value })}
            placeholder="my-deployment"
            className="mt-1"
          />
        </div>
        <div>
          <label className="text-sm font-medium text-neutral-700">Version</label>
          <Input
            value={deployment.version}
            onChange={(e) => onDeploymentChange({ ...deployment, version: e.target.value })}
            placeholder="1.0.0"
            className="mt-1"
          />
        </div>
      </div>
      
      <div>
        <label className="text-sm font-medium text-neutral-700">Description</label>
        <Input
          value={deployment.description}
          onChange={(e) => onDeploymentChange({ ...deployment, description: e.target.value })}
          placeholder="Description of the deployment"
          className="mt-1"
        />
      </div>
      
      <div className="grid grid-cols-2 gap-4">
        <div>
          <label className="text-sm font-medium text-neutral-700">Runtime</label>
          <Select 
            value={deployment.runtime} 
            onValueChange={(value) => onDeploymentChange({ ...deployment, runtime: value as Runtime })}
          >
            <SelectTrigger className="mt-1">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="node">Node.js</SelectItem>
              <SelectItem value="python">Python</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div>
          <label className="text-sm font-medium text-neutral-700">Package URL</label>
          <Input
            value={deployment.package_url}
            onChange={(e) => onDeploymentChange({ ...deployment, package_url: e.target.value })}
            placeholder="https://example.com/package.zip"
            className="mt-1"
          />
        </div>
      </div>
      
      <div>
        <label className="text-sm font-medium text-neutral-700">Package Hash</label>
        <Input
          value={deployment.package_hash}
          onChange={(e) => onDeploymentChange({ ...deployment, package_hash: e.target.value })}
          placeholder="sha256:..."
          className="mt-1"
        />
      </div>
      
      <div className="flex justify-end gap-2 pt-4">
        <Button variant="outline" onClick={onCancel}>
          Cancel
        </Button>
        <Button onClick={onSubmit} disabled={isLoading}>
          {isLoading ? 'Creating...' : 'Create Deployment'}
        </Button>
      </div>
    </div>
  );
}

// Deployment Edit Form Component
function DeploymentEditForm({
  deployment,
  setDeployment,
  onSubmit,
  isLoading
}: {
  deployment: InternalDeploymentsDeployment;
  setDeployment: (deployment: InternalDeploymentsDeployment) => void;
  onSubmit: () => void;
  isLoading: boolean;
}) {
  return (
    <div className="space-y-4">
      <div>
        <label className="text-sm font-medium text-neutral-700">Description</label>
        <Input
          value={deployment.description || ''}
          onChange={(e) => setDeployment({ ...deployment, description: e.target.value })}
          placeholder="Description of the deployment"
          className="mt-1"
        />
      </div>
      
      <div>
        <label className="text-sm font-medium text-neutral-700">Status</label>
        <Select 
          value={deployment.status || 'active'} 
          onValueChange={(value) => setDeployment({ ...deployment, status: value as DeploymentStatus })}
        >
          <SelectTrigger className="mt-1">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="active">Active</SelectItem>
            <SelectItem value="inactive">Inactive</SelectItem>
            <SelectItem value="deprecated">Deprecated</SelectItem>
          </SelectContent>
        </Select>
      </div>
      
      <div className="flex justify-end gap-2 pt-4">
        <Button onClick={onSubmit} disabled={isLoading}>
          {isLoading ? 'Updating...' : 'Update Deployment'}
        </Button>
      </div>
    </div>
  );
} 