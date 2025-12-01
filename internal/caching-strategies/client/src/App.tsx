import './App.css'
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import Dashboard from "./pages/Dashboard.tsx";

function App() {
  return (
      <div className="w-screen min-h-screen p-4 justify-center flex bg-gray-950 overflow-x-hidden">
          <div className="w-full max-w-4xl mt-12">
            <BrowserRouter>
                <Routes>
                    <Route path="/" element={<Navigate to="/dashboard" />} />
                    <Route path="/dashboard" element={<Dashboard />} />
                </Routes>
            </BrowserRouter>
          </div>
      </div>
  )
}

export default App
