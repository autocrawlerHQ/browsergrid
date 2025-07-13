import React, { useState, useMemo } from 'react';
import { Plus, RefreshCw, Globe, ExternalLink, MoreVertical, Search, Filter, Eye, Download, Upload, Edit, Trash2 } from 'lucide-react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { 
  useGetApiV1Profiles, 
  usePostApiV1Profiles, 
  useDeleteApiV1ProfilesId, 
  usePatchApiV1ProfilesId,
  usePostApiV1ProfilesImport,
  useGetApiV1ProfilesIdExport
} from '@/lib/api/profiles/profiles';
import type { 
  InternalProfilesProfile, 
  InternalProfilesCreateProfileRequest, 
  InternalProfilesCreateProfileRequestBrowser,
  InternalProfilesStorageBackend,
  InternalProfilesUpdateProfileRequest
} from '@/lib/api/model';
import { toast } from 'sonner';
import { useNavigate } from 'react-router-dom';

export default function Profiles() {
  const { data: profilesData, isLoading, refetch } = useGetApiV1Profiles();
  const createProfile = usePostApiV1Profiles();
  const deleteProfile = useDeleteApiV1ProfilesId();
  const updateProfile = usePatchApiV1ProfilesId();
  const importProfile = usePostApiV1ProfilesImport();
  const navigate = useNavigate();
  
  // State management
  const [showCreateDialog, setShowCreateDialog] = useState(false);
  const [showEditDialog, setShowEditDialog] = useState(false);
  const [showImportDialog, setShowImportDialog] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const [browserFilter, setBrowserFilter] = useState<InternalProfilesCreateProfileRequestBrowser | 'all'>('all');
  const [editingProfile, setEditingProfile] = useState<InternalProfilesProfile | null>(null);
  
  const [newProfile, setNewProfile] = useState<InternalProfilesCreateProfileRequest>({
    name: '',
    description: '',
    browser: 'chrome',
  });

  const [importData, setImportData] = useState<{
    name: string;
    description: string;
    browser: InternalProfilesCreateProfileRequestBrowser;
    file: File | null;
  }>({
    name: '',
    description: '',
    browser: 'chrome',
    file: null,
  });

  // Filter and search profiles
  const filteredProfiles = useMemo(() => {
    if (!profilesData?.profiles) return [];
    
    return profilesData.profiles.filter(profile => {
      const matchesSearch = !searchQuery || 
        profile.name?.toLowerCase().includes(searchQuery.toLowerCase()) ||
        profile.description?.toLowerCase().includes(searchQuery.toLowerCase()) ||
        profile.browser?.toLowerCase().includes(searchQuery.toLowerCase());
      
      const matchesBrowser = browserFilter === 'all' || profile.browser === browserFilter;
      
      return matchesSearch && matchesBrowser;
    });
  }, [profilesData?.profiles, searchQuery, browserFilter]);

  const handleCreateProfile = async () => {
    try {
      await createProfile.mutateAsync({ data: newProfile });
      toast.success('Profile created successfully');
      setShowCreateDialog(false);
      refetch();
      // Reset form
      setNewProfile({
        name: '',
        description: '',
        browser: 'chrome',
      });
    } catch (error) {
      toast.error('Failed to create profile');
    }
  };

  const handleUpdateProfile = async () => {
    if (!editingProfile?.id) return;
    
    try {
      const updates: InternalProfilesUpdateProfileRequest = {};
      if (newProfile.name !== editingProfile.name) updates.name = newProfile.name;
      if (newProfile.description !== editingProfile.description) updates.description = newProfile.description;
      
      await updateProfile.mutateAsync({ 
        id: editingProfile.id, 
        data: updates 
      });
      toast.success('Profile updated successfully');
      setShowEditDialog(false);
      setEditingProfile(null);
      refetch();
    } catch (error) {
      toast.error('Failed to update profile');
    }
  };

  const handleDeleteProfile = async (profile: InternalProfilesProfile) => {
    if (!profile.id) return;
    
    if (confirm(`Are you sure you want to delete profile "${profile.name}"?`)) {
      try {
        await deleteProfile.mutateAsync({ id: profile.id });
        toast.success('Profile deleted successfully');
        refetch();
      } catch (error) {
        toast.error('Failed to delete profile');
      }
    }
  };

  const handleImportProfile = async () => {
    if (!importData.file) {
      toast.error('Please select a file to import');
      return;
    }
    
    try {
      await importProfile.mutateAsync({ 
        data: {
          name: importData.name,
          description: importData.description,
          browser: importData.browser,
          file: importData.file,
        }
      });
      toast.success('Profile imported successfully');
      setShowImportDialog(false);
      setImportData({ name: '', description: '', browser: 'chrome', file: null });
      refetch();
    } catch (error) {
      toast.error('Failed to import profile');
    }
  };

  const handleExportProfile = async (profile: InternalProfilesProfile) => {
    if (!profile.id) return;
    
    try {
      // Create a temporary anchor element to trigger download
      const response = await fetch(`/api/v1/profiles/${profile.id}/export`, {
        method: 'GET',
        headers: {
          'Authorization': 'Bearer 123',
        }
      });
      
      if (!response.ok) throw new Error('Export failed');
      
      const blob = await response.blob();
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `${profile.name}.zip`;
      document.body.appendChild(a);
      a.click();
      window.URL.revokeObjectURL(url);
      document.body.removeChild(a);
      
      toast.success('Profile exported successfully');
    } catch (error) {
      toast.error('Failed to export profile');
    }
  };

  const handleEditProfile = (profile: InternalProfilesProfile) => {
    setEditingProfile(profile);
    setNewProfile({
      name: profile.name || '',
      description: profile.description || '',
      browser: profile.browser || 'chrome',
    });
    setShowEditDialog(true);
  };

  // Stats calculation
  const stats = useMemo(() => {
    if (!profilesData?.profiles) return { total: 0, active: 0, chrome: 0, firefox: 0 };
    
    const profiles = profilesData.profiles;
    return {
      total: profiles.length,
      active: profiles.filter(p => (p.active_sessions || 0) > 0).length,
      chrome: profiles.filter(p => p.browser === 'chrome' || p.browser === 'chromium').length,
      firefox: profiles.filter(p => p.browser === 'firefox').length,
    };
  }, [profilesData?.profiles]);

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="flex items-center space-x-2">
          <RefreshCw className="h-4 w-4 animate-spin text-neutral-400" />
          <span className="text-sm text-neutral-600">Loading profiles...</span>
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
            Browser Profiles
          </h1>
          <p className="text-sm text-neutral-600 mt-1">
            Manage reusable browser profiles with saved state and configuration
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button onClick={() => refetch()} variant="outline" size="sm" disabled={isLoading} className="text-xs h-8">
            <RefreshCw className={`h-3 w-3 mr-1.5 ${isLoading ? 'animate-spin' : ''}`} />
            Refresh
          </Button>
          <Button onClick={() => setShowImportDialog(true)} variant="outline" size="sm" className="text-xs h-8">
            <Upload className="h-3 w-3 mr-1.5" />
            Import
          </Button>
          <Dialog open={showCreateDialog} onOpenChange={setShowCreateDialog}>
            <DialogTrigger asChild>
              <Button size="sm" className="bg-neutral-900 hover:bg-neutral-800 text-white text-xs h-8">
                <Plus className="h-3 w-3 mr-1.5" />
                New Profile
              </Button>
            </DialogTrigger>
            <DialogContent className="max-w-2xl border-neutral-200">
              <DialogHeader className="border-b border-neutral-100 pb-3">
                <DialogTitle className="text-lg font-semibold">Create New Profile</DialogTitle>
                <DialogDescription className="text-sm">
                  Create a new browser profile for saving and reusing browser state
                </DialogDescription>
              </DialogHeader>
              <ProfileCreateDialog 
                profile={newProfile} 
                setProfile={setNewProfile}
                onSubmit={handleCreateProfile}
                onCancel={() => setShowCreateDialog(false)}
                isLoading={createProfile.isPending}
              />
            </DialogContent>
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
          <span className="text-neutral-600">Chrome:</span>
          <span className="font-semibold text-neutral-900">{stats.chrome}</span>
        </div>
        <div className="flex items-center gap-2">
          <span className="text-neutral-600">Firefox:</span>
          <span className="font-semibold text-neutral-900">{stats.firefox}</span>
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
                  placeholder="Search profiles..."
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  className="pl-8 h-8 w-64 text-xs border-neutral-200"
                />
              </div>
              <div className="flex items-center gap-2">
                <Filter className="h-3 w-3 text-neutral-400" />
                <Select value={browserFilter} onValueChange={(value) => setBrowserFilter(value as InternalProfilesCreateProfileRequestBrowser | 'all')}>
                  <SelectTrigger className="w-28 h-8 text-xs border-neutral-200">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="all">All Browsers</SelectItem>
                    <SelectItem value="chrome">Chrome</SelectItem>
                    <SelectItem value="chromium">Chromium</SelectItem>
                    <SelectItem value="firefox">Firefox</SelectItem>
                    <SelectItem value="edge">Edge</SelectItem>
                    <SelectItem value="webkit">WebKit</SelectItem>
                    <SelectItem value="safari">Safari</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>
          </div>
        </CardHeader>
        <CardContent className="p-0">
          {filteredProfiles.length === 0 ? (
            <EmptyState 
              searchQuery={searchQuery}
              onCreateNew={() => setShowCreateDialog(true)}
            />
          ) : (
            <ProfilesTable 
              profiles={filteredProfiles}
              onEdit={handleEditProfile}
              onDelete={handleDeleteProfile}
              onExport={handleExportProfile}
            />
          )}
        </CardContent>
      </Card>

      {/* Edit Dialog */}
      <Dialog open={showEditDialog} onOpenChange={setShowEditDialog}>
        <DialogContent className="max-w-2xl border-neutral-200">
          <DialogHeader className="border-b border-neutral-100 pb-3">
            <DialogTitle className="text-lg font-semibold">Edit Profile</DialogTitle>
            <DialogDescription className="text-sm">
              Update profile information
            </DialogDescription>
          </DialogHeader>
          <ProfileCreateDialog 
            profile={newProfile} 
            setProfile={setNewProfile}
            onSubmit={handleUpdateProfile}
            onCancel={() => setShowEditDialog(false)}
            isLoading={updateProfile.isPending}
            isEdit={true}
          />
        </DialogContent>
      </Dialog>

      {/* Import Dialog */}
      <Dialog open={showImportDialog} onOpenChange={setShowImportDialog}>
        <DialogContent className="max-w-2xl border-neutral-200">
          <DialogHeader className="border-b border-neutral-100 pb-3">
            <DialogTitle className="text-lg font-semibold">Import Profile</DialogTitle>
            <DialogDescription className="text-sm">
              Import a browser profile from a ZIP file
            </DialogDescription>
          </DialogHeader>
          <ImportProfileDialog
            importData={importData}
            setImportData={setImportData}
            onSubmit={handleImportProfile}
            onCancel={() => setShowImportDialog(false)}
            isLoading={importProfile.isPending}
          />
        </DialogContent>
      </Dialog>
    </div>
  );
}

