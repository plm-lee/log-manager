import React from 'react';
import ReactDOM from 'react-dom/client';
import { ConfigProvider, theme as antTheme } from 'antd';
import zhCN from 'antd/locale/zh_CN';
import App from './App';
import './index.css';

// Industrial terminal theme (dark algorithm + custom tokens)
const theme = {
  algorithm: antTheme.darkAlgorithm,
  token: {
    colorPrimary: '#f59e0b',
    colorBgContainer: '#1e2a3a',
    colorBgElevated: '#243044',
    colorBorder: 'rgba(148, 163, 184, 0.12)',
    colorText: '#f1f5f9',
    colorTextSecondary: '#94a3b8',
    borderRadius: 8,
    fontFamily: "'IBM Plex Sans', -apple-system, BlinkMacSystemFont, sans-serif",
  },
  components: {
    Layout: {
      headerBg: '#0f1419',
      siderBg: '#1a2332',
      bodyBg: 'transparent',
    },
    Menu: {
      darkItemBg: '#1a2332',
      darkSubMenuItemBg: '#1e2a3a',
      darkItemSelectedBg: 'rgba(245, 158, 11, 0.15)',
      darkItemSelectedColor: '#f59e0b',
      itemSelectedBg: 'rgba(245, 158, 11, 0.15)',
    },
  },
};

const root = ReactDOM.createRoot(document.getElementById('root'));
root.render(
  <React.StrictMode>
    <ConfigProvider locale={zhCN} theme={theme}>
      <App />
    </ConfigProvider>
  </React.StrictMode>
);
