import React from 'react';
import { 
  BarChart3, 
  Server, 
  Clock, 
  Database, 
  LineChart,
  Settings,
  MessageSquare,
  Menu
} from 'lucide-react';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';

interface NavItem {
  name: string;
  id: string;
  icon: React.ComponentType<{ className?: string }>;
}

const navigation: NavItem[] = [
  { name: 'Queues', id: 'queues', icon: BarChart3 },
  { name: 'Servers', id: 'servers', icon: Server },
  { name: 'Schedulers', id: 'schedulers', icon: Clock },
  { name: 'Redis', id: 'redis', icon: Database },
  { name: 'Metrics', id: 'metrics', icon: LineChart },
];

const bottomNavigation: NavItem[] = [
  { name: 'Settings', id: 'settings', icon: Settings },
  { name: 'Send Feedback', id: 'feedback', icon: MessageSquare },
];

interface MonitoringSidebarProps {
  collapsed?: boolean;
  onToggle?: () => void;
  activeItem?: string;
  onNavigate?: (itemId: string) => void;
}

export default function MonitoringSidebar({ 
  collapsed = false, 
  onToggle,
  activeItem = 'queues',
  onNavigate 
}: MonitoringSidebarProps) {
  
  return (
    <div className={cn(
      "flex flex-col h-full bg-background border-r",
      collapsed ? "w-16" : "w-64"
    )}>
      {/* Header */}
      <div className="flex items-center justify-between p-4 border-b">
        <div className={cn(
          "flex items-center gap-2",
          collapsed && "justify-center"
        )}>
          <Button
            variant="ghost"
            size="icon"
            onClick={onToggle}
            className="h-8 w-8"
          >
            <Menu className="h-4 w-4" />
          </Button>
          {!collapsed && (
            <h2 className="text-lg font-semibold">Asynq Monitoring</h2>
          )}
        </div>
      </div>
      
      {/* Main Navigation */}
      <nav className="flex-1 p-2">
        <ul className="space-y-1">
          {navigation.map((item) => {
            const Icon = item.icon;
            const isActive = activeItem === item.id;
            
            return (
              <li key={item.name}>
                <button
                  onClick={() => onNavigate?.(item.id)}
                  className={cn(
                    "w-full flex items-center gap-3 px-3 py-2 rounded-md text-sm font-medium transition-colors",
                    isActive 
                      ? "bg-primary/10 text-primary" 
                      : "text-muted-foreground hover:bg-muted hover:text-foreground",
                    collapsed && "justify-center"
                  )}
                  title={collapsed ? item.name : undefined}
                >
                  <Icon className="h-4 w-4 flex-shrink-0" />
                  {!collapsed && <span>{item.name}</span>}
                </button>
              </li>
            );
          })}
        </ul>
      </nav>
      
      {/* Bottom Navigation */}
      <div className="border-t p-2">
        <ul className="space-y-1">
          {bottomNavigation.map((item) => {
            const Icon = item.icon;
            
            return (
              <li key={item.name}>
                <button
                  onClick={() => onNavigate?.(item.id)}
                  className={cn(
                    "w-full flex items-center gap-3 px-3 py-2 rounded-md text-sm font-medium transition-colors",
                    "text-muted-foreground hover:bg-muted hover:text-foreground",
                    collapsed && "justify-center"
                  )}
                  title={collapsed ? item.name : undefined}
                >
                  <Icon className="h-4 w-4 flex-shrink-0" />
                  {!collapsed && <span>{item.name}</span>}
                </button>
              </li>
            );
          })}
        </ul>
      </div>
    </div>
  );
}