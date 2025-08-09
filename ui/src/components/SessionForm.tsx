import React from 'react';
import { RefreshCw, User, Info } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { Separator } from '@/components/ui/separator';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { useGetApiV1Profiles } from '@/lib/api/profiles/profiles';
import type { Session, Browser, BrowserVersion, OperatingSystem } from '@/lib/api/model';

interface SessionFormProps {
  session: Partial<Session>;
  onSessionChange: (session: Partial<Session>) => void;
  onSubmit: () => void;
  onCancel: () => void;
  isLoading?: boolean;
}

export function SessionForm({ 
  session, 
  onSessionChange, 
  onSubmit, 
  onCancel, 
  isLoading = false 
}: SessionFormProps) {
  const { data: profilesData } = useGetApiV1Profiles();
  
  const updateSession = (updates: Partial<Session>) => {
    onSessionChange({ ...session, ...updates });
  };

  const updateScreen = (screenUpdates: Partial<typeof session.screen>) => {
    updateSession({
      screen: {
        width: session.screen?.width || 1920,
        height: session.screen?.height || 1080,
        dpi: session.screen?.dpi || 96,
        scale: session.screen?.scale || 1.0,
        ...session.screen,
        ...screenUpdates,
      }
    });
  };

  const updateResourceLimits = (limitsUpdates: Partial<typeof session.resource_limits>) => {
    updateSession({
      resource_limits: {
        ...session.resource_limits,
        ...limitsUpdates,
      }
    });
  };

  return (
    <ScrollArea className="max-h-[70vh] px-1">
      <div className="space-y-6 py-4">
        {/* Browser Configuration */}
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base">Browser Configuration</CardTitle>
            <CardDescription>
              Choose your browser type, version, and operating system
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label htmlFor="browser">Browser</Label>
                <Select 
                  value={session.browser} 
                  onValueChange={(value) => updateSession({ browser: value as Browser })}
                >
                  <SelectTrigger>
                    <SelectValue placeholder="Select browser" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="chrome">Chrome</SelectItem>
                    <SelectItem value="chromium">Chromium</SelectItem>
                    <SelectItem value="firefox">Firefox</SelectItem>
                    <SelectItem value="edge">Edge</SelectItem>
                    <SelectItem value="webkit">Webkit</SelectItem>
                    <SelectItem value="safari">Safari</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <Label htmlFor="version">Version</Label>
                <Select 
                  value={session.version} 
                  onValueChange={(value) => updateSession({ version: value as BrowserVersion })}
                >
                  <SelectTrigger>
                    <SelectValue placeholder="Select version" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="latest">Latest</SelectItem>
                    <SelectItem value="stable">Stable</SelectItem>
                    <SelectItem value="canary">Canary</SelectItem>
                    <SelectItem value="dev">Dev</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>
            
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label htmlFor="os">Operating System</Label>
                <Select 
                  value={session.operating_system} 
                  onValueChange={(value) => updateSession({ operating_system: value as OperatingSystem })}
                >
                  <SelectTrigger>
                    <SelectValue placeholder="Select OS" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="linux">Linux</SelectItem>
                    <SelectItem value="windows">Windows</SelectItem>
                    <SelectItem value="macos">macOS</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <Label htmlFor="provider">Provider</Label>
                <Select 
                  value={session.provider || 'docker'} 
                  onValueChange={(value) => updateSession({ provider: value })}
                >
                  <SelectTrigger>
                    <SelectValue placeholder="Select provider" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="docker">Docker</SelectItem>
                    <SelectItem value="local">Local</SelectItem>
                    <SelectItem value="kubernetes">Kubernetes</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>
            
            <div className="flex items-center space-x-2">
              <Switch 
                id="headless"
                checked={session.headless} 
                onCheckedChange={(checked) => updateSession({ headless: checked })}
              />
              <Label htmlFor="headless">Headless mode</Label>
            </div>
          </CardContent>
        </Card>

        {/* Profile Configuration */}
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base flex items-center gap-2">
              <User className="h-4 w-4" />
              Profile Configuration
            </CardTitle>
            <CardDescription>
              Optionally attach a browser profile to save and restore session state
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="profile">Browser Profile (Optional)</Label>
              <Select 
                value={session.profile_id || 'none'} 
                onValueChange={(value) => updateSession({ profile_id: value === 'none' ? undefined : value })}
              >
                <SelectTrigger>
                  <SelectValue placeholder="Select a profile (optional)" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="none">No profile</SelectItem>
                  {profilesData?.profiles?.map((profile) => (
                    <SelectItem key={profile.id} value={profile.id || ''}>
                      <div className="flex items-center justify-between w-full">
                        <span>{profile.name}</span>
                        <Badge variant="outline" className="ml-2 text-xs">
                          {profile.browser}
                        </Badge>
                      </div>
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {session.profile_id && session.profile_id !== 'none' && profilesData?.profiles && (
                <div className="text-xs text-neutral-600 flex items-start gap-2 p-2 bg-neutral-50 rounded">
                  <Info className="h-3 w-3 mt-0.5 flex-shrink-0" />
                  <div>
                    <div className="font-medium">
                      {profilesData.profiles.find(p => p.id === session.profile_id)?.name}
                    </div>
                    <div className="text-neutral-500">
                      {profilesData.profiles.find(p => p.id === session.profile_id)?.description || 'No description'}
                    </div>
                    <div className="text-neutral-500">
                      Size: {profilesData.profiles.find(p => p.id === session.profile_id)?.size_bytes 
                        ? `${(profilesData.profiles.find(p => p.id === session.profile_id)!.size_bytes! / 1024 / 1024).toFixed(1)} MB`
                        : 'Unknown'
                      }
                    </div>
                  </div>
                </div>
              )}
            </div>
          </CardContent>
        </Card>

        {/* Screen Configuration */}
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base">Screen Configuration</CardTitle>
            <CardDescription>
              Set the screen dimensions and display properties
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label htmlFor="width">Width (pixels)</Label>
                <Input 
                  id="width"
                  type="number"
                  min="800"
                  max="3840"
                  value={session.screen?.width || 1920} 
                  onChange={(e) => updateScreen({ width: parseInt(e.target.value) || 1920 })}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="height">Height (pixels)</Label>
                <Input 
                  id="height"
                  type="number"
                  min="600"
                  max="2160"
                  value={session.screen?.height || 1080} 
                  onChange={(e) => updateScreen({ height: parseInt(e.target.value) || 1080 })}
                />
              </div>
            </div>
            
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label htmlFor="dpi">DPI</Label>
                <Select 
                  value={session.screen?.dpi?.toString() || '96'} 
                  onValueChange={(value) => updateScreen({ dpi: parseInt(value) })}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="96">96 DPI (Standard)</SelectItem>
                    <SelectItem value="120">120 DPI (High)</SelectItem>
                    <SelectItem value="144">144 DPI (Very High)</SelectItem>
                    <SelectItem value="192">192 DPI (Ultra High)</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <Label htmlFor="scale">Scale Factor</Label>
                <Select 
                  value={session.screen?.scale?.toString() || '1.0'} 
                  onValueChange={(value) => updateScreen({ scale: parseFloat(value) })}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="1.0">1.0x (Normal)</SelectItem>
                    <SelectItem value="1.25">1.25x</SelectItem>
                    <SelectItem value="1.5">1.5x</SelectItem>
                    <SelectItem value="2.0">2.0x (Retina)</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Resource Limits */}
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base">Resource Limits</CardTitle>
            <CardDescription>
              Configure CPU, memory, and timeout limits for the session
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid grid-cols-3 gap-4">
              <div className="space-y-2">
                <Label htmlFor="cpu">CPU Cores</Label>
                <Input 
                  id="cpu"
                  type="number"
                  step="0.5"
                  min="0.5"
                  max="8"
                  placeholder="2.0"
                  value={session.resource_limits?.cpu || ''} 
                  onChange={(e) => updateResourceLimits({ 
                    cpu: e.target.value ? parseFloat(e.target.value) : undefined 
                  })}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="memory">Memory</Label>
                <Select 
                  value={session.resource_limits?.memory || ''} 
                  onValueChange={(value) => updateResourceLimits({ memory: value || undefined })}
                >
                  <SelectTrigger>
                    <SelectValue placeholder="Select memory" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="512MB">512 MB</SelectItem>
                    <SelectItem value="1GB">1 GB</SelectItem>
                    <SelectItem value="2GB">2 GB</SelectItem>
                    <SelectItem value="4GB">4 GB</SelectItem>
                    <SelectItem value="8GB">8 GB</SelectItem>
                    <SelectItem value="16GB">16 GB</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <Label htmlFor="timeout">Timeout (minutes)</Label>
                <Input 
                  id="timeout"
                  type="number"
                  min="5"
                  max="480"
                  placeholder="30"
                  value={session.resource_limits?.timeout_minutes || ''} 
                  onChange={(e) => updateResourceLimits({ 
                    timeout_minutes: e.target.value ? parseInt(e.target.value) : undefined 
                  })}
                />
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Advanced Options */}
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base">Advanced Options</CardTitle>
            <CardDescription>
              Additional configuration options for the session
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="flex items-center space-x-2">
              <Switch 
                id="webhooks"
                checked={session.webhooks_enabled || false} 
                onCheckedChange={(checked) => updateSession({ webhooks_enabled: checked })}
              />
              <Label htmlFor="webhooks">Enable webhooks</Label>
            </div>
            
            <div className="flex items-center space-x-2">
              <Switch 
                id="pooled"
                checked={session.is_pooled || false} 
                onCheckedChange={(checked) => updateSession({ is_pooled: checked })}
              />
              <Label htmlFor="pooled">Use session pooling</Label>
            </div>
          </CardContent>
        </Card>
      </div>
      
      {/* Action Buttons */}
      <div className="flex justify-end gap-2 pt-4 border-t">
        <Button variant="outline" onClick={onCancel} disabled={isLoading}>
          Cancel
        </Button>
        <Button onClick={onSubmit} disabled={isLoading}>
          {isLoading && <RefreshCw className="h-4 w-4 mr-2 animate-spin" />}
          Create Session
        </Button>
      </div>
    </ScrollArea>
  );
} 