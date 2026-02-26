import React from 'react';
import { BrowserRouter as Router, Routes, Route, Navigate } from 'react-router-dom';
import { Layout } from 'antd';
import AppHeader from './components/AppHeader';
import AppSider from './components/AppSider';
import ProtectedRoute from './components/ProtectedRoute';
import Login from './pages/Login';
import Dashboard from './pages/Dashboard';
import LogList from './pages/LogList';
import LogUpload from './pages/LogUpload';
import MetricsList from './pages/MetricsList';
import Classification from './pages/Classification';
import BillingStats from './pages/BillingStats';
import BillingConfig from './pages/BillingConfig';
import './App.css';

const { Content } = Layout;

const MainLayout = () => (
  <Layout className="lm-layout" style={{ minHeight: '100vh' }}>
    <AppHeader />
    <Layout>
      <AppSider />
      <Layout className="lm-content-wrap">
        <Content className="lm-content">
          <Routes>
            <Route path="/" element={<Dashboard />} />
            <Route path="/logs" element={<LogList />} />
            <Route path="/logs/upload" element={<LogUpload />} />
            <Route path="/metrics" element={<MetricsList />} />
            <Route path="/classification" element={<Classification />} />
            <Route path="/billing" element={<BillingStats />} />
            <Route path="/billing/config" element={<BillingConfig />} />
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </Content>
      </Layout>
    </Layout>
  </Layout>
);

/**
 * 应用主组件
 * 包含路由配置和整体布局
 */
function App() {
  return (
    <Router basename={process.env.NODE_ENV === 'production' ? '/log/manager' : '/'}>
      <Routes>
        <Route path="/login" element={<Login />} />
        <Route path="*" element={<ProtectedRoute><MainLayout /></ProtectedRoute>} />
      </Routes>
    </Router>
  );
}

export default App;
