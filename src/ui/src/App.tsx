import { RouterProvider, createBrowserRouter } from 'react-router-dom'
import { Toaster } from 'sonner'
import { ThemeProvider } from '@/components/theme-provider'
import RootLayout from '@/layouts/RootLayout'
import Dashboard from '@/pages/Dashboard'
import Sessions from '@/pages/sessions/Sessions'
import SessionDetails from '@/pages/sessions/SessionDetails'
import CreateSession from '@/pages/sessions/CreateSession'
import Webhooks from '@/pages/webhooks/Webhooks'
// import WebhookDetails from '@/pages/webhooks/WebhookDetails'
// import CreateWebhook from '@/pages/webhooks/CreateWebhook'
// import WorkPools from '@/pages/workpools/WorkPools'
// import WorkPoolDetails from '@/pages/workpools/WorkPoolDetails'
// import Workers from '@/pages/workpools/Workers'
// import WorkerDetails from '@/pages/workpools/WorkerDetails'
import NotFound from '@/pages/NotFound'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'


// Create router configuration
const router = createBrowserRouter([
  {
    path: '/',
    element: <RootLayout />,
    children: [
      { index: true, element: <Dashboard /> },
      // { path: 'sessions', element: <Sessions /> },
      // { path: 'sessions/new', element: <CreateSession /> },
      // { path: 'sessions/:id', element: <SessionDetails /> },
      // { path: 'webhooks', element: <Webhooks /> },
      // { path: 'webhooks/new', element: <CreateWebhook /> },
      // { path: 'webhooks/:id', element: <WebhookDetails /> },
      // { path: 'workpools', element: <WorkPools /> },
      // { path: 'workpools/:id', element: <WorkPoolDetails /> },
      // { path: 'workers', element: <Workers /> },
      // { path: 'workers/:id', element: <WorkerDetails /> },
      { path: '*', element: <NotFound /> },
    ],
  },
])

const queryClient = new QueryClient()

function App() {
  return (
    <ThemeProvider defaultTheme="system" storageKey="browsergrid-theme">
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
        <Toaster/>
      </QueryClientProvider>
    </ThemeProvider>
  )
}

export default App
