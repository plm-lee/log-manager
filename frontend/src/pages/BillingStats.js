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
  const [totalAmount, setTotalAmount] = useState(0);
  const [dateRange, setDateRange] = useState([
    dayjs().subtract(7, 'day'),
    dayjs(),
  ]);

  const loadStats = async () => {
    if (!dateRange || dateRange.length !== 2) {
      message.warning('请选择日期范围');
      return;
    }
    setLoading(true);
    try {
      const res = await billingApi.getStats({
        start_date: dateRange[0].format('YYYY-MM-DD'),
        end_date: dateRange[1].format('YYYY-MM-DD'),
      });
      setData(res.data.data || []);
      setTotalAmount(res.data.total_amount || 0);
    } catch (err) {
      message.error('加载计费统计失败: ' + (err.response?.data?.message || err.message));
    } finally {
      setLoading(false);
    }
  };

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
          rowKey={(r) => `${r.date}-${r.bill_key}`}
          loading={loading}
          pagination={false}
        />
        {data.length === 0 && !loading && (
          <Text type="secondary" style={{ display: 'block', textAlign: 'center', padding: 24 }}>
            暂无数据，请选择日期范围并点击查询；需先配置计费规则
          </Text>
        )}
      </Card>
    </div>
  );
};

export default BillingStats;
