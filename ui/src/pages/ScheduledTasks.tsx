import React, { useState, useEffect } from 'react';
import { Clock, Plus, Play, Pause, Trash2, Edit, Calendar, RefreshCw, AlertCircle, CheckCircle, XCircle } from 'lucide-react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Textarea } from '@/components/ui/textarea';
import { Switch } from '@/components/ui/switch';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useGetApiV1Workpools } from '@/lib/api/workpools/workpools';
import { toast } from 'sonner';

// API configuration
const ASYNQMON_API = 'http://localhost:4444/api';

// Task types available for scheduling
const TASK_TYPES = [
  { value: 'session:start', label: 'Start Session', description: 'Start a new browser session' },
  { value: 'pool:scale', label: 'Scale Pool', description: 'Scale a work pool to desired size' },
  { value: 'cleanup:expired', label: 'Cleanup Expired', description: 'Clean up expired sessions' },
  { value: 'session:health_check', label: 'Health Check', description: 'Check session health' },
  { value: 'session:timeout', label: 'Session Timeout', description: 'Handle session timeout' },
];

// Queue options
const QUEUE_OPTIONS = [
  { value: 'default', label: 'Default' },
  { value: 'critical', label: 'Critical' },
  { value: 'low', label: 'Low Priority' },
  { value: 'scheduler', label: 'Scheduler' },
];

// Common cron patterns
const CRON_PRESETS = [
  { value: '0 * * * *', label: 'Every hour' },
  { value: '0 */6 * * *', label: 'Every 6 hours' },
  { value: '0 9 * * *', label: 'Daily at 9 AM' },
  { value: '0 9 * * 1', label: 'Weekly on Monday at 9 AM' },
  { value: '0 9 1 * *', label: 'Monthly on 1st at 9 AM' },
];

interface ScheduledTask {
  id: string;
  spec: string;
  task: {
    type: string;
    payload: any;
  };
  next_enqueue_at: string;
  prev_enqueue_at?: string;
  opts: {
    queue: string;
    max_retry: number;
    timeout: number;
  };
}

interface TaskFormData {
  type: string;
  schedule: string;
  queue: string;
  max_retry: number;
  timeout: number;
  payload: any;
  enabled: boolean;
}

// Custom fetch wrapper for Asynqmon API
const fetchAsynqmon = async (endpoint: string, options?: RequestInit) => {
  const response = await fetch(`${ASYNQMON_API}${endpoint}`, {
    headers: {
      'Content-Type': 'application/json',
      ...options?.headers,
    },
    ...options,
  });
  if (!response.ok) {
    throw new Error(`API error: ${response.status}`);
  }
  return response.json();
};

