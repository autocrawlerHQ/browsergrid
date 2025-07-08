import React from 'react';
import MonitoringLayout from './worker/layout';

// This is an example of how to use the monitoring components
// In your actual app, you would import these from their respective files

export default function App() {
  return (
    <div className="h-screen bg-background">
      <MonitoringLayout />
    </div>
  );
}

// Example of how to use individual components if you don't want the full layout:
/*
import Metrics from './Metrics';
import Queues from './Queues';
import Workers from './Workers';
import Schedulers from './Schedulers';

export default function App() {
  return (
    <div className="container mx-auto p-6">
      <Metrics />
      // or
      <Queues />
      // or
      <Workers />
      // or
      <Schedulers />
    </div>
  );
}
*/