import React from 'react';
import { Layout, Typography } from 'antd';
import { FileTextOutlined } from '@ant-design/icons';

const { Header } = Layout;
const { Title } = Typography;

/**
 * 应用头部组件
 * 工业风暗色主题
 */
const AppHeader = () => {
  return (
    <Header
      className="lm-header"
      style={{
        background: 'var(--lm-bg-base)',
        borderBottom: '1px solid var(--lm-border)',
        padding: '0 28px',
        display: 'flex',
        alignItems: 'center',
      }}
    >
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
          fontFamily: "var(--lm-font-sans)",
        }}
      >
        日志管理系统
      </Title>
    </Header>
  );
};

export default AppHeader;
