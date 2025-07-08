import { Outlet } from 'react-router-dom'
import { useTheme } from '../components/theme-provider'
import { Button } from '../components/ui/button'
import { 
  Moon, 
  Sun, 
  ChevronLeft, 
  PlayCircle, 
  Plus, 
  Settings, 
  LayoutGrid, 
  Globe, 
  Layers, 
  ListTodo, 
  Webhook 
} from 'lucide-react'
import { Link, useLocation } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

// Navigation items for the main tabs
const tabItems = [
  { id: 'overview', label: 'Overview', icon: LayoutGrid, path: '/' },
  { id: 'sessions', label: 'Browser Sessions', icon: Globe, path: '/sessions' },
  { id: 'workpools', label: 'Work Pools', icon: Layers, path: '/workpools' },
  { id: 'queue', label: 'Queue', icon: ListTodo, path: '/queue' },
  { id: 'webhooks', label: 'Webhooks', icon: Webhook, path: '/webhooks' }
]

const queryClient = new QueryClient()

export default function RootLayout() {
  const { theme, setTheme } = useTheme()
  const location = useLocation()

  return (
    <QueryClientProvider client={queryClient}>
      <div className="bg-background flex flex-grow flex-col">
        <div className="flex w-full flex-grow flex-col">
          <div className="before:bg-secondary/40 before:border-muted relative flex h-full w-full flex-col items-center justify-center p-8 before:absolute before:top-[293px] before:left-0 before:z-0 before:h-[calc(100%-293px)] before:w-full before:border-t before:border-b before:dark:bg-black">
            <div className="z-10 flex min-h-screen h-full w-full max-w-7xl flex-col items-stretch justify-start">
              
              {/* Header */}
              <header className="mb-6 flex items-start justify-between">
                <div>
                  <Link to="/" className="text-primary/70 mb-4 flex items-center gap-2 text-sm hover:underline">
                    <ChevronLeft className="h-3 w-3" /> Back to Home
                  </Link>
                  <h1 className="mb-2 text-4xl font-bold">Browsergrid</h1>
                  <p className="text-primary/70 mb-6 text-sm">
                    Browser infrastructure for automation, testing, and development
                  </p>
                  
                  <div className="mb-6 flex space-x-2">
                    <Button className="bg-blue-600 text-white hover:bg-blue-700">
                      <PlayCircle className="mr-2 h-4 w-4" />
                      Launch Browser
                    </Button>
                    <Button variant="outline">
                      <Plus className="mr-2 h-4 w-4" />
                      New Work Pool
                    </Button>
                    <Button variant="outline">
                      <Settings className="mr-2 h-4 w-4" />
                      Settings
                    </Button>
                  </div>
                </div>
                
                <div className="flex items-center gap-2">
                  <Button 
                    variant="ghost" 
                    size="icon"
                    onClick={() => setTheme(theme === 'light' ? 'dark' : 'light')}
                  >
                    {theme === 'light' ? <Moon size={20} /> : <Sun size={20} />}
                    <span className="sr-only">Toggle theme</span>
                  </Button>
                </div>
              </header>
              
              {/* Navigation Tabs */}
              <div className="mb-6">
                <div className="flex space-x-6">
                  {tabItems.map((tab) => (
                    <Link
                      key={tab.id}
                      to={tab.path}
                      className={`flex items-center gap-2 border-b-2 px-1 py-3 text-sm font-medium transition-all ${
                        location.pathname === tab.path
                          ? "border-blue-600 text-blue-600 dark:border-blue-400 dark:text-blue-400" 
                          : "border-transparent text-gray-500 hover:border-gray-300 hover:text-gray-700 dark:text-gray-400 dark:hover:border-gray-600 dark:hover:text-gray-300"
                      }`}
                    >
                      <tab.icon className="h-4 w-4" />
                      {tab.label}
                    </Link>
                  ))}
                </div>
              </div>
              
              {/* Main content */}
              <div className="flex-1">
                <Outlet />
              </div>
            </div>
          </div>
        </div>
      </div>
    </QueryClientProvider>
  )
}