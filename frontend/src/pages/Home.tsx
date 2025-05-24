import { useEffect } from 'react';
import { useAtom } from 'jotai';
import { apiStatusAtom, backendMessageAtom } from '../store/atoms';

const Home = () => {
  const [apiStatus, setApiStatus] = useAtom(apiStatusAtom);
  const [message, setMessage] = useAtom(backendMessageAtom);

  useEffect(() => {
    const testBackendConnection = async () => {
      try {
        setApiStatus('loading');
        const response = await fetch('/api/hello');
        const text = await response.text();
        setMessage(text);
        setApiStatus('connected');
      } catch {
        setMessage('Failed to connect to backend');
        setApiStatus('error');
      }
    };

    testBackendConnection();
  }, [setApiStatus, setMessage]);

  return (
    <div className="max-w-4xl mx-auto text-center">
      <div className="bg-slate-900 rounded-lg p-8 shadow-xl">
        <h1 className="text-4xl font-bold text-white mb-4">Welcome to Viewra</h1>
        <p className="text-slate-300 mb-6">Your personal media management system</p>

        <div className="bg-slate-800 rounded-lg p-6 mb-6">
          <h2 className="text-xl font-semibold text-white mb-3">Backend Connection Test</h2>
          <p
            className={`text-lg ${
              apiStatus === 'connected'
                ? 'text-green-400'
                : apiStatus === 'error'
                  ? 'text-red-400'
                  : 'text-yellow-400'
            }`}
          >
            {message || 'Connecting...'}
          </p>
        </div>

        <div className="text-slate-400 text-sm">
          <p>🎬 Future features:</p>
          <ul className="mt-2 space-y-1">
            <li>• Media library management</li>
            <li>• Video streaming & playback</li>
            <li>• User authentication</li>
            <li>• Metadata scraping</li>
          </ul>
        </div>
      </div>
    </div>
  );
};

export default Home;
