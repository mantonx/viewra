import { useState } from 'react';

interface ApiResponse {
  status: number;
  data: unknown;
  error?: string;
}

const ApiTester = () => {
  const [response, setResponse] = useState<ApiResponse | null>(null);
  const [loading, setLoading] = useState(false);

  const testEndpoint = async (endpoint: string, method: 'GET' | 'POST' = 'GET', body?: unknown) => {
    setLoading(true);
    setResponse(null);

    try {
      const options: RequestInit = {
        method,
        headers: {
          'Content-Type': 'application/json',
        },
      };

      if (body && method === 'POST') {
        options.body = JSON.stringify(body);
      }

      const res = await fetch(endpoint, options);

      // Get the response text first, then try to parse as JSON
      const responseText = await res.text();
      let data;

      try {
        data = JSON.parse(responseText);
      } catch {
        // If JSON parsing fails, use the text as-is
        data = responseText;
      }

      setResponse({
        status: res.status,
        data,
      });
    } catch (error) {
      setResponse({
        status: 0,
        data: null,
        error: error instanceof Error ? error.message : 'Unknown error',
      });
    } finally {
      setLoading(false);
    }
  };

  const createTestUser = () => {
    const testUser = {
      username: `testuser_${Date.now()}`,
      email: `test_${Date.now()}@example.com`,
      password: 'testpassword123',
    };
    testEndpoint('/api/users/', 'POST', testUser);
  };

  return (
    <div className="bg-slate-900 rounded-lg p-6 shadow-xl">
      <h2 className="text-xl font-semibold text-white mb-4">🧪 API Tester</h2>

      <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-4">
        <button
          onClick={() => testEndpoint('/api/health')}
          className="bg-green-600 hover:bg-green-700 text-white px-3 py-2 rounded text-sm transition-colors"
          disabled={loading}
        >
          Health
        </button>
        <button
          onClick={() => testEndpoint('/api/hello')}
          className="bg-blue-600 hover:bg-blue-700 text-white px-3 py-2 rounded text-sm transition-colors"
          disabled={loading}
        >
          Hello
        </button>
        <button
          onClick={() => testEndpoint('/api/db-status')}
          className="bg-purple-600 hover:bg-purple-700 text-white px-3 py-2 rounded text-sm transition-colors"
          disabled={loading}
        >
          DB Status
        </button>
        <button
          onClick={() => testEndpoint('/api/users/')}
          className="bg-indigo-600 hover:bg-indigo-700 text-white px-3 py-2 rounded text-sm transition-colors"
          disabled={loading}
        >
          Get Users
        </button>
      </div>

      <div className="grid grid-cols-2 gap-3 mb-4">
        <button
          onClick={() => testEndpoint('/api/media/')}
          className="bg-orange-600 hover:bg-orange-700 text-white px-3 py-2 rounded text-sm transition-colors"
          disabled={loading}
        >
          Get Media
        </button>
        <button
          onClick={createTestUser}
          className="bg-red-600 hover:bg-red-700 text-white px-3 py-2 rounded text-sm transition-colors"
          disabled={loading}
        >
          Create Test User
        </button>
      </div>

      {loading && (
        <div className="mb-4">
          <div className="animate-pulse text-yellow-400">🔄 Loading...</div>
        </div>
      )}

      {response && (
        <div className="bg-slate-800 rounded p-4">
          <div className="flex items-center gap-2 mb-2">
            <span className="text-sm font-medium text-white">Status:</span>
            <span
              className={`text-sm font-bold ${
                response.status >= 200 && response.status < 300
                  ? 'text-green-400'
                  : response.status >= 400
                    ? 'text-red-400'
                    : 'text-yellow-400'
              }`}
            >
              {response.status || 'ERROR'}
            </span>
          </div>

          {response.error && (
            <div className="mb-2">
              <span className="text-sm font-medium text-white">Error:</span>
              <div className="text-red-400 text-sm font-mono bg-slate-700 p-2 rounded mt-1">
                {response.error}
              </div>
            </div>
          )}

          <div>
            <span className="text-sm font-medium text-white">Response:</span>
            <pre className="text-slate-300 text-xs font-mono bg-slate-700 p-2 rounded mt-1 overflow-auto max-h-40">
              {typeof response.data === 'string'
                ? response.data
                : JSON.stringify(response.data, null, 2)}
            </pre>
          </div>
        </div>
      )}
    </div>
  );
};

export default ApiTester;
