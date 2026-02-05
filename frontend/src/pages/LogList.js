import React, { useState, useEffect } from 'react';
import { useSearchParams } from 'react-router-dom';
import {
  Table,
  Card,
  Input,
  Select,
  DatePicker,
  Button,
  Space,
  Tag,
  Typography,
  message,
  Row,
  Col,
  Modal,
  Descriptions,
} from 'antd';
import { SearchOutlined, ReloadOutlined, DownloadOutlined } from '@ant-design/icons';
import dayjs from 'dayjs';
import { logApi } from '../api';

const { Option } = Select;
const { RangePicker } = DatePicker;
const { Text } = Typography;

// 根据日志内容检测级别，返回边框色
const getLogLevelColor = (text) => {
  if (!text) return 'transparent';
  const upper = text.toUpperCase();
  if (upper.includes('ERROR') || upper.includes('FATAL') || upper.includes('CRITICAL') || upper.includes('EXCEPTION'))
    return '#cf1322';
  if (upper.includes('WARN') || upper.includes('WARNING')) return '#d46b08';
  if (upper.includes('INFO')) return '#1890ff';
  if (upper.includes('DEBUG')) return '#8c8c8c';
  return 'transparent';
};

// 高亮关键词
const highlightKeyword = (text, keyword) => {
  if (!text) return text;
  if (!keyword || !keyword.trim()) return text;
  const kw = keyword.trim();
  const regex = new RegExp(`(${kw.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')})`, 'gi');
  const parts = text.split(regex);
  return parts.map((part, i) =>
    i % 2 === 1 ? (
      <mark key={i} style={{ backgroundColor: '#fff566', padding: '0 2px' }}>
        {part}
      </mark>
    ) : (
      part
    )
  );
};

/**
 * 日志列表页面组件
 * 显示日志数据，支持按标签、规则名称、关键词、时间范围等条件查询
 */