// Profile Create/Edit Dialog Component
function ProfileCreateDialog({ 
  profile, 
  setProfile, 
  onSubmit, 
  onCancel,
  isLoading,
  isEdit = false
}: {
  profile: InternalProfilesCreateProfileRequest;
  setProfile: (profile: InternalProfilesCreateProfileRequest) => void;
  onSubmit: () => void;
  onCancel: () => void;
  isLoading: boolean;
  isEdit?: boolean;
}) {
  return (
    <div className="space-y-4 p-4">
      <div className="space-y-2">
        <label htmlFor="name" className="text-sm font-medium text-neutral-700">
          Name *
        </label>
        <Input
          id="name"
          value={profile.name}
          onChange={(e) => setProfile({ ...profile, name: e.target.value })}
          placeholder="Enter profile name"
          className="text-sm h-9"
          maxLength={255}
        />
        {profile.name.length > 0 && profile.name.length < 3 && (
          <div className="text-xs text-red-500">Name must be at least 3 characters</div>
        )}
      </div>

      <div className="space-y-2">
        <label htmlFor="description" className="text-sm font-medium text-neutral-700">
          Description
        </label>
        <Textarea
          id="description"
          value={profile.description}
          onChange={(e) => setProfile({ ...profile, description: e.target.value })}
          placeholder="Enter profile description (optional)"
          className="text-sm min-h-[80px] resize-none"
          maxLength={1000}
        />
        <div className="text-xs text-neutral-500 text-right">
          {profile.description?.length || 0}/1000
        </div>
      </div>

      {!isEdit && (
        <div className="space-y-2">
          <label htmlFor="browser" className="text-sm font-medium text-neutral-700">
            Browser Type *
          </label>
                  <Select value={profile.browser} onValueChange={(value) => setProfile({ ...profile, browser: value as InternalProfilesCreateProfileRequestBrowser })}>
          <SelectTrigger className="h-9 text-sm">
            <SelectValue />
          </SelectTrigger>
            <SelectContent>
              <SelectItem value="chrome">Chrome</SelectItem>
              <SelectItem value="chromium">Chromium</SelectItem>
              <SelectItem value="firefox">Firefox</SelectItem>
              <SelectItem value="edge">Edge</SelectItem>
              <SelectItem value="webkit">WebKit</SelectItem>
              <SelectItem value="safari">Safari</SelectItem>
            </SelectContent>
          </Select>
        </div>
      )}

      <div className="flex justify-end gap-2 pt-4">
        <Button variant="outline" size="sm" onClick={onCancel} disabled={isLoading}>
          Cancel
        </Button>
        <Button 
          size="sm" 
          onClick={onSubmit} 
          disabled={isLoading || !profile.name.trim() || profile.name.length < 3}
          className="bg-neutral-900 hover:bg-neutral-800 text-white"
        >
          {isLoading ? 'Saving...' : (isEdit ? 'Update' : 'Create')} Profile
        </Button>
      </div>
    </div>
  );
}

