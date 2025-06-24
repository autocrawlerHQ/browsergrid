import { type ClassValue, clsx } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

/**
 * Processes a VNC URL by replacing host.docker.internal with localhost
 * and appending the VNC viewer path and parameters
 */
export function processVncUrl(liveUrl?: string): string | undefined {
  if (!liveUrl) return undefined;
  
  // Replace host.docker.internal with localhost for local development
  let processedUrl = liveUrl.replace('host.docker.internal', 'localhost');
  
  // Append VNC viewer path and parameters if not already present
  if (!processedUrl.includes('/vnc/')) {
    processedUrl += '/vnc/vnc_lite.html?autoconnect=true&resize=scale';
  }
  
  return processedUrl;
}
