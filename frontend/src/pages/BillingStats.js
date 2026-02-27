import React, { useState, useEffect } from 'react';
import {
  Card,
  Table,
  DatePicker,
  Button,
  Space,
  Typography,
  message,
  Statistic,
  Select,
} from 'antd';
import { useNavigate } from 'react-router-dom';
import { SearchOutlined, SettingOutlined } from '@ant-design/icons';
import dayjs from 'dayjs';
import { billingApi } from '../api';

const { RangePicker } = DatePicker;
const { Title, Text } = Typography;

/**
 * 计费统计页面
 * 按日期范围统计计费金额，展示日期、计费类型、数量、单价、金额
 */
const BillingStats = () => {
  const navigate = useNavigate();
  const [loading, setLoading] = useState(false);
  const [data, setData] = useState([]);
  const [unmatchedData, setUnmatchedData] = useState([]);
  const [totalAmount, setTotalAmount] = useState(0);
  const [dateRange, setDateRange] = useState([
    dayjs().subtract(7, 'day'),
    dayjs(),
  ]);
  const [selectedTags, setSelectedTags] = useState([]);
  const [tagOptions, setTagOptions] = useState([]);

  const loadTags = async () => {
    try {
      const res = await billingApi.getTags();
      setTagOptions((res.data.data || []).map((v) => ({ label: v, value: v })));
    } catch {
      setTagOptions([]);
    }
  };

  const loadStats = async () => {
    if (!dateRange || dateRange.length !== 2) {
      message.warning('请选择日期范围');
      return;
    }
    setLoading(true);
    try {
      const start = dateRange[0].format('YYYY-MM-DD');
      const end = dateRange[1].format('YYYY-MM-DD');
      const [statsRes, unmatchedRes] = await Promise.all([
        billingApi.getStats({
          start_date: start,
          end_date: end,
          tags: selectedTags.length ? selectedTags : undefined,
        }),
        billingApi.getUnmatched({ start_date: start, end_date: end }),
      ]);
      setData(statsRes.data.data || []);
      setTotalAmount(statsRes.data.total_amount || 0);
      setUnmatchedData(unmatchedRes.data.data || []);
    } catch (err) {
      message.error('加载计费统计失败: ' + (err.response?.data?.message || err.message));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadTags();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    loadStats();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const columns = [
    {
      title: '日期',
      dataIndex: 'date',
      key: 'date',
      width: 120,
      render: (date) => (date ? dayjs(date).format('YYYY年MM月DD日') : '-'),
    },
    {
      title: '计费类型',
      dataIndex: 'bill_key',
      key: 'bill_key',
    },
    {
      title: '标签',
      dataIndex: 'tag',
      key: 'tag',
      width: 120,
      render: (v) => (v || '-'),
    },
    {
      title: '数量',
      dataIndex: 'count',
      key: 'count',
      width: 100,
    },
    {
      title: '单价',
      dataIndex: 'unit_price',
      key: 'unit_price',
      width: 100,
      render: (v) => (v != null ? v.toFixed(4) : '-'),
    },
    {
      title: '金额（元）',
      dataIndex: 'amount',
      key: 'amount',
      width: 120,
      render: (v) => (v != null ? v.toFixed(3) : '-'),
    },
  ];

  return (
    <div>
      <Title level={4} className="lm-page-title">
        计费统计
      </Title>
      <Card style={{ marginBottom: 16 }}>
        <Space wrap size="middle">
          <RangePicker
            value={dateRange}
            onChange={setDateRange}
            format="YYYY-MM-DD"
          />
          <Select
            mode="multiple"
            placeholder="按标签筛选（不选=全部）"
            value={selectedTags}
            onChange={setSelectedTags}
            options={tagOptions}
            allowClear
            style={{ minWidth: 200 }}
          />
          <Button
            type="primary"
            icon={<SearchOutlined />}
            onClick={loadStats}
            loading={loading}
          >
            查询
          </Button>
          <Button
            icon={<SettingOutlined />}
            onClick={() => navigate('/billing/config')}
          >
            计费配置
          </Button>
        </Space>
      </Card>

      <Card
        title="统计明细"
        extra={
          <Statistic
            title="汇总金额（元）"
            value={totalAmount}
            precision={3}
            valueStyle={{ color: 'var(--lm-accent-amber)', fontSize: 18 }}
          />
        }
      >
        <Table
          columns={columns}
          dataSource={data}
          rowKey={(r) => `${r.date}-${r.bill_key}-${r.tag || ''}`}
          loading={loading}
          pagination={false}
        />
        {data.length === 0 && !loading && (
          <Text type="secondary" style={{ display: 'block', textAlign: 'center', padding: 24 }}>
            暂无数据，请选择日期范围并点击查询；需先配置计费规则
          </Text>
        )}
      </Card>

      {unmatchedData.length > 0 && (
        <Card title="无匹配规则的计费日志" style={{ marginTop: 16 }}>
          <p style={{ marginBottom: 16, color: 'var(--lm-text-secondary)' }}>
            以下为归属计费项目但未匹配任何计费规则的日志，请前往「计费配置」新增规则。
          </p>
          <Table
            columns={[
              { title: '日期', dataIndex: 'date', key: 'date', width: 120 },
              { title: '标签', dataIndex: 'tag', key: 'tag', width: 120 },
              { title: '数量', dataIndex: 'count', key: 'count', width: 80 },
              {
                title: '示例日志',
                dataIndex: 'sample_log_line',
                key: 'sample_log_line',
                ellipsis: true,
                render: (v) => (v || '-'),
              },
            ]}
            dataSource={unmatchedData}
            rowKey={(r) => `${r.date}-${r.tag}`}
            pagination={false}
            size="small"
          />
        </Card>
      )}
    </div>
  );
};

export default BillingStats;
