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
  Collapse,
} from 'antd';
import { useNavigate } from 'react-router-dom';
import { SearchOutlined, SettingOutlined, WarningOutlined } from '@ant-design/icons';
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
  const [selectedTags, setSelectedTags] = useState([]);
  const [tagOptions, setTagOptions] = useState([]);
  const [unmatched, setUnmatched] = useState([]);
  const [unmatchedLoading, setUnmatchedLoading] = useState(false);

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
      const res = await billingApi.getStats({
        start_date: dateRange[0].format('YYYY-MM-DD'),
        end_date: dateRange[1].format('YYYY-MM-DD'),
        tags: selectedTags.length ? selectedTags : undefined,
      });
      setData(res.data.data || []);
      setTotalAmount(res.data.total_amount || 0);
    } catch (err) {
      message.error('加载计费统计失败: ' + (err.response?.data?.message || err.message));
    } finally {
      setLoading(false);
    }
  };

  const loadUnmatched = async () => {
    setUnmatchedLoading(true);
    try {
      const res = await billingApi.getUnmatched();
      setUnmatched(res.data.data || []);
    } catch {
      setUnmatched([]);
    } finally {
      setUnmatchedLoading(false);
    }
  };

  useEffect(() => {
    loadTags();
    loadUnmatched();
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

      <Collapse
        style={{ marginTop: 16 }}
        items={[
          {
            key: 'unmatched',
            label: (
              <Space>
                <WarningOutlined style={{ color: 'var(--lm-accent-amber)' }} />
                <span>无匹配规则（归属计费项目的 tag 但未命中任何计费规则，可据此新增配置）</span>
                {unmatched.length > 0 && (
                  <Text type="secondary">共 {unmatched.length} 条</Text>
                )}
              </Space>
            ),
            children: (
              <Table
                size="small"
                columns={[
                  { title: 'Tag', dataIndex: 'tag', key: 'tag', width: 120 },
                  { title: '规则名', dataIndex: 'rule_name', key: 'rule_name', width: 150 },
                  { title: '条数', dataIndex: 'count', key: 'count', width: 80 },
                  {
                    title: '最近时间',
                    dataIndex: 'last_seen',
                    key: 'last_seen',
                    width: 180,
                    render: (v) => (v ? dayjs(v).format('YYYY-MM-DD HH:mm:ss') : '-'),
                  },
                  {
                    title: '日志样例',
                    dataIndex: 'log_line_sample',
                    key: 'log_line_sample',
                    ellipsis: true,
                  },
                ]}
                dataSource={unmatched}
                rowKey={(r) => `${r.tag}-${r.rule_name}`}
                loading={unmatchedLoading}
                pagination={false}
              />
            ),
          },
        ]}
      />
    </div>
  );
};

export default BillingStats;
