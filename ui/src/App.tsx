import { RouterProvider, createBrowserRouter } from 'react-router-dom'
import { Toaster } from 'sonner'
import { Providers } from '@/providers/Providers'
import RootLayout from '@/layouts/RootLayout'
import Overview from '@/pages/Overview'
import Sessions from '@/pages/Sessions'
import SessionDetails from '@/pages/SessionDetails'
import Profiles from '@/pages/Profiles'
import WorkPools from '@/pages/WorkPools'
import Deployments from '@/pages/Deployments'
import DeploymentDetails from '@/pages/DeploymentDetails'
import Queue from '@/pages/Queue'
//import ScheduledTasks from '@/pages/ScheduledTasks'
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
      { path: 'profiles', element: <Profiles /> },
      { path: 'workpools', element: <WorkPools /> },
      { path: 'deployments', element: <Deployments /> },
      { path: 'deployments/:id', element: <DeploymentDetails /> },
      { path: 'queue', element: <Queue /> },
      //{ path: 'scheduled-tasks', element: <ScheduledTasks /> },
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