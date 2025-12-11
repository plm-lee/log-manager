import React, { useState, useEffect } from 'react';
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
} from 'antd';
import { SearchOutlined, ReloadOutlined } from '@ant-design/icons';
import dayjs from 'dayjs';
import { metricsApi, logApi } from '../api';

const { Option } = Select;
const { RangePicker } = DatePicker;
const { Text, Title } = Typography;

/**
 * 指标列表页面组件
 * 显示指标统计数据，支持按标签、时间范围等条件查询
 */
const MetricsList = () => {
  const [metrics, setMetrics] = useState([]);
  const [loading, setLoading] = useState(false);
  const [tags, setTags] = useState([]);
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

  // 加载指标数据
  const loadMetrics = async (page = 1, pageSize = 20) => {
    setLoading(true);
    try {
      const params = {
        page,
        page_size: pageSize,
        ...filters,
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
  }, []);

  // 处理查询
  const handleSearch = () => {
    setPagination({ ...pagination, current: 1 });
    loadMetrics(1, pagination.pageSize);
  };

  // 处理重置
  const handleReset = () => {
    setFilters({
      tag: undefined,
      start_time: undefined,
      end_time: undefined,
    });
    setPagination({ ...pagination, current: 1 });
    setTimeout(() => {
      loadMetrics(1, pagination.pageSize);
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
            <Col span={8}>
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
