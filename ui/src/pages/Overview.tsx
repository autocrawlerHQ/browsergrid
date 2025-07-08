import React from 'react';
import { Globe, Layers, Activity } from 'lucide-react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { useGetApiV1Sessions } from '@/lib/api/sessions/sessions';
import { useGetApiV1Workpools } from '@/lib/api/workpools/workpools';
import { useQuery } from '@tanstack/react-query';

export default function Overview() {
  const { data: sessions } = useGetApiV1Sessions();
  const { data: workpools } = useGetApiV1Workpools();
  
  // Fetch queue data from Asynqmon
  const { data: queuesData } = useQuery({
    queryKey: ['overview-queues'],
    queryFn: async () => {
      try {
        const response = await fetch('http://localhost:4444/api/queues');
        if (!response.ok) return null;
        return response.json();
      } catch {
        return null;
      }
    },
    refetchInterval: 5000,
  });

  const totalTasks = queuesData?.queues?.reduce((acc: number, q: any) => acc + q.size, 0) || 0;
  const totalProcessed = queuesData?.queues?.reduce((acc: number, q: any) => acc + q.processed, 0) || 0;

  const stats = [
    {
      title: 'Active Sessions',
      value: sessions?.sessions?.filter((s: any) => s.status === 'running' || s.status === 'active').length || 0,
      total: sessions?.total || 0,
      icon: Globe,
      color: 'text-blue-600'
    },
    {
      title: 'Work Pools',
      value: workpools?.pools?.filter((p: any) => !p.paused).length || 0,
      total: workpools?.total || 0,
      icon: Layers,
      color: 'text-green-600'
    },
    {
      title: 'Queued Tasks',
      value: totalTasks,
      total: totalProcessed,
      icon: Activity,
      color: 'text-purple-600',
      subtitle: 'processed total'
    },
  ];

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">Overview</h1>
        <p className="text-muted-foreground">
          Monitor your browser infrastructure at a glance
        </p>
      </div>
      
      <div className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-4">
        {stats.map((stat, index) => (
          <Card key={index}>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">{stat.title}</CardTitle>
              <stat.icon className={`h-4 w-4 ${stat.color}`} />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{stat.value}</div>
              <p className="text-xs text-muted-foreground">
                {stat.subtitle ? stat.subtitle : `of ${stat.total} total`}
              </p>
            </CardContent>
          </Card>
        ))}
      </div>
    </div>
  );
}