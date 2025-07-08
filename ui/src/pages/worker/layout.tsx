import React, { useState } from 'react';
import MonitoringSidebar from './sidebar';
import Metrics from './metrics';
import Queues from './queues';
import Workers from './Workers'; // The existing Workers component you provided
import Schedulers from './scheduler';

// Import other page components as needed
// import Redis from './Redis';

export default function MonitoringLayout() {
  const [activeView, setActiveView] = useState('queues');
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  
  const renderContent = () => {
    switch (activeView) {
      case 'queues':
        return <Queues />;
      case 'servers':
        return <Workers />;
      case 'metrics':
        return <Metrics />;
      case 'schedulers':
        return <Schedulers />;
      case 'redis':
        return (
          <div className="p-6">
            <h1 className="text-3xl font-bold mb-2">Redis</h1>
            <p className="text-muted-foreground">Redis server information and stats</p>
            {/* Add Redis component here */}
          </div>
        );
      case 'settings':
        return (
          <div className="p-6">
            <h1 className="text-3xl font-bold mb-2">Settings</h1>
            <p className="text-muted-foreground">Configure monitoring preferences</p>
            {/* Add Settings component here */}
          </div>
        );
      case 'feedback':
        return (
          <div className="p-6">
            <h1 className="text-3xl font-bold mb-2">Send Feedback</h1>
            <p className="text-muted-foreground">Help us improve Asynq Monitoring</p>
            {/* Add Feedback component here */}
          </div>
        );
      default:
        return <Queues />;
    }
  };
  
  return (
    <div className="flex h-screen bg-background">
      {/* Sidebar */}
      <MonitoringSidebar
        collapsed={sidebarCollapsed}
        onToggle={() => setSidebarCollapsed(!sidebarCollapsed)}
        activeItem={activeView}
        onNavigate={setActiveView}
      />
      
      {/* Main Content */}
      <main className="flex-1 overflow-y-auto">
        <div className="container mx-auto p-6">
          {renderContent()}
        </div>
      </main>
    </div>
  );
}