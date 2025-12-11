import React from 'react';
import { Layout, Typography } from 'antd';
import { FileTextOutlined } from '@ant-design/icons';

const { Header } = Layout;
const { Title } = Typography;

/**
 * 应用头部组件
 * 显示系统标题和图标
 */
const AppHeader = () => {
  return (
    <Header
      style={{
        background: '#001529',
        padding: '0 24px',
        display: 'flex',
        alignItems: 'center',
      }}
    >
      <FileTextOutlined
        style={{ fontSize: '24px', color: '#fff', marginRight: '16px' }}
      />
      <Title
        level={4}
        style={{ color: '#fff', margin: 0, lineHeight: '64px' }}
      >
        日志管理系统
      </Title>
    </Header>
  );
};

export default AppHeader;