export default function ScheduledTasks() {
  const [showCreateDialog, setShowCreateDialog] = useState(false);
  const [selectedTask, setSelectedTask] = useState<ScheduledTask | null>(null);
  const queryClient = useQueryClient();
  const { data: workPools } = useGetApiV1Workpools();

  const [formData, setFormData] = useState<TaskFormData>({
    type: '',
    schedule: '',
    queue: 'default',
    max_retry: 3,
    timeout: 300,
    payload: {},
    enabled: true,
  });

  // Fetch scheduled tasks
  const { data: scheduledTasks, isLoading, error } = useQuery({
    queryKey: ['scheduled-tasks'],
    queryFn: () => fetchAsynqmon('/schedulers'),
    refetchInterval: 5000,
  });

  // Create scheduled task mutation
  const createTaskMutation = useMutation({
    mutationFn: async (data: TaskFormData) => {
      const taskPayload = buildTaskPayload(data);
      return fetchAsynqmon('/schedulers', {
        method: 'POST',
        body: JSON.stringify({
          spec: data.schedule,
          task: {
            type: data.type,
            payload: taskPayload,
          },
          opts: {
            queue: data.queue,
            max_retry: data.max_retry,
            timeout: data.timeout,
          },
        }),
      });
    },
    onSuccess: () => {
      toast.success('Scheduled task created successfully');
      setShowCreateDialog(false);
      queryClient.invalidateQueries({ queryKey: ['scheduled-tasks'] });
      resetForm();
    },
    onError: (error) => {
      toast.error(`Failed to create scheduled task: ${error.message}`);
    },
  });

  // Delete scheduled task mutation
  const deleteTaskMutation = useMutation({
    mutationFn: async (taskId: string) => {
      return fetchAsynqmon(`/schedulers/${taskId}`, {
        method: 'DELETE',
      });
    },
    onSuccess: () => {
      toast.success('Scheduled task deleted successfully');
      queryClient.invalidateQueries({ queryKey: ['scheduled-tasks'] });
    },
    onError: (error) => {
      toast.error(`Failed to delete scheduled task: ${error.message}`);
    },
  });

  const buildTaskPayload = (data: TaskFormData) => {
    switch (data.type) {
      case 'pool:scale':
        return {
          work_pool_id: data.payload.work_pool_id,
          desired_sessions: parseInt(data.payload.desired_sessions) || 1,
        };
      case 'cleanup:expired':
        return {
          max_age: parseInt(data.payload.max_age) || 24,
        };
      case 'session:start':
        return {
          session_id: data.payload.session_id,
          work_pool_id: data.payload.work_pool_id,
          max_session_duration: parseInt(data.payload.max_session_duration) || 3600,
          redis_addr: data.payload.redis_addr || 'redis:6379',
          queue_name: data.queue,
        };
      case 'session:health_check':
        return {
          session_id: data.payload.session_id,
          redis_addr: data.payload.redis_addr || 'redis:6379',
        };
      case 'session:timeout':
        return {
          session_id: data.payload.session_id,
        };
      default:
        return data.payload;
    }
  };

  const resetForm = () => {
    setFormData({
      type: '',
      schedule: '',
      queue: 'default',
      max_retry: 3,
      timeout: 300,
      payload: {},
      enabled: true,
    });
  };

  const handleCreateTask = () => {
    if (!formData.type || !formData.schedule) {
      toast.error('Please fill in all required fields');
      return;
    }
    createTaskMutation.mutate(formData);
  };

  const handleDeleteTask = (taskId: string) => {
    if (confirm('Are you sure you want to delete this scheduled task?')) {
      deleteTaskMutation.mutate(taskId);
    }
  };

  const formatNextRun = (nextEnqueueAt: string) => {
    const date = new Date(nextEnqueueAt);
    const now = new Date();
    const diffMs = date.getTime() - now.getTime();
    const diffMinutes = Math.floor(diffMs / 60000);
    const diffHours = Math.floor(diffMinutes / 60);
    const diffDays = Math.floor(diffHours / 24);

    if (diffDays > 0) return `in ${diffDays} days`;
    if (diffHours > 0) return `in ${diffHours} hours`;
    if (diffMinutes > 0) return `in ${diffMinutes} minutes`;
    return 'soon';
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="flex items-center space-x-2">
          <RefreshCw className="h-4 w-4 animate-spin text-neutral-400" />
          <span className="text-sm text-neutral-600">Loading scheduled tasks...</span>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-center">
          <AlertCircle className="h-8 w-8 mx-auto text-red-500 mb-2" />
          <p className="text-sm text-red-600">Failed to load scheduled tasks</p>
          <p className="text-xs text-neutral-500 mt-1">Make sure Asynqmon is running</p>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-neutral-900 tracking-tight">
            Scheduled Tasks
          </h1>
          <p className="text-sm text-neutral-600 mt-1">
            Create and manage recurring background tasks
          </p>
        </div>
        <Dialog open={showCreateDialog} onOpenChange={setShowCreateDialog}>
          <DialogTrigger asChild>
            <Button className="bg-neutral-900 hover:bg-neutral-800 text-white">
              <Plus className="h-4 w-4 mr-2" />
              Schedule Task
            </Button>
          </DialogTrigger>
          <DialogContent className="max-w-2xl">
            <DialogHeader>
              <DialogTitle>Schedule New Task</DialogTitle>
              <DialogDescription>
                Create a new recurring task to run on a schedule
              </DialogDescription>
            </DialogHeader>
            <TaskForm
              formData={formData}
              setFormData={setFormData}
              workPools={workPools?.pools || []}
              onSubmit={handleCreateTask}
              onCancel={() => setShowCreateDialog(false)}
              isLoading={createTaskMutation.isPending}
            />
          </DialogContent>
        </Dialog>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-neutral-600">Total Tasks</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-neutral-900">
              {scheduledTasks?.length || 0}
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-neutral-600">Active Tasks</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-green-600">
              {scheduledTasks?.filter((t: ScheduledTask) => t.next_enqueue_at)?.length || 0}
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-neutral-600">Next Task</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-sm text-neutral-900">
              {scheduledTasks?.length > 0 ? (
                formatNextRun(scheduledTasks[0].next_enqueue_at)
              ) : (
                'No tasks scheduled'
              )}
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Tasks Table */}
      <Card>
        <CardHeader>
          <CardTitle className="text-lg">Scheduled Tasks</CardTitle>
        </CardHeader>
        <CardContent className="p-0">
          {scheduledTasks?.length === 0 ? (
            <div className="text-center py-12">
              <Clock className="h-8 w-8 mx-auto text-neutral-400 mb-3" />
              <h3 className="text-sm font-semibold text-neutral-900 mb-1">No scheduled tasks</h3>
              <p className="text-xs text-neutral-600 mb-4">
                Create your first scheduled task to automate recurring operations.
              </p>
              <Button onClick={() => setShowCreateDialog(true)} size="sm">
                <Plus className="h-3 w-3 mr-1.5" />
                Schedule Task
              </Button>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Task</TableHead>
                  <TableHead>Schedule</TableHead>
                  <TableHead>Queue</TableHead>
                  <TableHead>Next Run</TableHead>
                  <TableHead>Last Run</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {scheduledTasks?.map((task: ScheduledTask) => (
                  <TableRow key={task.id}>
                    <TableCell>
                      <div className="space-y-1">
                        <div className="font-medium text-sm">
                          {TASK_TYPES.find(t => t.value === task.task.type)?.label || task.task.type}
                        </div>
                        <div className="text-xs text-neutral-500">
                          {task.id.substring(0, 8)}...
                        </div>
                      </div>
                    </TableCell>
                    <TableCell>
                      <div className="font-mono text-xs bg-neutral-100 px-2 py-1 rounded">
                        {task.spec}
                      </div>
                    </TableCell>
                    <TableCell>
                      <Badge variant="outline" className="text-xs">
                        {task.opts.queue}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <div className="text-sm">
                        {formatNextRun(task.next_enqueue_at)}
                      </div>
                      <div className="text-xs text-neutral-500">
                        {new Date(task.next_enqueue_at).toLocaleString()}
                      </div>
                    </TableCell>
                    <TableCell>
                      {task.prev_enqueue_at ? (
                        <div className="text-xs text-neutral-500">
                          {new Date(task.prev_enqueue_at).toLocaleString()}
                        </div>
                      ) : (
                        <span className="text-xs text-neutral-400">Never</span>
                      )}
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center gap-1">
                        <CheckCircle className="h-3 w-3 text-green-500" />
                        <span className="text-xs text-green-600">Active</span>
                      </div>
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="flex items-center justify-end gap-1">
                        <Button
                          size="sm"
                          variant="ghost"
                          onClick={() => handleDeleteTask(task.id)}
                          className="h-7 w-7 p-0 hover:bg-red-50 hover:text-red-600"
                        >
                          <Trash2 className="h-3 w-3" />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

// Task Form Component
function TaskForm({
  formData,
  setFormData,
  workPools,
  onSubmit,
  onCancel,
  isLoading,
}: {
  formData: TaskFormData;
  setFormData: (data: TaskFormData) => void;
  workPools: any[];
  onSubmit: () => void;
  onCancel: () => void;
  isLoading: boolean;
}) {
  const updateFormData = (field: string, value: any) => {
    setFormData({ ...formData, [field]: value });
  };

  const updatePayload = (field: string, value: any) => {
    setFormData({
      ...formData,
      payload: { ...formData.payload, [field]: value },
    });
  };

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 gap-4">
        <div>
          <Label htmlFor="type">Task Type</Label>
          <Select value={formData.type} onValueChange={(value) => updateFormData('type', value)}>
            <SelectTrigger>
              <SelectValue placeholder="Select task type" />
            </SelectTrigger>
            <SelectContent>
              {TASK_TYPES.map((type) => (
                <SelectItem key={type.value} value={type.value}>
                  <div>
                    <div className="font-medium">{type.label}</div>
                    <div className="text-xs text-neutral-500">{type.description}</div>
                  </div>
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <div>
          <Label htmlFor="queue">Queue</Label>
          <Select value={formData.queue} onValueChange={(value) => updateFormData('queue', value)}>
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {QUEUE_OPTIONS.map((queue) => (
                <SelectItem key={queue.value} value={queue.value}>
                  {queue.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>

      <div>
        <Label htmlFor="schedule">Schedule (Cron Expression)</Label>
        <div className="space-y-2">
          <Input
            placeholder="0 * * * * (every hour)"
            value={formData.schedule}
            onChange={(e) => updateFormData('schedule', e.target.value)}
          />
          <div className="flex flex-wrap gap-2">
            {CRON_PRESETS.map((preset) => (
              <Button
                key={preset.value}
                size="sm"
                variant="outline"
                onClick={() => updateFormData('schedule', preset.value)}
                className="text-xs"
              >
                {preset.label}
              </Button>
            ))}
          </div>
        </div>
      </div>

      <div className="grid grid-cols-2 gap-4">
        <div>
          <Label htmlFor="max_retry">Max Retries</Label>
          <Input
            type="number"
            min="0"
            max="10"
            value={formData.max_retry}
            onChange={(e) => updateFormData('max_retry', parseInt(e.target.value))}
          />
        </div>
        <div>
          <Label htmlFor="timeout">Timeout (seconds)</Label>
          <Input
            type="number"
            min="60"
            max="3600"
            value={formData.timeout}
            onChange={(e) => updateFormData('timeout', parseInt(e.target.value))}
          />
        </div>
      </div>

      {/* Task-specific payload fields */}
      {formData.type === 'pool:scale' && (
        <div className="space-y-3">
          <Label>Pool Scaling Configuration</Label>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <Label htmlFor="work_pool_id">Work Pool</Label>
              <Select
                value={formData.payload.work_pool_id}
                onValueChange={(value) => updatePayload('work_pool_id', value)}
              >
                <SelectTrigger>
                  <SelectValue placeholder="Select work pool" />
                </SelectTrigger>
                <SelectContent>
                  {workPools.map((pool) => (
                    <SelectItem key={pool.id} value={pool.id}>
                      {pool.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div>
              <Label htmlFor="desired_sessions">Desired Sessions</Label>
              <Input
                type="number"
                min="1"
                max="100"
                value={formData.payload.desired_sessions || ''}
                onChange={(e) => updatePayload('desired_sessions', e.target.value)}
              />
            </div>
          </div>
        </div>
      )}

      {formData.type === 'cleanup:expired' && (
        <div>
          <Label htmlFor="max_age">Max Age (hours)</Label>
          <Input
            type="number"
            min="1"
            max="720"
            value={formData.payload.max_age || '24'}
            onChange={(e) => updatePayload('max_age', e.target.value)}
          />
        </div>
      )}

      <div className="flex items-center justify-between pt-4">
        <div className="flex items-center space-x-2">
          <Switch
            checked={formData.enabled}
            onCheckedChange={(checked) => updateFormData('enabled', checked)}
          />
          <Label>Enable task</Label>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={onCancel}>
            Cancel
          </Button>
          <Button onClick={onSubmit} disabled={isLoading}>
            {isLoading ? (
              <>
                <RefreshCw className="h-3 w-3 mr-1.5 animate-spin" />
                Creating...
              </>
            ) : (
              'Create Task'
            )}
          </Button>
        </div>
      </div>
    </div>
  );
} 