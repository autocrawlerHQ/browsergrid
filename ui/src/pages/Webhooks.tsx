import React from 'react';
import { Plus, Webhook } from 'lucide-react';
import { Card, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';

export default function Webhooks() {
  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-3xl font-bold">Webhooks</h1>
          <p className="text-muted-foreground">
            Configure webhooks for session events and notifications
          </p>
        </div>
        <Button>
          <Plus className="h-4 w-4 mr-2" />
          New Webhook
        </Button>
      </div>
      
      <Card>
        <CardContent className="flex items-center justify-center h-64">
          <div className="text-center">
            <Webhook className="h-12 w-12 mx-auto mb-4 text-muted-foreground" />
            <p className="text-muted-foreground">Webhook management coming soon</p>
          </div>
        </CardContent>
      </Card>
    </div>
  );
} 