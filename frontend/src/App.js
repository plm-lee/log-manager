import React from 'react';
import { BrowserRouter as Router, Routes, Route } from 'react-router-dom';
import { Layout } from 'antd';
import AppHeader from './components/AppHeader';
import AppSider from './components/AppSider';
import Dashboard from './pages/Dashboard';
import LogList from './pages/LogList';
import LogUpload from './pages/LogUpload';
import MetricsList from './pages/MetricsList';
import Classification from './pages/Classification';
import './App.css';

const { Content } = Layout;

/**
 * 应用主组件
 * 包含路由配置和整体布局
 */
function App() {
  return (
    <Router>
      <Layout style={{ minHeight: '100vh' }}>
        <AppHeader />
        <Layout>
          <AppSider />
          <Layout style={{ padding: '24px' }}>
            <Content
              style={{
                background: '#fff',
                padding: 24,
                margin: 0,
                minHeight: 280,
              }}
            >
              <Routes>
                <Route path="/" element={<Dashboard />} />
                <Route path="/logs" element={<LogList />} />
                <Route path="/logs/upload" element={<LogUpload />} />
                <Route path="/metrics" element={<MetricsList />} />
                <Route path="/classification" element={<Classification />} />
              </Routes>
            </Content>
          </Layout>
        </Layout>
      </Layout>
    </Router>
  );
}

export default App;
