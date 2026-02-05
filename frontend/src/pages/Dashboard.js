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

  return (
    <div>
      <Title level={4} style={{ marginBottom: 24 }}>
        概览
      </Title>
      <Row gutter={[16, 16]}>
        <Col xs={24} sm={12} lg={6}>
          <Card hoverable onClick={() => navigate('/logs')} style={{ cursor: 'pointer' }}>
            <Statistic
              title="日志总数"
              value={stats?.total_logs ?? 0}
              prefix={<FileTextOutlined />}
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card hoverable onClick={() => navigate('/metrics')} style={{ cursor: 'pointer' }}>
            <Statistic
              title="指标总数"
              value={stats?.total_metrics ?? 0}
              prefix={<BarChartOutlined />}
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card>
            <Statistic title="今日日志" value={stats?.today_logs ?? 0} />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card>
            <Statistic title="今日指标" value={stats?.today_metrics ?? 0} />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card>
            <Statistic
              title="项目/标签数"
              value={stats?.distinct_tags ?? 0}
              prefix={<TagsOutlined />}
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card>
            <Statistic
              title="规则数"
              value={stats?.distinct_rules ?? 0}
              prefix={<FilterOutlined />}
            />
          </Card>
        </Col>
      </Row>
    </div>
  );
};

export default Dashboard;