// Import Profile Dialog Component
function ImportProfileDialog({
  importData,
  setImportData,
  onSubmit,
  onCancel,
  isLoading
}: {
  importData: { name: string; description: string; browser: InternalProfilesCreateProfileRequestBrowser; file: File | null };
  setImportData: (data: { name: string; description: string; browser: InternalProfilesCreateProfileRequestBrowser; file: File | null }) => void;
  onSubmit: () => void;
  onCancel: () => void;
  isLoading: boolean;
}) {
  return (
    <div className="space-y-4 p-4">
      <div className="space-y-2">
        <label htmlFor="file" className="text-sm font-medium text-neutral-700">
          Profile ZIP File *
        </label>
        <Input
          id="file"
          type="file"
          accept=".zip"
          onChange={(e) => setImportData({ ...importData, file: e.target.files?.[0] || null })}
          className="text-sm h-9"
        />
        {importData.file && (
          <div className="text-xs text-green-600 flex items-center gap-1">
            <span>✓</span>
            {importData.file.name} ({(importData.file.size / 1024 / 1024).toFixed(2)} MB)
          </div>
        )}
      </div>

      <div className="space-y-2">
        <label htmlFor="import-name" className="text-sm font-medium text-neutral-700">
          Profile Name *
        </label>
        <Input
          id="import-name"
          value={importData.name}
          onChange={(e) => setImportData({ ...importData, name: e.target.value })}
          placeholder="Enter profile name"
          className="text-sm h-9"
          maxLength={255}
        />
        {importData.name.length > 0 && importData.name.length < 3 && (
          <div className="text-xs text-red-500">Name must be at least 3 characters</div>
        )}
      </div>

      <div className="space-y-2">
        <label htmlFor="import-description" className="text-sm font-medium text-neutral-700">
          Description
        </label>
        <Textarea
          id="import-description"
          value={importData.description}
          onChange={(e) => setImportData({ ...importData, description: e.target.value })}
          placeholder="Enter profile description (optional)"
          className="text-sm min-h-[80px] resize-none"
          maxLength={1000}
        />
        <div className="text-xs text-neutral-500 text-right">
          {importData.description?.length || 0}/1000
        </div>
      </div>

      <div className="space-y-2">
        <label htmlFor="import-browser" className="text-sm font-medium text-neutral-700">
          Browser Type *
        </label>
        <Select value={importData.browser} onValueChange={(value) => setImportData({ ...importData, browser: value as InternalProfilesCreateProfileRequestBrowser })}>
          <SelectTrigger className="h-9 text-sm">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="chrome">Chrome</SelectItem>
            <SelectItem value="chromium">Chromium</SelectItem>
            <SelectItem value="firefox">Firefox</SelectItem>
            <SelectItem value="edge">Edge</SelectItem>
            <SelectItem value="webkit">WebKit</SelectItem>
            <SelectItem value="safari">Safari</SelectItem>
          </SelectContent>
        </Select>
      </div>

      <div className="flex justify-end gap-2 pt-4">
        <Button variant="outline" size="sm" onClick={onCancel} disabled={isLoading}>
          Cancel
        </Button>
        <Button 
          size="sm" 
          onClick={onSubmit} 
          disabled={isLoading || !importData.name.trim() || importData.name.length < 3 || !importData.file}
          className="bg-neutral-900 hover:bg-neutral-800 text-white"
        >
          {isLoading ? 'Importing...' : 'Import Profile'}
        </Button>
      </div>
    </div>
  );
}

