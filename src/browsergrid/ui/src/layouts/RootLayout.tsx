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
      <header className="">
        <div className="flex h-16 items-center px-4 sm:px-6">
       

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
        

        {/* Main content */}
        <main className="flex-1">
          <Outlet />
        </main>
      </div>
    </div>
    </QueryClientProvider>
  )
} 