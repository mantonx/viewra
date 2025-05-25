import { useAtom } from 'jotai';
import { Link, useLocation } from 'react-router-dom';
import { apiStatusAtom } from '../store/atoms';

const Header = () => {
  const [apiStatus] = useAtom(apiStatusAtom);
  const location = useLocation();

  const getStatusColor = () => {
    switch (apiStatus) {
      case 'connected':
        return 'text-green-400';
      case 'error':
        return 'text-red-400';
      default:
        return 'text-yellow-400';
    }
  };

  return (
    <header className="bg-slate-900 border-b border-slate-700 p-4">
      <div className="max-w-7xl mx-auto flex justify-between items-center">
        <div className="flex items-center space-x-6">
          <Link to="/" className="text-2xl font-bold text-white">
            Viewra
          </Link>

          <nav className="hidden md:flex space-x-4">
            <Link
              to="/"
              className={`px-3 py-2 rounded-md text-sm font-medium ${
                location.pathname === '/'
                  ? 'bg-slate-800 text-white'
                  : 'text-slate-300 hover:bg-slate-800 hover:text-white'
              }`}
            >
              Home
            </Link>
            <Link
              to="/admin"
              className={`px-3 py-2 rounded-md text-sm font-medium ${
                location.pathname === '/admin'
                  ? 'bg-slate-800 text-white'
                  : 'text-slate-300 hover:bg-slate-800 hover:text-white'
              }`}
            >
              Admin
            </Link>
          </nav>
        </div>

        <div className="flex items-center space-x-2">
          <span className="text-sm text-slate-400">Backend:</span>
          <span className={`text-sm font-medium ${getStatusColor()}`}>{apiStatus}</span>
        </div>
      </div>
    </header>
  );
};

export default Header;
