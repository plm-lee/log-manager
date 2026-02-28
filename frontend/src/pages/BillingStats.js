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
  Modal,
} from 'antd';
import { useNavigate } from 'react-router-dom';
import { SearchOutlined, SettingOutlined, WarningOutlined, EyeOutlined } from '@ant-design/icons';
import dayjs from 'dayjs';
import { billingApi } from '../api';

const { RangePicker } = DatePicker;
const { Title, Text } = Typography;

/**
 * 计费统计页面
 * 主列表：按日汇总（日期、总数量、当天金额），分页每页20条，按日期倒序
 * 详情：点击某日「详情」弹窗展示该日计费明细
 */
const BillingStats = () => {
  const navigate = useNavigate();
  const [loading, setLoading] = useState(false);
  const [data, setData] = useState([]);
  const [totalAmount, setTotalAmount] = useState(0);
  const [totalDays, setTotalDays] = useState(0);
  const [page, setPage] = useState(1);
  const [dateRange, setDateRange] = useState([
    dayjs().subtract(7, 'day'),
    dayjs(),
  ]);
  const [selectedTags, setSelectedTags] = useState([]);
  const [tagOptions, setTagOptions] = useState([]);
  const [unmatched, setUnmatched] = useState([]);
  const [unmatchedLoading, setUnmatchedLoading] = useState(false);
  const [detailVisible, setDetailVisible] = useState(false);
  const [detailDate, setDetailDate] = useState('');
  const [detailData, setDetailData] = useState([]);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailTotal, setDetailTotal] = useState(0);
  const [detailTotalAmount, setDetailTotalAmount] = useState(0);
  const [detailPage, setDetailPage] = useState(1);

  const loadTags = async () => {
    try {
      const res = await billingApi.getTags();
      setTagOptions((res.data.data || []).map((v) => ({ label: v, value: v })));
    } catch {
      setTagOptions([]);
    }
  };

  const loadStats = async (p = 1) => {
    if (!dateRange || dateRange.length !== 2) {
      message.warning('请选择日期范围');
      return;
    }
    setLoading(true);
    try {
      const res = await billingApi.getStatsSummary({
        start_date: dateRange[0].format('YYYY-MM-DD'),
        end_date: dateRange[1].format('YYYY-MM-DD'),
        tags: selectedTags.length ? selectedTags : undefined,
        page: p,
        page_size: 20,
      });
      setData(res.data.data || []);
      setTotalAmount(res.data.total_amount || 0);
      setTotalDays(res.data.total || 0);
    } catch (err) {
      message.error('加载计费统计失败: ' + (err.response?.data?.message || err.message));
    } finally {
      setLoading(false);
    }
  };

  const loadDetail = async (dateStr, p = 1) => {
    setDetailLoading(true);
    try {
      const res = await billingApi.getStatsDetail({
        date: dateStr,
        tags: selectedTags.length ? selectedTags : undefined,
        page: p,
        page_size: 20,
      });
      setDetailData(res.data.data || []);
      setDetailTotal(res.data.total || 0);
      setDetailTotalAmount(res.data.total_amount || 0);
    } catch (err) {
      message.error('加载明细失败: ' + (err.response?.data?.message || err.message));
    } finally {
      setDetailLoading(false);
    }
  };

  const showDetail = (dateStr) => {
    setDetailDate(dateStr);
    setDetailVisible(true);
    setDetailPage(1);
    loadDetail(dateStr, 1);
  };

  const handleDetailPageChange = (p) => {
    setDetailPage(p);
    loadDetail(detailDate, p);
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
    loadStats(1);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const handleSearch = () => {
    setPage(1);
    loadStats(1);
  };

  const columns = [
    {
      title: '日期',
      dataIndex: 'date',
      key: 'date',
      width: 160,
      render: (date) => (date ? dayjs(date).format('YYYY年MM月DD日') : '-'),
    },
    {
      title: '总数量',
      dataIndex: 'total_count',
      key: 'total_count',
      width: 120,
    },
    {
      title: '当天金额（元）',
      dataIndex: 'total_amount',
      key: 'total_amount',
      width: 140,
      render: (v) => (v != null ? Number(v).toFixed(3) : '-'),
    },
    {
      title: '操作',
      key: 'action',
      width: 100,
      render: (_, record) => (
        <Button type="link" size="small" icon={<EyeOutlined />} onClick={() => showDetail(record.date)}>
          详情
        </Button>
      ),
    },
  ];

  const detailColumns = [
    { title: '计费类型', dataIndex: 'bill_key', key: 'bill_key', width: 120 },
    { title: '标签', dataIndex: 'tag', key: 'tag', width: 120, render: (v) => (v || '-') },
    { title: '数量', dataIndex: 'count', key: 'count', width: 100 },
    {
      title: '单价',
      dataIndex: 'unit_price',
      key: 'unit_price',
      width: 100,
      render: (v) => (v != null ? Number(v).toFixed(4) : '-'),
    },
    {
      title: '金额（元）',
      dataIndex: 'amount',
      key: 'amount',
      width: 120,
      render: (v) => (v != null ? Number(v).toFixed(3) : '-'),
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
            onClick={handleSearch}
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
          rowKey="date"
          loading={loading}
          pagination={{
            current: page,
            pageSize: 20,
            total: totalDays,
            showSizeChanger: false,
            showTotal: (t) => `共 ${t} 天`,
            onChange: (p) => {
              setPage(p);
              loadStats(p);
            },
          }}
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

      <Modal
        title={`${detailDate ? dayjs(detailDate).format('YYYY年MM月DD日') : ''} 计费明细`}
        open={detailVisible}
        onCancel={() => setDetailVisible(false)}
        footer={null}
        width={700}
      >
        <div style={{ marginBottom: 12 }}>
          <Text strong>当日汇总金额（元）：</Text>{' '}
          <Text style={{ color: 'var(--lm-accent-amber)' }}>{Number(detailTotalAmount).toFixed(3)}</Text>
        </div>
        <Table
          size="small"
          columns={detailColumns}
          dataSource={detailData}
          rowKey={(r) => `${r.date}-${r.bill_key}-${r.tag || ''}`}
          loading={detailLoading}
          pagination={
            detailTotal <= 20
              ? false
              : {
                  current: detailPage,
                  pageSize: 20,
                  total: detailTotal,
                  showSizeChanger: false,
                  showTotal: (t) => `共 ${t} 条`,
                  onChange: handleDetailPageChange,
                }
          }
        />
      </Modal>
    </div>
  );
};

export default BillingStats;
