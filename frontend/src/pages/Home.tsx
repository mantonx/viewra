import { useEffect, useState } from 'react';
import { useAtom } from 'jotai';
import { apiStatusAtom, backendMessageAtom } from '../store/atoms';
import ApiTester from '../components/ApiTester';
import SystemInfo from '../components/SystemInfo';
import MediaUpload from '../components/MediaUpload';
import MediaLibraryManager from '../components/MediaLibraryManager';
import MusicLibrary from '../components/MusicLibrary';

interface User {
  id: number;
  username: string;
  email: string;
  created_at: string;
}

interface Media {
  id: number;
  filename: string;
  original_name: string;
  size: number;
  mime_type: string;
  created_at: string;
}

const Home = () => {
  const [apiStatus, setApiStatus] = useAtom(apiStatusAtom);
  const [message, setMessage] = useAtom(backendMessageAtom);
  const [dbStatus, setDbStatus] = useState<string>('');
  const [users, setUsers] = useState<User[]>([]);
  const [media, setMedia] = useState<Media[]>([]);

  useEffect(() => {
    const testConnections = async () => {
      try {
        setApiStatus('loading');

        // Test basic backend connection
        const helloResponse = await fetch('/api/hello');
        const helloText = await helloResponse.text();
        setMessage(helloText);

        // Test database connection
        const dbResponse = await fetch('/api/db-status');
        const dbData = await dbResponse.json();
        setDbStatus(dbData.status);

        // Fetch users
        const usersResponse = await fetch('/api/users');
        const usersData = await usersResponse.json();
        setUsers(usersData.users || []);

        // Fetch media
        const mediaResponse = await fetch('/api/media');
        const mediaData = await mediaResponse.json();
        setMedia(mediaData.media || []);

        setApiStatus('connected');
      } catch (error) {
        console.error('Connection test failed:', error);
        setMessage('Failed to connect to backend');
        setApiStatus('error');
      }
    };

    testConnections();
  }, [setApiStatus, setMessage]);

  const createTestUser = async () => {
    try {
      const response = await fetch('/api/users', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          username: `testuser_${Date.now()}`,
          email: `test_${Date.now()}@example.com`,
          password: 'testpassword',
        }),
      });

      if (response.ok) {
        const newUser = await response.json();
        setUsers((prev) => [...prev, newUser.user]);
      }
    } catch (error) {
      console.error('Failed to create user:', error);
    }
  };

  return (
    <div className="max-w-6xl mx-auto">
      <div className="text-center mb-8">
        <h1 className="text-4xl font-bold text-white mb-4">Welcome to Viewra</h1>
        <p className="text-slate-300">Your personal media management system</p>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
        {/* Backend Connection Status */}
        <div className="bg-slate-900 rounded-lg p-6 shadow-xl">
          <h2 className="text-xl font-semibold text-white mb-3">🔧 Backend Status</h2>
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

        {/* Database Status */}
        <div className="bg-slate-900 rounded-lg p-6 shadow-xl">
          <h2 className="text-xl font-semibold text-white mb-3">💾 Database Status</h2>
          <p className={`text-lg ${dbStatus === 'connected' ? 'text-green-400' : 'text-red-400'}`}>
            {dbStatus === 'connected' ? '✅ Connected' : '❌ Not Connected'}
          </p>
        </div>

        {/* Media Count */}
        <div className="bg-slate-900 rounded-lg p-6 shadow-xl">
          <h2 className="text-xl font-semibold text-white mb-3">🎬 Media Files</h2>
          <p className="text-2xl font-bold text-blue-400">{media.length}</p>
          <p className="text-slate-400 text-sm">files in library</p>
        </div>

        {/* Users Management */}
        <div className="bg-slate-900 rounded-lg p-6 shadow-xl md:col-span-2 lg:col-span-3">
          <div className="flex justify-between items-center mb-4">
            <h2 className="text-xl font-semibold text-white">👥 Users ({users.length})</h2>
            <button
              onClick={createTestUser}
              className="bg-blue-600 hover:bg-blue-700 text-white px-4 py-2 rounded-lg transition-colors"
            >
              Create Test User
            </button>
          </div>

          {users.length === 0 ? (
            <p className="text-slate-400">No users found. Create one to get started!</p>
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
              {users.map((user) => (
                <div key={user.id} className="bg-slate-800 rounded-lg p-4">
                  <h3 className="text-white font-semibold">{user.username}</h3>
                  <p className="text-slate-400 text-sm">{user.email}</p>
                  <p className="text-slate-500 text-xs mt-2">
                    Created: {new Date(user.created_at).toLocaleDateString()}
                  </p>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>

      <div className="mt-8 bg-slate-900 rounded-lg p-6 shadow-xl">
        <h2 className="text-xl font-semibold text-white mb-4">🚀 Development Environment</h2>
        <div className="text-slate-400 text-sm space-y-2">
          <p>✅ Frontend: React + TypeScript + Vite + Tailwind CSS</p>
          <p>✅ Backend: Go + Gin + GORM</p>
          <p>✅ Database: SQLite (dev) / PostgreSQL (production)</p>
          <p>✅ Container Orchestration: Docker Compose</p>
          <p>✅ Hot Reloading: Enabled for both frontend and backend</p>
        </div>
      </div>

      {/* API Tester Section */}
      <div className="mt-8">
        <ApiTester />
      </div>

      {/* Media Upload Section */}
      <div className="mt-8">
        <MediaUpload />
      </div>

      {/* Media Library Manager Section */}
      <div className="mt-8">
        <MediaLibraryManager />
      </div>

      {/* Music Library Section */}
      <div className="mt-8">
        <MusicLibrary />
      </div>

      {/* System Information */}
      <div className="mt-8">
        <SystemInfo />
      </div>

      {/* Future Features Section */}
      <div className="mt-8 bg-slate-900 rounded-lg p-6 shadow-xl">
        <h2 className="text-xl font-semibold text-white mb-4">🔮 Coming Soon</h2>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
          <div>
            <h3 className="text-lg font-medium text-white mb-2">📱 Core Features</h3>
            <ul className="text-slate-400 text-sm space-y-1">
              <li>• Media upload & organization</li>
              <li>• Video streaming & playback</li>
              <li>• User authentication & profiles</li>
              <li>• Search & filtering</li>
              <li>• Playlist management</li>
            </ul>
          </div>
          <div>
            <h3 className="text-lg font-medium text-white mb-2">🎯 Advanced Features</h3>
            <ul className="text-slate-400 text-sm space-y-1">
              <li>• Metadata scraping & enrichment</li>
              <li>• Thumbnail generation</li>
              <li>• Video transcoding</li>
              <li>• Mobile responsive design</li>
              <li>• API access & integrations</li>
            </ul>
          </div>
        </div>
      </div>
    </div>
  );
};

export default Home;
