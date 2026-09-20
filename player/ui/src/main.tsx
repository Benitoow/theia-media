import { createRoot } from 'react-dom/client';
import { MemoryRouter } from 'react-router-dom';

import App from './App';
import './index.css';

createRoot(document.getElementById('root')!).render(
	<MemoryRouter initialEntries={['/home']}>
		<App />
	</MemoryRouter>
);
