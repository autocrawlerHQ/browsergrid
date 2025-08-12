import { useState, useMemo } from 'react';
import { Plus, RefreshCw, Globe, ExternalLink, MoreVertical, Search, Filter, Eye, User, StopCircle } from 'lucide-react';
import { Card, CardContent, CardHeader } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { StatusBadge } from '@/components/dashboard/StatusBadge';
import { SessionForm } from '@/components/SessionForm';
import { useGetApiV1Sessions, usePostApiV1Sessions, useDeleteApiV1SessionsId } from '@/lib/api/sessions/sessions';
import { useGetApiV1Profiles } from '@/lib/api/profiles/profiles';
import { processVncUrl } from '@/lib/utils';
import type { Session, Browser, BrowserVersion, OperatingSystem, SessionStatus } from '@/lib/api/model';
import { toast } from 'sonner';
import { useNavigate } from 'react-router-dom';

export default function Sessions() {
  const { data: sessionsData, isLoading, refetch } = useGetApiV1Sessions();
  const { data: profilesData } = useGetApiV1Profiles();
  const createSession = usePostApiV1Sessions();
  const stopSession = useDeleteApiV1SessionsId();
  const navigate = useNavigate();
  
  // State management
  const [showCreateDialog, setShowCreateDialog] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const [statusFilter, setStatusFilter] = useState<SessionStatus | 'all'>('all');
  const [browserFilter, setBrowserFilter] = useState<Browser | 'all'>('all');
  const [stopLoadingId, setStopLoadingId] = useState<string | null>(null);
  
  const [newSession, setNewSession] = useState<Partial<Session>>({
    browser: 'chrome' as Browser,
    version: 'latest' as BrowserVersion,
    operating_system: 'linux' as OperatingSystem,
    headless: true,
    screen: {
      width: 1920,
      height: 1080,
      dpi: 96,
      scale: 1.0,
    },
  });

  // Filter and search sessions
  const filteredSessions = useMemo(() => {
    if (!sessionsData?.sessions) return [];
    
    return sessionsData.sessions.filter(session => {
      const matchesSearch = !searchQuery || 
        session.id?.toLowerCase().includes(searchQuery.toLowerCase()) ||
        session.browser?.toLowerCase().includes(searchQuery.toLowerCase()) ||
        session.status?.toLowerCase().includes(searchQuery.toLowerCase());
      
      const matchesStatus = statusFilter === 'all' || session.status === statusFilter;
      const matchesBrowser = browserFilter === 'all' || session.browser === browserFilter;
      
      return matchesSearch && matchesStatus && matchesBrowser;
    });
  }, [sessionsData?.sessions, searchQuery, statusFilter, browserFilter]);

  const handleCreateSession = async () => {
    try {
      await createSession.mutateAsync({ data: newSession as Session });
      toast.success('Session created successfully');
      setShowCreateDialog(false);
      refetch();
      // Reset form
      setNewSession({
        browser: 'chrome' as Browser,
        version: 'latest' as BrowserVersion,
        operating_system: 'linux' as OperatingSystem,
        headless: true,
        screen: {
          width: 1920,
          height: 1080,
          dpi: 96,
          scale: 1.0,
        },
      });
    } catch (error) {
      toast.error('Failed to create session');
    }
  };

  const handleViewDetails = (session: Session) => {
    navigate(`/sessions/${session.id}`);
  };

  const handleStopSession = async (session: Session) => {
    if (!session.id) return;
    try {
      setStopLoadingId(session.id);
      await stopSession.mutateAsync({ id: session.id });
      toast.success('Session termination initiated');
      await refetch();
    } catch (error) {
      toast.error('Failed to stop session');
    } finally {
      setStopLoadingId(null);
    }
  };

  // Stats calculation
  const stats = useMemo(() => {
    if (!sessionsData?.sessions) return { total: 0, running: 0, available: 0, failed: 0 };
    
    const sessions = sessionsData.sessions;
    return {
      total: sessions.length,
      running: sessions.filter(s => s.status === 'running' || s.status === 'claimed').length,
      available: sessions.filter(s => s.status === 'available').length,
      failed: sessions.filter(s => s.status === 'failed' || s.status === 'crashed').length,
    };
  }, [sessionsData?.sessions]);

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="flex items-center space-x-2">
          <RefreshCw className="h-4 w-4 animate-spin text-neutral-400" />
          <span className="text-sm text-neutral-600">Loading sessions...</span>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6 ">
      {/* Header */}
      <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-neutral-900 tracking-tight">
            Browser Sessions
          </h1>
          <p className="text-sm text-neutral-600 mt-1">
            Manage and monitor your browser automation sessions
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
                New Session
              </Button>
            </DialogTrigger>
            <SessionCreateDialog 
              newSession={newSession} 
              setNewSession={setNewSession}
              onSubmit={handleCreateSession}
              isLoading={createSession.isPending}
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
          <span className="text-neutral-600">Running:</span>
          <span className="font-semibold text-neutral-900">{stats.running}</span>
        </div>
        <div className="flex items-center gap-2">
          <span className="text-neutral-600">Available:</span>
          <span className="font-semibold text-neutral-900">{stats.available}</span>
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
                  placeholder="Search sessions..."
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  className="pl-8 h-8 w-64 text-xs border-neutral-200"
                />
              </div>
              <div className="flex items-center gap-2">
                <Filter className="h-3 w-3 text-neutral-400" />
                <Select value={statusFilter} onValueChange={(value) => setStatusFilter(value as SessionStatus | 'all')}>
                  <SelectTrigger className="w-28 h-8 text-xs border-neutral-200">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="all">All Status</SelectItem>
                    <SelectItem value="pending">Pending</SelectItem>
                    <SelectItem value="starting">Starting</SelectItem>
                    <SelectItem value="available">Available</SelectItem>
                    <SelectItem value="claimed">Claimed</SelectItem>
                    <SelectItem value="running">Running</SelectItem>
                    <SelectItem value="idle">Idle</SelectItem>
                    <SelectItem value="completed">Completed</SelectItem>
                    <SelectItem value="failed">Failed</SelectItem>
                    <SelectItem value="expired">Expired</SelectItem>
                    <SelectItem value="crashed">Crashed</SelectItem>
                    <SelectItem value="terminated">Terminated</SelectItem>
                  </SelectContent>
                </Select>
                <Select value={browserFilter} onValueChange={(value) => setBrowserFilter(value as Browser | 'all')}>
                  <SelectTrigger className="w-28 h-8 text-xs border-neutral-200">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="all">All Browsers</SelectItem>
                    <SelectItem value="chrome">Chrome</SelectItem>
                    <SelectItem value="firefox">Firefox</SelectItem>
                    <SelectItem value="edge">Edge</SelectItem>
                    <SelectItem value="safari">Safari</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>
          </div>
        </CardHeader>
        <CardContent className="p-0">
          {filteredSessions.length === 0 ? (
            <EmptyState 
              searchQuery={searchQuery}
              onCreateNew={() => setShowCreateDialog(true)}
            />
          ) : (
            <SessionsTable 
              sessions={filteredSessions}
              profiles={profilesData?.profiles || []}
              onViewDetails={handleViewDetails}
              onStopSession={handleStopSession}
              stopLoadingId={stopLoadingId}
            />
          )}
        </CardContent>
      </Card>
    </div>
  );
}

