import React, { useState, useEffect } from 'react';
import { Card, Row, Col, Statistic, Typography, Spin, message } from 'antd';
import {
  FileTextOutlined,
  BarChartOutlined,
  TagsOutlined,
  FilterOutlined,
} from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import { dashboardApi } from '../api';

const { Title } = Typography;

/**
 * 仪表盘页面
 * 展示日志和指标概览统计
 */
const Dashboard = () => {
  const navigate = useNavigate();
  const [loading, setLoading] = useState(true);
  const [stats, setStats] = useState(null);

  useEffect(() => {
    dashboardApi
      .getStats()
      .then((res) => setStats(res.data))
      .catch((err) => message.error('加载统计失败'))
      .finally(() => setLoading(false));
  }, []);

  if (loading) {
    return (
      <div style={{ textAlign: 'center', padding: 80 }}>
        <Spin size="large" />
      </div>
    );
  }

  const cardDelays = [0, 60, 120, 180, 240, 300];

  return (
    <div className="lm-dashboard">
      <Title level={4} className="lm-page-title">
        概览
      </Title>
      <Row gutter={[20, 20]}>
        {[
          { key: 'logs', title: '日志总数', value: stats?.total_logs ?? 0, icon: <FileTextOutlined />, to: '/logs' },
          { key: 'metrics', title: '指标总数', value: stats?.total_metrics ?? 0, icon: <BarChartOutlined />, to: '/metrics' },
          { key: 'today-logs', title: '今日日志', value: stats?.today_logs ?? 0 },
          { key: 'today-metrics', title: '今日指标', value: stats?.today_metrics ?? 0 },
          { key: 'tags', title: '项目/标签数', value: stats?.distinct_tags ?? 0, icon: <TagsOutlined /> },
          { key: 'rules', title: '规则数', value: stats?.distinct_rules ?? 0, icon: <FilterOutlined /> },
        ].map((item, i) => (
          <Col xs={24} sm={12} lg={8} key={item.key}>
            <Card
              hoverable
              className="lm-stat-card"
              style={{ cursor: item.to ? 'pointer' : 'default', animationDelay: `${cardDelays[i]}ms` }}
              onClick={item.to ? () => navigate(item.to) : undefined}
            >
              <Statistic
                title={item.title}
                value={item.value}
                prefix={item.icon}
              />
            </Card>
          </Col>
        ))}
      </Row>
    </div>
  );
};

export default Dashboard;
