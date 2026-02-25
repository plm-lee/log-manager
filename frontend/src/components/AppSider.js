import React, { useState } from 'react';
import { Layout, Menu } from 'antd';
import { useNavigate, useLocation } from 'react-router-dom';
import {
  DashboardOutlined,
  FileTextOutlined,
  BarChartOutlined,
  UploadOutlined,
  TagsOutlined,
} from '@ant-design/icons';

const { Sider } = Layout;

/**
 * 应用侧边栏组件
 * 提供导航菜单
 */
const AppSider = () => {
  const navigate = useNavigate();
  const location = useLocation();
  const [collapsed, setCollapsed] = useState(false);

  // 菜单项配置
  const menuItems = [
    {
      key: '/',
      icon: <DashboardOutlined />,
      label: '仪表盘',
    },
    {
      key: '/logs',
      icon: <FileTextOutlined />,
      label: '日志列表',
    },
    {
      key: '/logs/upload',
      icon: <UploadOutlined />,
      label: '日志上传',
    },
    {
      key: '/metrics',
      icon: <BarChartOutlined />,
      label: '指标统计',
    },
    {
      key: '/classification',
      icon: <TagsOutlined />,
      label: '分类管理',
    },
  ];

  // 处理菜单点击
  const handleMenuClick = ({ key }) => {
    navigate(key);
  };

  return (
    <Sider
      collapsible
      collapsed={collapsed}
      onCollapse={setCollapsed}
      width={220}
      style={{
        overflow: 'auto',
        height: '100vh',
        background: 'var(--lm-bg-elevated)',
      }}
    >
      <Menu
        theme="dark"
        selectedKeys={[location.pathname]}
        mode="inline"
        items={menuItems}
        onClick={handleMenuClick}
        style={{
          background: 'transparent',
          borderRight: 0,
        }}
      />
    </Sider>
  );
};

export default AppSider;
