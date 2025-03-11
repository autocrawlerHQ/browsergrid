import { Outlet } from 'react-router-dom'
import { useTheme } from '../components/theme-provider'
import { Button } from '../components/ui/button'
import { Moon, Sun, Menu, X } from 'lucide-react'
import { useState } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { cn } from '../lib/utils'
import { Separator } from '../components/ui/separator'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

// Navigation items
const navItems = [
  { name: 'Dashboard', path: '/' },
  { name: 'Sessions', path: '/sessions' },
  { name: 'Webhooks', path: '/webhooks' },
  { name: 'Work Pools', path: '/workpools' },
  { name: 'Workers', path: '/workers' },
]

const queryClient = new QueryClient()
export default function RootLayout() {
  const { theme, setTheme } = useTheme()
  const [sidebarOpen, setSidebarOpen] = useState(false)
  const location = useLocation()

  return (
    <QueryClientProvider client={queryClient}>

    <div className="flex min-h-screen flex-col">
      {/* Header */}
      <header className="sticky top-0 z-10 border-b bg-background">
        <div className="flex h-16 items-center px-4 sm:px-6">
          <Button 
            variant="ghost" 
            size="icon" 
            className="md:hidden" 
            onClick={() => setSidebarOpen(!sidebarOpen)}
          >
            {sidebarOpen ? <X size={20} /> : <Menu size={20} />}
            <span className="sr-only">Toggle menu</span>
          </Button>
          
          <div className="flex items-center gap-2 font-semibold">
            <span className="flex items-center gap-2 text-xl">
              <span className="hidden md:inline">BrowserGrid</span>
            </span>
          </div>

          <div className="ml-auto flex items-center gap-2">
            <Button 
              variant="ghost" 
              size="icon"
              onClick={() => setTheme(theme === 'light' ? 'dark' : 'light')}
            >
              {theme === 'light' ? <Moon size={20} /> : <Sun size={20} />}
              <span className="sr-only">Toggle theme</span>
            </Button>
          </div>
        </div>
      </header>

      <div className="flex flex-1">
        {/* Sidebar for mobile */}
        <div
          className={cn(
            "fixed inset-0 z-50 transform md:hidden",
            sidebarOpen ? "translate-x-0" : "-translate-x-full",
            "transition-transform duration-300 ease-in-out"
          )}
        >
          <div className="flex h-full w-64 flex-col bg-background">
            <div className="flex h-16 items-center px-4 border-b">
              <Button 
                variant="ghost" 
                size="icon" 
                onClick={() => setSidebarOpen(false)}
              >
                <X size={20} />
                <span className="sr-only">Close sidebar</span>
              </Button>
              <span className="ml-2 text-lg font-semibold">BrowserGrid</span>
            </div>
            <nav className="flex-1 overflow-y-auto p-4">
              <ul className="space-y-2">
                {navItems.map((item) => (
                  <li key={item.path}>
                    <Link
                      to={item.path}
                      className={cn(
                        "flex items-center rounded-md px-3 py-2 text-sm font-medium",
                        location.pathname === item.path ||
                        (item.path !== '/' && location.pathname.startsWith(item.path))
                          ? "bg-primary text-primary-foreground"
                          : "text-foreground hover:bg-accent hover:text-accent-foreground"
                      )}
                      onClick={() => setSidebarOpen(false)}
                    >
                      {item.name}
                    </Link>
                  </li>
                ))}
              </ul>
            </nav>
          </div>
          <div 
            className="opacity-50 fixed inset-0 bg-background/80 backdrop-blur-sm"
            onClick={() => setSidebarOpen(false)}
          ></div>
        </div>

        {/* Sidebar for desktop */}
        <div className="hidden md:flex md:w-64 md:flex-col md:border-r">
          <nav className="flex-1 overflow-y-auto p-4">
            <ul className="space-y-2">
              {navItems.map((item) => (
                <li key={item.path}>
                  <Link
                    to={item.path}
                    className={cn(
                      "flex items-center rounded-md px-3 py-2 text-sm font-medium",
                      location.pathname === item.path ||
                      (item.path !== '/' && location.pathname.startsWith(item.path))
                        ? "bg-primary text-primary-foreground"
                        : "text-foreground hover:bg-accent hover:text-accent-foreground"
                    )}
                  >
                    {item.name}
                  </Link>
                </li>
              ))}
            </ul>
          </nav>
        </div>

        {/* Main content */}
        <main className="flex-1 overflow-y-auto p-4 sm:p-6">
          <Outlet />
        </main>
      </div>
    </div>
    </QueryClientProvider>
  )
} 