import { useAtom } from 'jotai';
import { apiStatusAtom } from '../store/atoms';

const Header = () => {
  const [apiStatus] = useAtom(apiStatusAtom);

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
        <h1 className="text-2xl font-bold text-white">Viewra</h1>
        <div className="flex items-center space-x-2">
          <span className="text-sm text-slate-400">Backend:</span>
          <span className={`text-sm font-medium ${getStatusColor()}`}>{apiStatus}</span>
        </div>
      </div>
    </header>
  );
};

export default Header;
