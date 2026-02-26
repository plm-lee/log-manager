import React, { useState, useEffect } from 'react';
import { Layout, Typography, Button, Space } from 'antd';
import { FileTextOutlined, LogoutOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import { authApi, clearToken } from '../api';

const { Header } = Layout;
const { Title } = Typography;

/**
 * 应用头部组件
 * 工业风暗色主题
 */
const AppHeader = () => {
  const navigate = useNavigate();
  const [loginEnabled, setLoginEnabled] = useState(false);

  useEffect(() => {
    authApi.getConfig().then((r) => setLoginEnabled(r.data?.login_enabled ?? false)).catch(() => {});
  }, []);

  const handleLogout = () => {
    clearToken();
    navigate('/login', { replace: true });
  };

  return (
    <Header
      className="lm-header"
      style={{
        background: 'var(--lm-bg-base)',
        borderBottom: '1px solid var(--lm-border)',
        padding: '0 28px',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
      }}
    >
      <Space>
        <FileTextOutlined
          style={{
            fontSize: '22px',
            color: 'var(--lm-accent-amber)',
            marginRight: '14px',
          }}
        />
        <Title
          level={4}
          style={{
            color: 'var(--lm-text-primary)',
            margin: 0,
            lineHeight: '64px',
            fontWeight: 600,
            fontFamily: 'var(--lm-font-sans)',
          }}
        >
          日志管理系统
        </Title>
      </Space>
      {loginEnabled && (
        <Button type="text" icon={<LogoutOutlined />} onClick={handleLogout} style={{ color: 'var(--lm-text-secondary)' }}>
          退出
        </Button>
      )}
    </Header>
  );
};

export default AppHeader;
