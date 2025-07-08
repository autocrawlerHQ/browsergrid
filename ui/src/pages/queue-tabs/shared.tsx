import { RefreshCw } from 'lucide-react';
import { Card, CardContent } from '@/components/ui/card';

// API configuration
export const ASYNQMON_API = 'http://localhost:4444/api';
export const POLLING_INTERVAL = 5000; // 5 seconds

// Custom fetch wrapper for Asynqmon API
export const fetchAsynqmon = async (endpoint: string) => {
    const response = await fetch(`${ASYNQMON_API}${endpoint}`);
    if (!response.ok) throw new Error('Failed to fetch');
    return response.json();
};

// Helper Components
export function LoadingState({ message }: { message: string }) {
    return (
        <div className="flex items-center justify-center h-64">
            <div className="flex items-center space-x-2">
                <RefreshCw className="h-4 w-4 animate-spin text-muted-foreground" />
                <span className="text-sm text-muted-foreground">{message}</span>
            </div>
        </div>
    );
}

export function EmptyState({
    icon: Icon,
    title,
    description
}: {
    icon: any;
    title: string;
    description?: string;
}) {
    return (
        <Card>
            <CardContent className="flex flex-col items-center justify-center h-64">
                <Icon className="h-8 w-8 text-muted-foreground mb-3" />
                <h3 className="text-sm font-medium text-foreground">{title}</h3>
                {description && (
                    <p className="text-xs text-muted-foreground mt-1">{description}</p>
                )}
            </CardContent>
        </Card>
    );
}

// Helper Functions
export function formatBytes(bytes: number) {
    if (!bytes || bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    const formattedValue = parseFloat((bytes / Math.pow(k, i)).toFixed(2));
    return `${formattedValue} ${sizes[i]}`;
}

export function formatDuration(seconds: number) {
    if (seconds < 60) return `${seconds}s`;
    if (seconds < 3600) return `${Math.round(seconds / 60)}m`;
    if (seconds < 86400) return `${Math.round(seconds / 3600)}h`;
    return `${Math.round(seconds / 86400)}d`;
} 