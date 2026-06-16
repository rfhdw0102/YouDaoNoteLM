import { useEffect, useState } from 'react';
import { RouterProvider } from 'react-router-dom';
import { router } from './routes';
import { useAuthStore } from './stores/useAuthStore';
import Spinner from './components/ui/Spinner';

function App() {
  const restoreSession = useAuthStore((s) => s.restoreSession);
  const [ready, setReady] = useState(false);

  useEffect(() => {
    restoreSession();
    setReady(true);
  }, [restoreSession]);

  if (!ready) return <Spinner />;

  return <RouterProvider router={router} />;
}

export default App;
