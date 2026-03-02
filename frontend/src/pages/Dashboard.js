import React, { useState, useEffect } from 'react';
import { Card, Row, Col, Statistic, Typography, Spin, message, Progress, Table } from 'antd';
import {
  FileTextOutlined,
  BarChartOutlined,
  TagsOutlined,
  FilterOutlined,
  DatabaseOutlined,
  DashboardOutlined,
  ThunderboltOutlined,
  CloudServerOutlined,
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

  const formatBytes = (bytes) => {
    if (!bytes && bytes !== 0) return '-';
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
    return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`;
  };

  const storage = stats?.storage;
  const processStats = stats?.process;
  const reqMetrics = stats?.request_metrics;
  const agentNodes = stats?.agent_nodes || [];

  const formatTime = (t) => {
    if (!t) return '-';
    const d = new Date(t);
    const now = new Date();
    const diff = Math.floor((now - d) / 1000);
    if (diff < 60) return `${diff}秒前`;
    if (diff < 3600) return `${Math.floor(diff / 60)}分钟前`;
    if (diff < 86400) return `${Math.floor(diff / 3600)}小时前`;
    return d.toLocaleString();
  };

  let storagePercent = 0;
  let storageStatus = 'normal';
  let storageStrokeColor = '#52c41a';
  if (storage && storage.critical_bytes > 0) {
    storagePercent = Math.min(100, (storage.used_bytes / storage.critical_bytes) * 100);
    const warnPercent = storage.warn_bytes > 0 ? (storage.warn_bytes / storage.critical_bytes) * 100 : 50;
    if (storagePercent >= 100) {
      storageStatus = 'exception';
      storageStrokeColor = '#ff4d4f';
    } else if (storagePercent >= warnPercent) {
      storageStatus = 'active';
      storageStrokeColor = '#fa8c16';
    }
  }

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

        {storage && (
          <Col xs={24} sm={12} lg={8}>
            <Card
              className="lm-stat-card"
              style={{ animationDelay: '360ms' }}
              title={
                <span>
                  <DatabaseOutlined style={{ marginRight: 8 }} />
                  存储用量
                </span>
              }
            >
              <div>
                <Typography.Text type="secondary">
                  {storage.storage_type === 'sqlite' ? 'SQLite' : storage.storage_type === 'mysql' ? 'MySQL' : storage.storage_type}
                </Typography.Text>
                <div style={{ marginTop: 8, marginBottom: 4 }}>
                  <Typography.Text strong>{formatBytes(storage.used_bytes)}</Typography.Text>
                </div>
                <Progress
                  percent={Math.round(storagePercent)}
                  status={storageStatus}
                  strokeColor={storageStrokeColor}
                  showInfo={true}
                />
                <Typography.Text type="secondary" style={{ fontSize: 12, marginTop: 4, display: 'block' }}>
                  超过 {formatBytes(storage.critical_bytes)} 建议清理
                </Typography.Text>
              </div>
            </Card>
          </Col>
        )}

        {processStats && (
          <Col xs={24} sm={12} lg={8}>
            <Card
              className="lm-stat-card"
              style={{ animationDelay: '420ms' }}
              title={
                <span>
                  <DashboardOutlined style={{ marginRight: 8 }} />
                  进程资源
                </span>
              }
            >
              <Row gutter={16}>
                <Col span={12}>
                  <Statistic title="内存占用" value={processStats.mem_alloc_mb?.toFixed(1) ?? '-'} suffix="MB" />
                </Col>
                <Col span={12}>
                  <Statistic
                    title="CPU"
                    value={
                      processStats.cpu_percent != null && processStats.cpu_percent > 0
                        ? processStats.cpu_percent.toFixed(1)
                        : '-'
                    }
                    suffix={processStats.cpu_percent > 0 ? '%' : ''}
                  />
                </Col>
              </Row>
            </Card>
          </Col>
        )}

        {reqMetrics && (
          <Col xs={24} sm={12} lg={8}>
            <Card
              className="lm-stat-card"
              style={{ animationDelay: '480ms' }}
              title={
                <span>
                  <ThunderboltOutlined style={{ marginRight: 8 }} />
                  请求指标
                </span>
              }
            >
              <Row gutter={16}>
                <Col span={12}>
                  <Statistic
                    title="近1分钟请求数"
                    value={reqMetrics.requests_last_minute ?? 0}
                  />
                </Col>
                <Col span={12}>
                  <Statistic
                    title="平均耗时"
                    value={
                      reqMetrics.avg_latency_ms != null && reqMetrics.avg_latency_ms > 0
                        ? reqMetrics.avg_latency_ms.toFixed(0)
                        : '-'
                    }
                    suffix={reqMetrics.avg_latency_ms > 0 ? 'ms' : ''}
                  />
                </Col>
              </Row>
            </Card>
          </Col>
        )}

        {agentNodes.length > 0 && (
          <Col xs={24}>
            <Card
              className="lm-stat-card"
              style={{ animationDelay: '540ms' }}
              title={
                <span>
                  <CloudServerOutlined style={{ marginRight: 8 }} />
                  日志子节点
                </span>
              }
            >
              <Table
                dataSource={agentNodes}
                rowKey="host"
                size="small"
                pagination={false}
                columns={[
                  {
                    title: '节点名称',
                    dataIndex: 'host',
                    key: 'host',
                    render: (v) => v || '(未知)',
                  },
                  {
                    title: '累计上报数量',
                    dataIndex: 'log_count',
                    key: 'log_count',
                    align: 'right',
                    render: (v) => v?.toLocaleString?.() ?? '0',
                  },
                  {
                    title: '最近上报',
                    dataIndex: 'last_reported_at',
                    key: 'last_reported_at',
                    render: (v) => formatTime(v),
                  },
                ]}
              />
            </Card>
          </Col>
        )}
      </Row>
    </div>
  );
};

export default Dashboard;
