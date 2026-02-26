import React, { useState, useEffect } from 'react';
import { Navigate } from 'react-router-dom';
import { Spin } from 'antd';
import { authApi, getToken } from '../api';

/**
 * 保护需要登录的路由
 * 若未启用登录则直接渲染子组件，否则校验 token
 */
const ProtectedRoute = ({ children }) => {
  const [state, setState] = useState({ loading: true, loginEnabled: false });

  useEffect(() => {
    authApi
      .getConfig()
      .then((r) => setState({ loading: false, loginEnabled: r.data?.login_enabled ?? false }))
      .catch(() => setState({ loading: false, loginEnabled: true }));
  }, []);

  if (state.loading) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '50vh' }}>
        <Spin size="large" />
      </div>
    );
  }

  if (!state.loginEnabled) {
    return children;
  }

  if (!getToken()) {
    return <Navigate to="/login" replace />;
  }

  return children;
};

export default ProtectedRoute;
