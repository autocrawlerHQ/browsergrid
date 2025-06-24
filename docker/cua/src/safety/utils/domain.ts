export function extractDomain(url: string): string {
    try {
      const parsedUrl = new URL(url);
      return parsedUrl.hostname;
    } catch (error) {
      return url;
    }
}
  
export function matchesDomain(url: string, pattern: string): boolean {
    const domain = extractDomain(url);
    
    if (domain === pattern) {
      return true;
    }
    
    if (domain.endsWith(`.${pattern}`)) {
      return true;
    }
    
    if (domain.includes(pattern)) {
      return true;
    }
    
    return false;
}
  
export function matchesAnyDomain(url: string, patterns: string[]): boolean {
    return patterns.some(pattern => matchesDomain(url, pattern));
}