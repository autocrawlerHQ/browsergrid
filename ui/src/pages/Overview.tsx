import React from 'react';
import { Globe, Layers, Pickaxe, Activity } from 'lucide-react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { useGetApiV1Sessions } from '@/lib/api/sessions/sessions';
import { useGetApiV1Workpools } from '@/lib/api/workpools/workpools';
import { useGetApiV1Workers } from '@/lib/api/workers/workers';

export default function Overview() {
  const { data: sessions } = useGetApiV1Sessions();
  const { data: workpools } = useGetApiV1Workpools();
  const { data: workers } = useGetApiV1Workers();

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
      title: 'Online Workers',
      value: workers?.workers?.filter((w: any) => !w.paused).length || 0,
      total: workers?.total || 0,
      icon: Pickaxe,
      color: 'text-purple-600'
    },
    {
      title: 'Total Capacity',
      value: workers?.workers?.reduce((acc: number, w: any) => acc + (w.max_slots || 0), 0) || 0,
      total: workers?.workers?.reduce((acc: number, w: any) => acc + (w.active || 0), 0) || 0,
      icon: Activity,
      color: 'text-orange-600'
    }
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
                of {stat.total} total
              </p>
            </CardContent>
          </Card>
        ))}
      </div>
    </div>
  );
} 