const LogList = () => {
  const [searchParams] = useSearchParams();
  const tagFromUrl = searchParams.get('tag');
  const [logs, setLogs] = useState([]);
  const [loading, setLoading] = useState(false);
  const [tags, setTags] = useState([]);
  const [ruleNames, setRuleNames] = useState([]);
  const [pagination, setPagination] = useState({
    current: 1,
    pageSize: 20,
    total: 0,
  });

  // 查询条件（支持从 URL 传入 tag）
  const [filters, setFilters] = useState({
    tag: tagFromUrl || undefined,
    rule_name: undefined,
    keyword: undefined,
    start_time: undefined,
    end_time: undefined,
  });

  // 日志详情弹窗
  const [detailModalVisible, setDetailModalVisible] = useState(false);
  const [selectedLog, setSelectedLog] = useState(null);

  // 加载标签列表
  const loadTags = async () => {
    try {
      const response = await logApi.getTags();
      setTags(response.data.tags || []);
    } catch (error) {
      console.error('加载标签失败:', error);
    }
  };

  // 加载规则名称列表
  const loadRuleNames = async () => {
    try {
      const response = await logApi.getRuleNames();
      setRuleNames(response.data.rule_names || []);
    } catch (error) {
      console.error('加载规则名称失败:', error);
    }
  };

  // 加载日志数据
  const loadLogs = async (page = 1, pageSize = 20, override = {}) => {
    setLoading(true);
    try {
      const params = {
        page,
        page_size: pageSize,
        ...filters,
        ...override,
      };

      // 移除空值
      Object.keys(params).forEach(
        (key) => params[key] === undefined && delete params[key]
      );

      const response = await logApi.queryLogs(params);
      const { logs: logList, total, page: currentPage, page_size } = response.data;

      setLogs(logList || []);
      setPagination({
        current: currentPage || page,
        pageSize: page_size || pageSize,
        total: total || 0,
      });
    } catch (error) {
      message.error('加载日志失败: ' + (error.message || '未知错误'));
      console.error('加载日志失败:', error);
    } finally {
      setLoading(false);
    }
  };

  // 当 URL tag 变化时更新 filters 并重新加载
  useEffect(() => {
    if (tagFromUrl) {
      setFilters((f) => ({ ...f, tag: tagFromUrl }));
      loadLogs(1, 20, { tag: tagFromUrl });
    }
  }, [tagFromUrl]);

  // 初始加载
  useEffect(() => {
    loadTags();
    loadRuleNames();
    loadLogs(1, 20, tagFromUrl ? { tag: tagFromUrl } : {});
  }, []);

  // 处理查询
  const handleSearch = () => {
    setPagination({ ...pagination, current: 1 });
    loadLogs(1, pagination.pageSize);
  };

  // 处理重置
  const handleReset = () => {
    setFilters({
      tag: undefined,
      rule_name: undefined,
      keyword: undefined,
      start_time: undefined,
      end_time: undefined,
    });
    setPagination({ ...pagination, current: 1 });
    setTimeout(() => {
      loadLogs(1, pagination.pageSize);
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
        return tag ? <Tag color="blue">{tag}</Tag> : '-';
      },
    },
    {
      title: '规则名称',
      dataIndex: 'rule_name',
      key: 'rule_name',
      width: 150,
    },
    {
      title: '日志内容',
      dataIndex: 'log_line',
      key: 'log_line',
      ellipsis: { showTitle: false },
      render: (text) => {
        const borderColor = getLogLevelColor(text);
        return (
          <div
            style={{
              maxWidth: 600,
              whiteSpace: 'pre-wrap',
              wordBreak: 'break-word',
              borderLeft: `3px solid ${borderColor}`,
              paddingLeft: 8,
            }}
            title={text}
          >
            {highlightKeyword(text, filters.keyword)}
          </div>
        );
      },
    },
    {
      title: '日志文件',
      dataIndex: 'log_file',
      key: 'log_file',
      width: 200,
      ellipsis: true,
    },
  ];

  // 处理分页变化
  const handleTableChange = (newPagination) => {
    loadLogs(newPagination.current, newPagination.pageSize);
  };

  // 导出日志
  const handleExport = (format) => {
    const params = { format, ...filters };
    Object.keys(params).forEach((key) => params[key] === undefined && delete params[key]);
    logApi.exportLogs(params).then((res) => {
      const blob = new Blob([res.data]);
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `logs_export_${Date.now()}.${format}`;
      a.click();
      window.URL.revokeObjectURL(url);
      message.success(`已导出为 ${format.toUpperCase()} 格式`);
    }).catch((err) => {
      message.error('导出失败：' + (err.message || '未知错误'));
    });
  };

  return (
    <div>
      <Card title="日志查询" style={{ marginBottom: 16 }}>
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
            <Col span={6}>
              <Select
                placeholder="选择规则名称"
                style={{ width: '100%' }}
                allowClear
                value={filters.rule_name}
                onChange={(value) =>
                  setFilters({ ...filters, rule_name: value })
                }
              >
                {ruleNames.map((name) => (
                  <Option key={name} value={name}>
                    {name}
                  </Option>
                ))}
              </Select>
            </Col>
            <Col span={6}>
              <Input
                placeholder="关键词搜索（日志内容）"
                value={filters.keyword}
                onChange={(e) =>
                  setFilters({ ...filters, keyword: e.target.value })
                }
                onPressEnter={handleSearch}
              />
            </Col>
            <Col span={6}>
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
            <Button icon={<DownloadOutlined />} onClick={() => handleExport('csv')}>
              导出 CSV
            </Button>
            <Button icon={<DownloadOutlined />} onClick={() => handleExport('json')}>
              导出 JSON
            </Button>
          </Space>
        </Space>
      </Card>

      <Card title="日志列表">
        <Table
          columns={columns}
          dataSource={logs}
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
          onRow={(record) => ({
            onClick: () => {
              setSelectedLog(record);
              setDetailModalVisible(true);
            },
            style: { cursor: 'pointer' },
          })}
          scroll={{ x: 1200 }}
        />
      </Card>

      <Modal
        title="日志详情"
        open={detailModalVisible}
        onCancel={() => setDetailModalVisible(false)}
        footer={null}
        width={700}
      >
        {selectedLog && (
          <Descriptions bordered column={1} size="small">
            <Descriptions.Item label="ID">{selectedLog.id}</Descriptions.Item>
            <Descriptions.Item label="时间">
              {dayjs.unix(selectedLog.timestamp).format('YYYY-MM-DD HH:mm:ss')}
            </Descriptions.Item>
            <Descriptions.Item label="标签">
              {selectedLog.tag ? <Tag color="blue">{selectedLog.tag}</Tag> : '-'}
            </Descriptions.Item>
            <Descriptions.Item label="规则名称">{selectedLog.rule_name || '-'}</Descriptions.Item>
            <Descriptions.Item label="规则描述">{selectedLog.rule_desc || '-'}</Descriptions.Item>
            <Descriptions.Item label="日志文件">
              <Text copyable>{selectedLog.log_file || '-'}</Text>
            </Descriptions.Item>
            <Descriptions.Item label="匹配模式">
              <Text code copyable>
                {selectedLog.pattern || '-'}
              </Text>
            </Descriptions.Item>
            <Descriptions.Item label="日志内容">
              <pre
                style={{
                  margin: 0,
                  whiteSpace: 'pre-wrap',
                  wordBreak: 'break-word',
                  maxHeight: 300,
                  overflow: 'auto',
                  borderLeft: `3px solid ${getLogLevelColor(selectedLog.log_line)}`,
                  paddingLeft: 8,
                }}
              >
                {highlightKeyword(selectedLog.log_line, filters.keyword)}
              </pre>
            </Descriptions.Item>
          </Descriptions>
        )}
      </Modal>
    </div>
  );
};

export default LogList;
