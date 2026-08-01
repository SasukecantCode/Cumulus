import React, { Suspense, lazy, useEffect, useState } from 'react'
import { WorkspaceProvider, useWorkspace } from '@/context/WorkspaceProvider'
import { CumulusLogo } from '@/components/CumulusLogo'
import { Loader2 } from 'lucide-react'

/* Split at the signed-in boundary. Only the landing page animates its
   headline, so this keeps GSAP out of the workspace bundle entirely — the
   screen people actually spend their time on never downloads it. */
const LandingPage = lazy(() =>
  import('@/components/LandingPage').then((m) => ({ default: m.LandingPage })),
)
const Dashboard = lazy(() =>
  import('@/components/Dashboard').then((m) => ({ default: m.Dashboard })),
)

function LoadingState() {
  return (
    <div className="min-h-screen grid place-items-center p-6">
      <div className="card px-10 py-9 flex flex-col items-center gap-4">
        <CumulusLogo size="lg" layout="vertical" withWordmark={false} />
        <p className="text-sm text-muted inline-flex items-center gap-2">
          <Loader2 className="w-4 h-4 animate-spin" />
          Loading your drive…
        </p>
      </div>
    </div>
  )
}

function AppContent() {
  const workspaceContext = useWorkspace()
  const { loading, authenticated, workspaceId } = workspaceContext
  const [errorMsg, setErrorMsg] = useState(null)

  useEffect(() => {
    const params = new URLSearchParams(window.location.search)
    const err = params.get('error')
    if (err) setErrorMsg(err)
  }, [])

  if (loading) return <LoadingState />

  return (
    <Suspense fallback={<LoadingState />}>
      {authenticated ? (
        <Dashboard workspaceContext={workspaceContext} />
      ) : (
        <LandingPage workspaceId={workspaceId} errorMsg={errorMsg} />
      )}
    </Suspense>
  )
}

export default function App() {
  return (
    <WorkspaceProvider>
      <AppContent />
    </WorkspaceProvider>
  )
}
