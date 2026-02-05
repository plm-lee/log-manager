import React, { useState, useEffect } from 'react';
import {
  Card,
  Table,
  Tag,
  Typography,
  message,
  Space,
} from 'antd';
import { useNavigate } from 'react-router-dom';
import { logApi } from '../api';

const { Title } = Typography;

/**
 * 分类管理页面
 * 展示标签及对应日志数量
 */
const Classification = () => {
  const navigate = useNavigate();
  const [loading, setLoading] = useState(true);
  const [tagStats, setTagStats] = useState([]);

  useEffect(() => {
    loadTagStats();
  }, []);

  const loadTagStats = async () => {
    setLoading(true);
    try {
      const res = await logApi.getTagStats();
      setTagStats(res.data.tags || []);
    } catch (err) {
      message.error('加载标签统计失败');
    } finally {
      setLoading(false);
    }
  };

  const columns = [
    {
      title: '标签',
      dataIndex: 'tag',
      key: 'tag',
      render: (tag) => <Tag color="blue">{tag}</Tag>,
    },
    {
      title: '日志数量',
      dataIndex: 'count',
      key: 'count',
    },
    {
      title: '操作',
      key: 'action',
      render: (_, record) => (
        <Space>
          <a onClick={() => navigate(`/logs?tag=${encodeURIComponent(record.tag)}`)}>
            查看日志
          </a>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <Title level={4} style={{ marginBottom: 24 }}>
        分类管理
      </Title>
      <Card>
        <p style={{ marginBottom: 16, color: '#666' }}>
          按标签区分不同项目上报的日志，点击「查看日志」可筛选该标签下的日志。
        </p>
        <Table
          columns={columns}
          dataSource={tagStats}
          rowKey="tag"
          loading={loading}
          pagination={false}
        />
      </Card>
    </div>
  );
};

export default Classification;
