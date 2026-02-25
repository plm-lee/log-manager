import React, { useState, useEffect, useRef } from 'react';
import {
  Table,
  Card,
  Select,
  DatePicker,
  Button,
  Space,
  Tag,
  Typography,
  message,
  Row,
  Col,
  Descriptions,
  Switch,
  Radio,
  Statistic,
} from 'antd';
import {
  SearchOutlined,
  ReloadOutlined,
  PlayCircleOutlined,
  PauseCircleOutlined,
} from '@ant-design/icons';
import { Line } from '@ant-design/charts';
import dayjs from 'dayjs';
import { metricsApi, logApi } from '../api';

const { Option } = Select;
const { RangePicker } = DatePicker;
const { Text, Title } = Typography;

/**
 * 指标列表页面组件
 * 显示指标统计数据，支持按标签、时间范围等条件查询
 * 包含图表展示和实时刷新功能
 */
const MetricsList = () => {
  const [metrics, setMetrics] = useState([]);
  const [stats, setStats] = useState([]);
  const [loading, setLoading] = useState(false);
  const [statsLoading, setStatsLoading] = useState(false);
  const [tags, setTags] = useState([]);
  const [autoRefresh, setAutoRefresh] = useState(false);
  const refreshTimerRef = useRef(null);
  const [pagination, setPagination] = useState({
    current: 1,
    pageSize: 20,
    total: 0,
  });

  // 查询条件
  const [filters, setFilters] = useState({
    tag: undefined,
    start_time: undefined,
    end_time: undefined,
    interval: '1h', // 默认1小时聚合
  });

  // 加载标签列表
  const loadTags = async () => {
    try {
      const response = await logApi.getTags();
      setTags(response.data.tags || []);
    } catch (error) {
      console.error('加载标签失败:', error);
    }
  };

  // 加载指标统计数据（用于图表）
  const loadMetricsStats = async () => {
    setStatsLoading(true);
    try {
      const params = {
        ...filters,
      };

      // 移除空值
      Object.keys(params).forEach(
        (key) => params[key] === undefined && delete params[key]
      );

      const response = await metricsApi.queryMetricsStats(params);
      setStats(response.data.stats || []);
    } catch (error) {
      message.error('加载指标统计失败: ' + (error.message || '未知错误'));
      console.error('加载指标统计失败:', error);
    } finally {
      setStatsLoading(false);
    }
  };

  // 加载指标数据
  const loadMetrics = async (page = 1, pageSize = 20) => {
    setLoading(true);
    try {
      const params = {
        page,
        page_size: pageSize,
        tag: filters.tag,
        start_time: filters.start_time,
        end_time: filters.end_time,
      };

      // 移除空值
      Object.keys(params).forEach(
        (key) => params[key] === undefined && delete params[key]
      );

      const response = await metricsApi.queryMetrics(params);
      const {
        metrics: metricsList,
        total,
        page: currentPage,
        page_size,
      } = response.data;

      setMetrics(metricsList || []);
      setPagination({
        current: currentPage || page,
        pageSize: page_size || pageSize,
        total: total || 0,
      });
    } catch (error) {
      message.error('加载指标失败: ' + (error.message || '未知错误'));
      console.error('加载指标失败:', error);
    } finally {
      setLoading(false);
    }
  };

  // 初始加载
  useEffect(() => {
    loadTags();
    loadMetrics();
    loadMetricsStats();
  }, []);

  // 自动刷新
  useEffect(() => {
    if (autoRefresh) {
      refreshTimerRef.current = setInterval(() => {
        loadMetricsStats();
        loadMetrics(pagination.current, pagination.pageSize);
      }, 5000); // 每5秒刷新一次
    } else {
      if (refreshTimerRef.current) {
        clearInterval(refreshTimerRef.current);
        refreshTimerRef.current = null;
      }
    }

    return () => {
      if (refreshTimerRef.current) {
        clearInterval(refreshTimerRef.current);
      }
    };
  }, [autoRefresh, pagination.current, pagination.pageSize]);

  // 处理查询
  const handleSearch = () => {
    setPagination({ ...pagination, current: 1 });
    loadMetrics(1, pagination.pageSize);
    loadMetricsStats();
  };

  // 处理重置
  const handleReset = () => {
    setFilters({
      tag: undefined,
      start_time: undefined,
      end_time: undefined,
      interval: '1h',
    });
    setPagination({ ...pagination, current: 1 });
    setTimeout(() => {
      loadMetrics(1, pagination.pageSize);
      loadMetricsStats();
    }, 100);
  };

  // 处理时间范围选择
  const handleTimeRangeChange = (dates) => {
    if (dates && dates.length === 2) {
      setFilters({
        ...filters,
        start_time: dates[0].unix(),
        end_time: dates[1].unix(),
      });
    } else {
      setFilters({
        ...filters,
        start_time: undefined,
        end_time: undefined,
      });
    }
  };

  // 处理聚合间隔变化
  const handleIntervalChange = (e) => {
    setFilters({
      ...filters,
      interval: e.target.value,
    });
  };

  // 准备图表数据
  const prepareChartData = () => {
    if (!stats || stats.length === 0) {
      return [];
    }

    // 收集所有规则名称
    const ruleNames = new Set();
    stats.forEach((stat) => {
      Object.keys(stat.rule_counts || {}).forEach((ruleName) => {
        ruleNames.add(ruleName);
      });
    });

    // 构建图表数据
    const chartData = [];
    stats.forEach((stat) => {
      // 总计数
      chartData.push({
        time: stat.time_str,
        timestamp: stat.time,
        type: '总计数',
        value: stat.total_count,
      });

      // 各规则计数
      Object.entries(stat.rule_counts || {}).forEach(([ruleName, count]) => {
        chartData.push({
          time: stat.time_str,
          timestamp: stat.time,
          type: ruleName,
          value: count,
        });
      });
    });

    return chartData;
  };

  // 计算总统计
  const calculateTotalStats = () => {
    let totalCount = 0;
    const ruleCounts = {};

    stats.forEach((stat) => {
      totalCount += stat.total_count || 0;
      Object.entries(stat.rule_counts || {}).forEach(([ruleName, count]) => {
        ruleCounts[ruleName] = (ruleCounts[ruleName] || 0) + count;
      });
    });

    return { totalCount, ruleCounts };
  };

  const { totalCount, ruleCounts } = calculateTotalStats();
  const chartData = prepareChartData();

  // 获取所有指标类型（用于颜色映射）
  const getMetricTypes = () => {
    const types = new Set();
    chartData.forEach((item) => {
      types.add(item.type);
    });
    return Array.from(types);
  };

  // 定义颜色映射（工业风主题）
  const getColorConfig = () => {
    const types = getMetricTypes();
    const colorPalette = [
      '#f59e0b', // 琥珀
      '#22d3ee', // 青色
      '#10b981', // 翠绿
      '#94a3b8', // 灰蓝
      '#f472b6', // 粉
      '#a78bfa', // 紫
      '#fb923c', // 橙
      '#34d399', // 薄荷绿
    ];

    const colorMap = {};
    let colorIndex = 0;

    if (types.includes('总计数')) {
      colorMap['总计数'] = '#f59e0b';
    }
    
    // 再处理其他规则
    types.forEach((type) => {
      if (type !== '总计数') {
        colorMap[type] = colorPalette[colorIndex % colorPalette.length];
        colorIndex++;
      }
    });

    const colorArray = types.map((type) => colorMap[type] || '#f59e0b');
    
    return { colorMap, colorArray };
  };

  const { colorArray } = getColorConfig();

  // 图表配置
  const lineConfig = {
    data: chartData,
    xField: 'time',
    yField: 'value',
    seriesField: 'type',
    smooth: true,
    point: {
      size: 4,
      shape: 'circle',
    },
    legend: {
      position: 'top',
    },
    color: colorArray,
    animation: {
      appear: {
        animation: 'wave-in',
        duration: 2000,
      },
    },
    tooltip: {
      formatter: (datum) => {
        return { name: datum.type, value: datum.value };
      },
    },
  };


  // 表格列定义
  const columns = [
    {
      title: 'ID',
      dataIndex: 'id',
      key: 'id',
      width: 80,
    },
    {
      title: '时间',
      dataIndex: 'timestamp',
      key: 'timestamp',
      width: 180,
      render: (timestamp) => {
        return dayjs.unix(timestamp).format('YYYY-MM-DD HH:mm:ss');
      },
    },
    {
      title: '标签',
      dataIndex: 'tag',
      key: 'tag',
      width: 120,
      render: (tag) => {
        return tag ? <Tag color="green">{tag}</Tag> : '-';
      },
    },
    {
      title: '总计数',
      dataIndex: 'total_count',
      key: 'total_count',
      width: 120,
      render: (count) => {
        return <Text strong>{count}</Text>;
      },
    },
    {
      title: '统计时长（秒）',
      dataIndex: 'duration',
      key: 'duration',
      width: 150,
    },
    {
      title: '规则计数',
      dataIndex: 'rule_counts',
      key: 'rule_counts',
      render: (ruleCounts) => {
        if (!ruleCounts || Object.keys(ruleCounts).length === 0) {
          return '-';
        }
        return (
          <Space wrap>
            {Object.entries(ruleCounts).map(([ruleName, count]) => (
              <Tag key={ruleName} color="purple">
                {ruleName}: {count}
              </Tag>
            ))}
          </Space>
        );
      },
    },
  ];

  // 处理分页变化
  const handleTableChange = (newPagination) => {
    loadMetrics(newPagination.current, newPagination.pageSize);
  };

  return (
    <div>
      <Card title="指标查询" style={{ marginBottom: 16 }}>
        <Space direction="vertical" style={{ width: '100%' }} size="middle">
          <Row gutter={16}>
            <Col span={6}>
              <Select
                placeholder="选择标签"
                style={{ width: '100%' }}
                allowClear
                value={filters.tag}
                onChange={(value) => setFilters({ ...filters, tag: value })}
              >
                {tags.map((tag) => (
                  <Option key={tag} value={tag}>
                    {tag}
                  </Option>
                ))}
              </Select>
            </Col>
            <Col span={8}>
              <RangePicker
                style={{ width: '100%' }}
                showTime
                format="YYYY-MM-DD HH:mm:ss"
                onChange={handleTimeRangeChange}
              />
            </Col>
            <Col span={6}>
              <Radio.Group
                value={filters.interval}
                onChange={handleIntervalChange}
                buttonStyle="solid"
              >
                <Radio.Button value="1m">1分钟</Radio.Button>
                <Radio.Button value="5m">5分钟</Radio.Button>
                <Radio.Button value="15m">15分钟</Radio.Button>
                <Radio.Button value="1h">1小时</Radio.Button>
                <Radio.Button value="1d">1天</Radio.Button>
              </Radio.Group>
            </Col>
            <Col span={4}>
              <Space>
                <Switch
                  checkedChildren={<PlayCircleOutlined />}
                  unCheckedChildren={<PauseCircleOutlined />}
                  checked={autoRefresh}
                  onChange={setAutoRefresh}
                />
                <Text>自动刷新</Text>
              </Space>
            </Col>
          </Row>
          <Space>
            <Button
              type="primary"
              icon={<SearchOutlined />}
              onClick={handleSearch}
            >
              查询
            </Button>
            <Button icon={<ReloadOutlined />} onClick={handleReset}>
              重置
            </Button>
          </Space>
        </Space>
      </Card>

      {/* 统计概览 */}
      <Row gutter={16} style={{ marginBottom: 16 }}>
        <Col span={8}>
          <Card>
            <Statistic
              title="总匹配数"
              value={totalCount}
              valueStyle={{ color: 'var(--lm-accent-amber)' }}
            />
          </Card>
        </Col>
        <Col span={16}>
          <Card title="各规则统计">
            <Space wrap>
              {Object.entries(ruleCounts).map(([ruleName, count]) => (
                <Tag key={ruleName} color="blue" style={{ fontSize: '14px', padding: '4px 12px' }}>
                  {ruleName}: {count}
                </Tag>
              ))}
              {Object.keys(ruleCounts).length === 0 && <Text type="secondary">暂无数据</Text>}
            </Space>
          </Card>
        </Col>
      </Row>

      {/* 图表展示 */}
      <Card
        title="指标趋势图"
        extra={
          <Space>
            <Text type="secondary">数据点: {stats.length}</Text>
            {autoRefresh && <Tag color="green">实时更新中</Tag>}
          </Space>
        }
        style={{ marginBottom: 16 }}
      >
        {chartData.length > 0 ? (
          <Line {...lineConfig} height={400} />
        ) : (
          <div style={{ textAlign: 'center', padding: '40px 0' }}>
            <Text type="secondary">暂无数据，请选择时间范围查询</Text>
          </div>
        )}
      </Card>

      {/* 指标列表 */}
      <Card title="指标列表">
        <Table
          columns={columns}
          dataSource={metrics}
          rowKey="id"
          loading={loading}
          pagination={{
            current: pagination.current,
            pageSize: pagination.pageSize,
            total: pagination.total,
            showSizeChanger: true,
            showTotal: (total) => `共 ${total} 条`,
            pageSizeOptions: ['10', '20', '50', '100'],
          }}
          onChange={handleTableChange}
          scroll={{ x: 1200 }}
          expandable={{
            expandedRowRender: (record) => (
              <Descriptions bordered size="small" column={2}>
                <Descriptions.Item label="规则计数详情" span={2}>
                  {record.rule_counts &&
                  Object.keys(record.rule_counts).length > 0 ? (
                    <Space direction="vertical">
                      {Object.entries(record.rule_counts).map(
                        ([ruleName, count]) => (
                          <div key={ruleName}>
                            <Text strong>{ruleName}:</Text> {count}
                          </div>
                        )
                      )}
                    </Space>
                  ) : (
                    '-'
                  )}
                </Descriptions.Item>
              </Descriptions>
            ),
          }}
        />
      </Card>
    </div>
  );
};

export default MetricsList;
