function apiBasePath() {
  const apiUrl = typeof window !== 'undefined' ? (window as typeof window & { APIUrl?: string }).APIUrl : ''
  const envBase = import.meta.env.VITE_APP_BASE_API_URL || ''
  const base = apiUrl || envBase || '/api'

  if (!base) {
    return ''
  }

  return base.endsWith('/') ? base.slice(0, -1) : base
}

function buildApiPath(path: string) {
  const normalizedPath = path.startsWith('/') ? path : `/${path}`
  return `${apiBasePath()}${normalizedPath}`
}

export async function fetchJSON<T>(path: string): Promise<T> {
  const response = await fetch(buildApiPath(path), {
    headers: {
      Accept: 'application/json',
    },
  })

  if (!response.ok) {
    throw new Error(`Request failed for ${path}: ${response.status}`)
  }

  return (await response.json()) as T
}

export async function fetchPaginatedJSON<T>(path: string): Promise<{ data: T[]; nextPage: number }> {
  const response = await fetch(buildApiPath(path), {
    headers: {
      Accept: 'application/json',
    },
  })

  if (!response.ok) {
    throw new Error(`Request failed for ${path}: ${response.status}`)
  }

  return {
    data: (await response.json()) as T[],
    nextPage: Number(response.headers.get('X-Next-Page') || '1'),
  }
}