// Profiles Table Component
function ProfilesTable({ 
  profiles, 
  onEdit,
  onDelete,
  onExport
}: { 
  profiles: InternalProfilesProfile[];
  onEdit: (profile: InternalProfilesProfile) => void;
  onDelete: (profile: InternalProfilesProfile) => void;
  onExport: (profile: InternalProfilesProfile) => void;
}) {
  const formatBytes = (bytes: number) => {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  };

  return (
    <Table>
      <TableHeader>
        <TableRow className="border-neutral-100">
          <TableHead className="font-medium text-neutral-700 text-xs h-10">Profile</TableHead>
          <TableHead className="font-medium text-neutral-700 text-xs h-10">Browser</TableHead>
          <TableHead className="font-medium text-neutral-700 text-xs h-10">Size</TableHead>
          <TableHead className="font-medium text-neutral-700 text-xs h-10">Sessions</TableHead>
          <TableHead className="font-medium text-neutral-700 text-xs h-10">Created</TableHead>
          <TableHead className="font-medium text-neutral-700 text-xs h-10">Last Used</TableHead>
          <TableHead className="text-right font-medium text-neutral-700 text-xs h-10">Actions</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {profiles.map((profile) => (
          <TableRow key={profile.id} className="border-neutral-100 hover:bg-neutral-50/50 transition-colors duration-150">
            <TableCell className="py-3">
              <div className="space-y-0.5">
                <div className="font-medium text-neutral-900 text-sm">
                  {profile.name}
                </div>
                <div className="text-xs text-neutral-500">
                  {profile.description || 'No description'}
                </div>
              </div>
            </TableCell>
            <TableCell className="py-3">
              <div className="flex items-center gap-2">
                <Globe className="h-3 w-3 text-neutral-400" />
                <span className="font-medium text-neutral-900 text-xs capitalize">{profile.browser}</span>
                <Badge variant="outline" className="text-xs border-neutral-200 text-neutral-600 px-1.5 py-0">
                  {profile.storage_backend}
                </Badge>
              </div>
            </TableCell>
            <TableCell className="py-3">
              <span className="text-xs text-neutral-900">
                {formatBytes(profile.size_bytes || 0)}
              </span>
            </TableCell>
            <TableCell className="py-3">
              <div className="flex items-center gap-2">
                <span className="text-xs text-neutral-900">
                  {profile.active_sessions || 0} active
                </span>
                <span className="text-xs text-neutral-500">
                  / {profile.total_sessions || 0} total
                </span>
              </div>
            </TableCell>
            <TableCell className="py-3">
              <div className="space-y-0.5">
                <div className="text-xs text-neutral-900">
                  {profile.created_at ? new Date(profile.created_at).toLocaleDateString() : 'N/A'}
                </div>
                <div className="text-xs text-neutral-500">
                  {profile.created_at ? new Date(profile.created_at).toLocaleTimeString() : ''}
                </div>
              </div>
            </TableCell>
            <TableCell className="py-3">
              <div className="text-xs text-neutral-900">
                {profile.last_used_at ? new Date(profile.last_used_at).toLocaleDateString() : 'Never'}
              </div>
            </TableCell>
            <TableCell className="text-right py-3">
              <div className="flex items-center justify-end gap-1">
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => onEdit(profile)}
                  className="h-7 w-7 p-0 hover:bg-neutral-100"
                >
                  <Edit className="h-3 w-3" />
                </Button>
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => onExport(profile)}
                  className="h-7 w-7 p-0 hover:bg-neutral-100"
                >
                  <Download className="h-3 w-3" />
                </Button>
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => onDelete(profile)}
                  className="h-7 w-7 p-0 hover:bg-neutral-100 text-red-600 hover:text-red-700"
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
      <Globe className="h-8 w-8 mx-auto text-neutral-400 mb-3" />
      {searchQuery ? (
        <>
          <h3 className="text-sm font-semibold text-neutral-900 mb-1">No profiles found</h3>
          <p className="text-xs text-neutral-600 mb-4">
            No profiles match your search criteria. Try adjusting your filters.
          </p>
        </>
      ) : (
        <>
          <h3 className="text-sm font-semibold text-neutral-900 mb-1">No profiles yet</h3>
          <p className="text-xs text-neutral-600 mb-4">
            Get started by creating your first browser profile to save and reuse browser state.
          </p>
        </>
      )}
      <Button onClick={onCreateNew} size="sm" className="bg-neutral-900 hover:bg-neutral-800 text-white text-xs h-8">
        <Plus className="h-3 w-3 mr-1.5" />
        Create New Profile
      </Button>
    </div>
  );
} 