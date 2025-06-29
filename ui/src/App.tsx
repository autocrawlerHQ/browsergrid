import { RouterProvider, createBrowserRouter } from 'react-router-dom'
import { Toaster } from 'sonner'
import { Providers } from '@/providers/Providers'
import RootLayout from '@/layouts/RootLayout'
import Overview from '@/pages/Overview'
import Sessions from '@/pages/Sessions'
import SessionDetails from '@/pages/SessionDetails'
import WorkPools from '@/pages/WorkPools'
import Workers from '@/pages/Workers'
import Webhooks from '@/pages/Webhooks'
import NotFound from '@/pages/NotFound'

// Create router configuration
const router = createBrowserRouter([
  {
    path: '/',
    element: <RootLayout />,
    children: [
      { index: true, element: <Overview /> },
      { path: 'sessions', element: <Sessions /> },
      { path: 'sessions/:id', element: <SessionDetails /> },
      { path: 'workpools', element: <WorkPools /> },
      { path: 'workers', element: <Workers /> },
      { path: 'webhooks', element: <Webhooks /> },
      { path: '*', element: <NotFound /> },
    ],
  },
])

function App() {
  return (
    <Providers>
      <RouterProvider router={router} />
      <Toaster />
    </Providers>
  )
}

export default App