// Session Create Dialog Component
function SessionCreateDialog({ 
  newSession, 
  setNewSession, 
  onSubmit, 
  isLoading 
}: {
  newSession: Partial<Session>;
  setNewSession: (session: Partial<Session>) => void;
  onSubmit: () => void;
  isLoading: boolean;
}) {
  return (
    <DialogContent className="max-w-3xl max-h-[90vh] border-neutral-200">
      <DialogHeader className="border-b border-neutral-100 pb-3">
        <DialogTitle className="text-lg font-semibold">Create New Session</DialogTitle>
        <DialogDescription className="text-sm">
          Configure your browser session settings
        </DialogDescription>
      </DialogHeader>
      <SessionForm
        session={newSession}
        onSessionChange={setNewSession}
        onSubmit={onSubmit}
        onCancel={() => {}}
        isLoading={isLoading}
      />
    </DialogContent>
  );
}

// Sessions Table Component
function SessionsTable({ 
  sessions, 
  profiles,
  onViewDetails,
  onStopSession,
  stopLoadingId,
}: { 
  sessions: Session[];
  profiles: any[];
  onViewDetails: (session: Session) => void;
  onStopSession: (session: Session) => void;
  stopLoadingId: string | null;
}) {
  const isTerminalStatus = (status?: SessionStatus) => {
    const terminal: SessionStatus[] = ['completed', 'failed', 'expired', 'crashed', 'timed_out', 'terminated'];
    return !!status && terminal.includes(status);
  };
  return (
    <Table>
      <TableHeader>
        <TableRow className="border-neutral-100">
          <TableHead className="font-medium text-neutral-700 text-xs h-10">Session</TableHead>
          <TableHead className="font-medium text-neutral-700 text-xs h-10">Browser</TableHead>
          <TableHead className="font-medium text-neutral-700 text-xs h-10">Profile</TableHead>
          <TableHead className="font-medium text-neutral-700 text-xs h-10">Status</TableHead>
          <TableHead className="font-medium text-neutral-700 text-xs h-10">Created</TableHead>
          <TableHead className="font-medium text-neutral-700 text-xs h-10">Pool</TableHead>
          <TableHead className="text-right font-medium text-neutral-700 text-xs h-10">Actions</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {sessions.map((session) => (
          <TableRow key={session.id} className="border-neutral-100 hover:bg-neutral-50/50 transition-colors duration-150">
            <TableCell className="py-3">
              <div className="space-y-0.5">
                <div className="font-mono text-xs font-medium text-neutral-900">
                  {session.id?.substring(0, 8)}...
                </div>
                <div className="text-xs text-neutral-500">
                  {session.operating_system} • {session.headless ? 'Headless' : 'GUI'}
                </div>
              </div>
            </TableCell>
            <TableCell className="py-3">
              <div className="flex items-center gap-2">
                <Globe className="h-3 w-3 text-neutral-400" />
                <span className="font-medium text-neutral-900 text-xs">{session.browser}</span>
                <Badge variant="outline" className="text-xs border-neutral-200 text-neutral-600 px-1.5 py-0">
                  {session.version}
                </Badge>
              </div>
            </TableCell>
            <TableCell className="py-3">
              {session.profile_id ? (
                <div className="flex items-center gap-2">
                  <User className="h-3 w-3 text-neutral-400" />
                  <span className="font-medium text-neutral-900 text-xs">
                    {profiles.find(p => p.id === session.profile_id)?.name || 'Unknown Profile'}
                  </span>
                </div>
              ) : (
                <span className="text-neutral-400 text-xs">-</span>
              )}
            </TableCell>
            <TableCell className="py-3">
              <StatusBadge status={session.status || 'unknown'} />
            </TableCell>
            <TableCell className="py-3">
              <div className="space-y-0.5">
                <div className="text-xs text-neutral-900">
                  {session.created_at ? new Date(session.created_at).toLocaleDateString() : 'N/A'}
                </div>
                <div className="text-xs text-neutral-500">
                  {session.created_at ? new Date(session.created_at).toLocaleTimeString() : ''}
                </div>
              </div>
            </TableCell>
            <TableCell className="py-3">
              {session.work_pool_id ? (
                <Badge variant="outline" className="text-xs border-neutral-200 text-neutral-600 px-1.5 py-0">
                  {session.work_pool_id?.substring(0, 8)}...
                </Badge>
              ) : (
                <span className="text-neutral-400 text-xs">-</span>
              )}
            </TableCell>
            <TableCell className="text-right py-3">
              <div className="flex items-center justify-end gap-1">
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => onViewDetails(session)}
                  className="h-7 w-7 p-0 hover:bg-neutral-100"
                >
                  <Eye className="h-3 w-3" />
                </Button>
                {session.live_url && (
                  <Button size="sm" variant="ghost" asChild className="h-7 w-7 p-0 hover:bg-neutral-100">
                    <a href={processVncUrl(session.live_url)} target="_blank" rel="noopener noreferrer">
                      <ExternalLink className="h-3 w-3" />
                    </a>
                  </Button>
                )}
                {!isTerminalStatus(session.status) && (
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() => onStopSession(session)}
                    disabled={stopLoadingId === session.id}
                    className="h-7 w-7 p-0 text-red-600 hover:bg-red-50"
                    title="Stop session"
                  >
                    {stopLoadingId === session.id ? (
                      <RefreshCw className="h-3 w-3 animate-spin" />
                    ) : (
                      <StopCircle className="h-3 w-3" />
                    )}
                  </Button>
                )}
                <Button size="sm" variant="ghost" className="h-7 w-7 p-0 hover:bg-neutral-100">
                  <MoreVertical className="h-3 w-3" />
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
      <Globe className="h-8 w-8 mx-auto text-neutral-400 mb-3" />
      {searchQuery ? (
        <>
          <h3 className="text-sm font-semibold text-neutral-900 mb-1">No sessions found</h3>
          <p className="text-xs text-neutral-600 mb-4">
            No sessions match your search criteria. Try adjusting your filters.
          </p>
        </>
      ) : (
        <>
          <h3 className="text-sm font-semibold text-neutral-900 mb-1">No sessions yet</h3>
          <p className="text-xs text-neutral-600 mb-4">
            Get started by creating your first browser session.
          </p>
        </>
      )}
      <Button onClick={onCreateNew} size="sm" className="bg-neutral-900 hover:bg-neutral-800 text-white text-xs h-8">
        <Plus className="h-3 w-3 mr-1.5" />
        Create New Session
      </Button>
    </div>
  );
} 