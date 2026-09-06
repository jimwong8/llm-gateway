import { Suspense } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider } from 'react-router-dom'
import { Toaster } from 'sonner'
import { router } from './router'
import { PageSkeleton } from './components/common/PageSkeleton'
import { ErrorBoundary } from './components/common/ErrorBoundary'
import { CommandMenu } from './components/common/CommandMenu'
import { ThemeProvider } from './context/ThemeContext'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 60_000,
      gcTime: 5 * 60_000,
      retry: 1,
      refetchOnWindowFocus: false,
      refetchOnMount: false,
    },
    mutations: { retry: 0 },
  },
})

export function App() {
  return (
    <ErrorBoundary>
      <QueryClientProvider client={queryClient}>
        <ThemeProvider>
          <Suspense fallback={<PageSkeleton />}>
            <RouterProvider router={router} />
          </Suspense>
          <CommandMenu />
          <Toaster position="top-right" richColors closeButton />
        </ThemeProvider>
      </QueryClientProvider>
    </ErrorBoundary>
  )
}
